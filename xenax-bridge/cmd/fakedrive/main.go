// fakedrive is a tiny XENAX ASCII-protocol simulator for trying out
// xenax-bridge without real hardware. It is NOT a faithful model of the
// drive; it only answers a handful of commands so the UI can be exercised.
//
//	go run ./cmd/fakedrive              (listens on 127.0.0.1:10001)
//	xenax-bridge -drive 127.0.0.1
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
)

type axis struct {
	mu                  sync.Mutex
	pos, speed, acc     int
	powered, referenced bool
}

func (a *axis) handle(cmd string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := strings.ToUpper(cmd)
	num := func(prefix string) (int, bool) {
		n, err := strconv.Atoi(strings.TrimPrefix(c, prefix))
		return n, err == nil
	}
	switch {
	case c == "PW" || c == "PWC":
		a.powered = true
	case c == "PQ":
		a.powered = false
	case c == "REF":
		if !a.powered {
			return "#"
		}
		a.pos, a.referenced = 0, true
	case c == "SM":
	case c == "TP":
		return fmt.Sprintf("%d\r>", a.pos)
	case c == "TPS":
		s := 0
		if a.powered {
			s |= 1
		}
		if a.referenced {
			s |= 2
		}
		return fmt.Sprintf("%08X\r>", s)
	case c == "TE":
		return "0\r>"
	case strings.HasPrefix(c, "SP"):
		if n, ok := num("SP"); ok {
			a.speed = n
		} else {
			return fmt.Sprintf("%d\r>", a.speed)
		}
	case strings.HasPrefix(c, "AC"):
		if n, ok := num("AC"); ok {
			a.acc = n
		} else {
			return fmt.Sprintf("%d\r>", a.acc)
		}
	case strings.HasPrefix(c, "G"):
		n, ok := num("G")
		if !ok {
			return "?"
		}
		if !a.powered || !a.referenced {
			return "#"
		}
		a.pos = n
	default:
		return "?"
	}
	return ">"
}

func main() {
	listen := flag.String("listen", "127.0.0.1:10001", "address to listen on")
	flag.Parse()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("fake XENAX drive listening on %s", ln.Addr())
	ax := &axis{speed: 100000, acc: 1000000}
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			r := bufio.NewReader(c)
			for {
				line, err := r.ReadString('\r')
				if err != nil {
					return
				}
				cmd := strings.TrimSpace(line)
				if cmd == "" {
					continue
				}
				fmt.Fprintf(c, "%s\r%s", cmd, ax.handle(cmd))
			}
		}(c)
	}
}
