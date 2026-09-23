package main

import (
	"net"
)

// LocalAddr is one IPv4 address of this PC.
type LocalAddr struct {
	Interface string `json:"interface"`
	CIDR      string `json:"cidr"`
	SameNet   bool   `json:"sameNet"` // drive is inside this subnet
}

// NetCheck lists this PC's IPv4 addresses and whether any of them shares a
// subnet with the drive. The usual reason the drive cannot be reached is a
// laptop Ethernet port that has no address in the drive's subnet.
type NetCheck struct {
	DriveIsIP bool        `json:"driveIsIP"`
	Addrs     []LocalAddr `json:"addrs"`
	SameNet   bool        `json:"sameNet"`
}

func checkNetwork(drive string) NetCheck {
	ip := net.ParseIP(drive).To4()
	nc := NetCheck{DriveIsIP: ip != nil}
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			la := LocalAddr{Interface: ifc.Name, CIDR: ipn.String()}
			if ip != nil && ipn.Contains(ip) {
				la.SameNet = true
				nc.SameNet = true
			}
			nc.Addrs = append(nc.Addrs, la)
		}
	}
	return nc
}
