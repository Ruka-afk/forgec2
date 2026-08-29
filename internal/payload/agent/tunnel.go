//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

// tunnelRoutes is a SOCKS CONNECT allowlist. When empty, SOCKS dials any
// destination (legacy behaviour). After one or more CIDRs are added, CONNECT
// is refused unless the resolved destination falls in a listed network.
// This is the operator-facing "dynamic subnet routing" surface: it does not
// create a TUN device (that needs extra host support) but it does consume
// routes instead of pretending to.
var (
	tunnelRoutesMu sync.Mutex
	tunnelRoutes   []net.IPNet
)

func handleTunnelAddRoute(task Task, res *TaskResult) {
	subnet := strings.TrimSpace(task.Command)
	if subnet == "" {
		res.Error = "subnet is required (e.g. 10.10.0.0/16)"
		return
	}
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		res.Error = "invalid CIDR: " + err.Error()
		return
	}
	tunnelRoutesMu.Lock()
	for _, existing := range tunnelRoutes {
		if existing.String() == ipnet.String() {
			tunnelRoutesMu.Unlock()
			res.Output = fmt.Sprintf("tunnel route already present: %s", ipnet)
			return
		}
	}
	tunnelRoutes = append(tunnelRoutes, *ipnet)
	n := len(tunnelRoutes)
	tunnelRoutesMu.Unlock()
	res.Output = fmt.Sprintf("tunnel route added: %s (SOCKS CONNECT allowlist, %d route(s))", ipnet, n)
}

func handleTunnelRemoveRoute(task Task, res *TaskResult) {
	subnet := strings.TrimSpace(task.Command)
	if subnet == "" {
		res.Error = "subnet is required"
		return
	}
	_, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		res.Error = "invalid CIDR: " + err.Error()
		return
	}
	want := ipnet.String()
	tunnelRoutesMu.Lock()
	defer tunnelRoutesMu.Unlock()
	filtered := tunnelRoutes[:0]
	removed := false
	for _, existing := range tunnelRoutes {
		if existing.String() == want {
			removed = true
			continue
		}
		filtered = append(filtered, existing)
	}
	tunnelRoutes = filtered
	if !removed {
		res.Error = fmt.Sprintf("route %s was not present", want)
		return
	}
	res.Output = fmt.Sprintf("tunnel route removed: %s (%d remaining)", want, len(tunnelRoutes))
}

func tunnelRouteAllowed(host string) bool {
	tunnelRoutesMu.Lock()
	routes := append([]net.IPNet(nil), tunnelRoutes...)
	tunnelRoutesMu.Unlock()
	if len(routes) == 0 {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return false
		}
		ip = ips[0]
	}
	for _, n := range routes {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
