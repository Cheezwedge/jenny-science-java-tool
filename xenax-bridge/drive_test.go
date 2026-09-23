package main

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"
)

func TestParseReply(t *testing.T) {
	cases := []struct {
		cmd, raw, value, status string
		done                    bool
	}{
		{"TP", "TP\r12345\r>", "12345", "ok", true},
		{"TP", "TP\r-50\r", "", "", false},
		{"PWC", "PWC\r>", "", "ok", true},
		{"XYZ", "XYZ\r?", "", "unknown", true},
		{"G100", "G100\r#", "", "not-executable", true},
		{"TP", "T", "", "", false},
		{"tp", "TP\r7\r\n>\r\n", "7", "ok", true},
	}
	for _, c := range cases {
		res, done := parseReply(c.cmd, c.raw)
		if done != c.done || (done && (res.Value != c.value || res.Status != c.status)) {
			t.Errorf("parseReply(%q,%q) = %+v,%v; want value=%q status=%q done=%v", c.cmd, c.raw, res, done, c.value, c.status, c.done)
		}
	}
}

// fakeServer answers like a drive, splitting replies into small chunks to
// exercise reassembly, and sends an unsolicited event line first.
func fakeServer(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.Write([]byte("@S0\r"))
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\r')
					if err != nil {
						return
					}
					cmd := strings.TrimSpace(line)
					var reply string
					switch cmd {
					case "TP":
						reply = "TP\r-4200\r>"
					case "SLOW":
						time.Sleep(500 * time.Millisecond)
						reply = "SLOW\r>"
					default:
						reply = cmd + "\r?"
					}
					for i := 0; i < len(reply); i += 2 {
						end := min(i+2, len(reply))
						c.Write([]byte(reply[i:end]))
						time.Sleep(time.Millisecond)
					}
				}
			}(c)
		}
	}()
	return ln.Addr().String()
}

func TestSend(t *testing.T) {
	d := NewDriveConn(fakeServer(t), 200*time.Millisecond)
	defer d.Close()
	time.Sleep(20 * time.Millisecond)

	res, err := d.Send("TP")
	if err != nil || res.Value != "-4200" || res.Status != "ok" {
		t.Fatalf("TP: %+v %v", res, err)
	}
	res, err = d.Send("FOO")
	if err != nil || res.Status != "unknown" {
		t.Fatalf("FOO: %+v %v", res, err)
	}
	if _, err = d.Send("SLOW"); err != errTimeout {
		t.Fatalf("SLOW: want timeout, got %v", err)
	}
}

func TestIsQuery(t *testing.T) {
	for cmd, want := range map[string]bool{"TP": true, "tps": true, "SM": true, "G100": false, "PW": false, "T": false, "T1": false} {
		if isQuery(cmd) != want {
			t.Errorf("isQuery(%q) != %v", cmd, want)
		}
	}
}
