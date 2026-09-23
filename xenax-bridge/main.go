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
	"time"
)

//go:embed web
var webFS embed.FS

var version = "dev"

func main() {
	driveIP := flag.String("drive", "192.168.2.100", "IP address or hostname of the XENAX drive")
	asciiPort := flag.Int("ascii-port", 10001, "TCP port of the XENAX ASCII command interface")
	httpPort := flag.Int("http-port", 80, "TCP port of the drive's built-in web server (for /drive/ proxy)")
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

	drive := NewDriveConn(net.JoinHostPort(*driveIP, fmt.Sprint(*asciiPort)), *timeout)
	defer drive.Close()

	mux := http.NewServeMux()

	// Built-in HTML5 UI.
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// JSON API used by the UI.
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version":   version,
			"drive":     *driveIP,
			"asciiPort": *asciiPort,
			"httpPort":  *httpPort,
			"readOnly":  *readOnly,
			"connected": drive.Connected(),
		})
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

	// Pass-through to the drive's own web server.
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(*driveIP, fmt.Sprint(*httpPort))}
	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		origDirector(r)
		r.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "could not reach drive web server at "+target.Host+": "+err.Error(), http.StatusBadGateway)
	}
	mux.Handle("/drive/", http.StripPrefix("/drive", proxy))

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("cannot listen on %s: %v", *listen, err)
	}
	uiURL := "http://" + ln.Addr().String() + "/"
	log.Printf("xenax-bridge %s", version)
	log.Printf("drive ASCII interface: %s:%d   drive web server: %s", *driveIP, *asciiPort, target.Host)
	if *readOnly {
		log.Printf("READ-ONLY mode: only T... query commands will be sent")
	}
	log.Printf("open %s in your browser (Ctrl+C to quit)", uiURL)

	// Try to connect up front so problems show immediately; not fatal.
	if err := drive.Connect(); err != nil {
		log.Printf("warning: %v (will retry on first command)", err)
	}

	if !*noBrowser {
		go openBrowser(uiURL)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.Serve(ln))
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
