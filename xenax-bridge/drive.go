package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Result is one command/response exchange with the drive.
//
// The XENAX ASCII protocol echoes the command, optionally returns a value,
// and finishes with a one-character prompt:
//
//	">"  command accepted
//	"?"  command unknown / syntax error
//	"#"  command known but not executable in the current state
type Result struct {
	Raw    string // everything the drive sent back
	Value  string // reply with echo and prompt removed
	Status string // "ok", "unknown", "not-executable" or "timeout"
}

// DriveConn is a single, serialised TCP connection to the ASCII interface.
// The drive only accepts one client on the ASCII port at a time, so all
// browser requests share this connection.
type DriveConn struct {
	addr    string
	timeout time.Duration

	mu   sync.Mutex
	conn net.Conn
}

func NewDriveConn(addr string, timeout time.Duration) *DriveConn {
	return &DriveConn{addr: addr, timeout: timeout}
}

func (d *DriveConn) Connected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn != nil
}

func (d *DriveConn) Connect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connectLocked()
}

func (d *DriveConn) connectLocked() error {
	if d.conn != nil {
		return nil
	}
	c, err := net.DialTimeout("tcp", d.addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to drive at %s: %w", d.addr, err)
	}
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
		_ = tc.SetKeepAlive(true)
	}
	d.conn = c
	return nil
}

func (d *DriveConn) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}
}

// Send transmits one command and waits for the prompt character.
// On an I/O error the connection is dropped and retried once.
func (d *DriveConn) Send(cmd string) (Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := d.connectLocked(); err != nil {
			return Result{Status: "error"}, err
		}
		res, err := d.exchange(cmd)
		if errors.Is(err, errTimeout) {
			// A late reply could be mistaken for the answer to the next
			// command, so start over with a fresh connection.
			d.conn.Close()
			d.conn = nil
			return res, err
		}
		if err == nil {
			return res, nil
		}
		lastErr = err
		d.conn.Close()
		d.conn = nil
	}
	return Result{Status: "error"}, lastErr
}

var errTimeout = errors.New("no complete reply from drive (timeout)")

func (d *DriveConn) exchange(cmd string) (Result, error) {
	c := d.conn
	d.drain()

	_ = c.SetWriteDeadline(time.Now().Add(d.timeout))
	if _, err := c.Write([]byte(cmd + "\r")); err != nil {
		return Result{}, err
	}

	deadline := time.Now().Add(d.timeout)
	var buf []byte
	tmp := make([]byte, 1024)
	for {
		_ = c.SetReadDeadline(deadline)
		n, err := c.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if res, done := parseReply(cmd, string(buf)); done {
			return res, nil
		}
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return Result{Raw: string(buf), Value: stripEcho(cmd, string(buf)), Status: "timeout"}, errTimeout
			}
			return Result{Raw: string(buf)}, err
		}
	}
}

// drain discards unsolicited bytes (e.g. event messages) waiting on the socket
// so they are not mistaken for the reply to the next command.
func (d *DriveConn) drain() {
	tmp := make([]byte, 1024)
	for {
		_ = d.conn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
		n, err := d.conn.Read(tmp)
		if n == 0 || err != nil {
			return
		}
	}
}

// parseReply decides whether raw holds a complete reply to cmd.
func parseReply(cmd, raw string) (Result, bool) {
	body := stripEcho(cmd, raw)
	t := strings.TrimRight(body, " \r\n\t")
	if t == "" {
		return Result{}, false
	}
	var status string
	switch t[len(t)-1] {
	case '>':
		status = "ok"
	case '?':
		status = "unknown"
	case '#':
		status = "not-executable"
	default:
		return Result{}, false
	}
	value := strings.TrimSpace(t[:len(t)-1])
	value = strings.Join(strings.Fields(strings.NewReplacer("\r", " ", "\n", " ").Replace(value)), " ")
	return Result{Raw: raw, Value: value, Status: status}, true
}

// stripEcho removes the drive's echo of the command from the start of raw.
func stripEcho(cmd, raw string) string {
	r := strings.TrimLeft(raw, "\r\n ")
	if len(r) >= len(cmd) && strings.EqualFold(r[:len(cmd)], cmd) {
		return r[len(cmd):]
	}
	return r
}
