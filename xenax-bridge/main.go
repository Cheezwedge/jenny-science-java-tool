// xenax-bridge is a small, dependency-free helper that lets a normal web
// browser (no Java plug-in) talk to a Jenny Science XENAX servo controller.
//
// It does two things:
//
//  1. Opens the XENAX ASCII command channel (TCP port 10001 by default) and
//     exposes it to a built-in HTML5 control page served on localhost.
//  2. Reverse-proxies the drive's own web server under /drive/ so the
//     original pages and files (manuals, applet JAR, etc.) can still be
//     reached through the same address.
//
// Run it, then open http://127.0.0.1:8080 in any browser.
package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

var version = "dev"

func main() {
	driveFlag := flag.String("drive", "", "IP address or hostname of the XENAX drive (default: last used, else 192.168.2.100)")
	asciiPortFlag := flag.Int("ascii-port", 0, "TCP port of the XENAX ASCII command interface (default 10001)")
	httpPortFlag := flag.Int("http-port", 0, "TCP port of the drive's built-in web server, for /drive/ (default 80)")
	listen := flag.String("listen", "127.0.0.1:8080", "address the local web UI listens on")
	timeout := flag.Duration("timeout", 2*time.Second, "how long to wait for a reply to one command")
	readOnly := flag.Bool("read-only", false, "only allow 'tell' (T...) queries and SM (stop); block everything else")
	noBrowser := flag.Bool("no-browser", false, "do not open the web browser automatically")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("xenax-bridge", version)
		return
	}

	// Saved settings, overridden by any command-line flags.
	cfg := loadSettings()
	if *driveFlag != "" {
		cfg.Drive = *driveFlag
	}
	if *asciiPortFlag != 0 {
		cfg.AsciiPort = *asciiPortFlag
	}
	if *httpPortFlag != 0 {
		cfg.HTTPPort = *httpPortFlag
	}
	if err := cfg.Validate(); err != nil {
		fatal(err)
	}
	var cfgMu sync.RWMutex
	current := func() Settings {
		cfgMu.RLock()
		defer cfgMu.RUnlock()
		return cfg
	}

	drive := NewDriveConn(cfg.ASCIIAddr(), *timeout)
	defer drive.Close()

	mux := http.NewServeMux()

	// Built-in HTML5 UI.
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// JSON API used by the UI.
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		c := current()
		writeJSON(w, http.StatusOK, map[string]any{
			"version":   version,
			"drive":     c.Drive,
			"asciiPort": c.AsciiPort,
			"httpPort":  c.HTTPPort,
			"readOnly":  *readOnly,
			"connected": drive.Connected(),
			"network":   checkNetwork(c.Drive),
		})
	})
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !sameOrigin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var s Settings
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&s); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad JSON"})
			return
		}
		s.Drive = strings.TrimSpace(s.Drive)
		if err := s.Validate(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		cfgMu.Lock()
		cfg = s
		cfgMu.Unlock()
		drive.SetAddr(s.ASCIIAddr())
		log.Printf("drive set to %s (ASCII %d, web %d)", s.Drive, s.AsciiPort, s.HTTPPort)
		out := map[string]any{"network": checkNetwork(s.Drive)}
		if err := saveSettings(s); err != nil {
			out["warning"] = "settings not saved: " + err.Error()
		}
		if err := drive.Connect(); err != nil {
			out["error"] = err.Error()
		}
		out["connected"] = drive.Connected()
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("/api/cmd", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		var req struct {
			Cmd string `json:"cmd"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			http.Error(w, "bad JSON", http.StatusBadRequest)
			return
		}
		cmd := strings.TrimSpace(req.Cmd)
		if cmd == "" || strings.ContainsAny(cmd, "\r\n") {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "empty or multi-line command"})
			return
		}
		if *readOnly && !isQuery(cmd) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "read-only mode: only T... (tell) commands and SM are allowed"})
			return
		}
		start := time.Now()
		res, err := drive.Send(cmd)
		out := map[string]any{
			"cmd":       cmd,
			"raw":       res.Raw,
			"value":     res.Value,
			"status":    res.Status,
			"ms":        time.Since(start).Milliseconds(),
			"connected": drive.Connected(),
		}
		if err != nil {
			out["error"] = err.Error()
			writeJSON(w, http.StatusBadGateway, out)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("/api/reconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !sameOrigin(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		drive.Close()
		err := drive.Connect()
		out := map[string]any{"connected": drive.Connected()}
		if err != nil {
			out["error"] = err.Error()
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Pass-through to the drive's own web server (follows settings changes).
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(&url.URL{Scheme: "http", Host: current().HTTPAddr()})
			pr.Out.Host = pr.Out.URL.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "could not reach drive web server at "+current().HTTPAddr()+": "+err.Error(), http.StatusBadGateway)
		},
	}
	mux.Handle("/drive/", http.StripPrefix("/drive", proxy))

	ln, err := net.Listen("tcp", *listen)
	if err != nil && !flagSet("listen") {
		// Port 8080 is often taken on office PCs; fall back to any free port.
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		fatal(fmt.Errorf("cannot listen on %s: %v", *listen, err))
	}
	uiURL := "http://" + ln.Addr().String() + "/"
	log.Printf("xenax-bridge %s", version)
	log.Printf("drive %s  (ASCII port %d, web port %d)", cfg.Drive, cfg.AsciiPort, cfg.HTTPPort)
	if *readOnly {
		log.Printf("READ-ONLY mode: only T... query commands and SM will be sent")
	}
	if nc := checkNetwork(cfg.Drive); nc.DriveIsIP && !nc.SameNet {
		log.Printf("warning: no network adapter on this PC is in the same subnet as %s", cfg.Drive)
	}
	log.Printf("open %s in your browser. Keep this window open; close it to quit.", uiURL)

	// Try to connect up front so problems show immediately; not fatal.
	go func() {
		if err := drive.Connect(); err != nil {
			log.Printf("warning: %v (will retry on first command)", err)
		}
	}()

	if !*noBrowser {
		go openBrowser(uiURL)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	fatal(srv.Serve(ln))
}

func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// fatal prints err and exits. On Windows it waits for Enter first, so the
// message stays readable when the program was started by double-clicking.
func fatal(err error) {
	log.Printf("error: %v", err)
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "Press Enter to close this window.")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(1)
}

// isQuery reports whether cmd is allowed in read-only mode: a "tell" command
// (TP, TPS, TE, ...) that only reads data, or SM (stop motion).
func isQuery(cmd string) bool {
	c := strings.ToUpper(cmd)
	if c == "SM" {
		return true
	}
	return len(c) >= 2 && c[0] == 'T' && c[1] >= 'A' && c[1] <= 'Z'
}

// sameOrigin blocks other web sites open in the same browser from driving the
// motor through this local server (simple CSRF protection).
func sameOrigin(r *http.Request) bool {
	// Only accept requests addressed to an IP literal or "localhost". This
	// stops DNS-rebinding attacks, which need a domain name in the Host header.
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	if net.ParseIP(strings.Trim(host, "[]")) == nil && !strings.EqualFold(host, "localhost") {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return r.Header.Get("X-Requested-With") == "xenax-bridge"
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func openBrowser(u string) {
	time.Sleep(300 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return
		}
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start()
}
