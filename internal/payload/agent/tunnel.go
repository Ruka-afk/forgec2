//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"strings"
)

// handleTunnelAddRoute and handleTunnelRemoveRoute are honest rejections of the
// legacy "dynamic subnet routing" task surface. The former implementation kept
// a tunnelRoutes map and reported "tunnel route added" success — but nothing in
// the agent or server ever consumed those routes: the SOCKS relay dials targets
// directly, and no frame source on either side drives tunnel_add/tunnel_remove.
// A feature that has no effect must not claim success, so the dead store was
// removed and the tasks now state exactly that.
func handleTunnelAddRoute(task Task, res *TaskResult) {
	subnet := strings.TrimSpace(task.Command)
	if subnet == "" {
		res.Error = "subnet is required (e.g. 10.10.0.0/16)"
		return
	}
	res.Error = fmt.Sprintf(
		"dynamic subnet routing is not implemented: the SOCKS relay dials targets directly and no routing layer consumes routes (route %q was NOT added)", subnet)
	if Debug {
		fmt.Printf("[tunnel] add route %s rejected (feature not implemented)\n", subnet)
	}
}

func handleTunnelRemoveRoute(task Task, res *TaskResult) {
	subnet := strings.TrimSpace(task.Command)
	if subnet == "" {
		res.Error = "subnet is required"
		return
	}
	res.Error = fmt.Sprintf(
		"dynamic subnet routing is not implemented; nothing to remove (route %q was never added)", subnet)
	if Debug {
		fmt.Printf("[tunnel] remove route %s rejected (feature not implemented)\n", subnet)
	}
}