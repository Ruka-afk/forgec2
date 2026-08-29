//go:build linux || windows || darwin

package main

import "testing"

func TestTunnelRouteAllowlist(t *testing.T) {
	tunnelRoutesMu.Lock()
	tunnelRoutes = nil
	tunnelRoutesMu.Unlock()
	t.Cleanup(func() {
		tunnelRoutesMu.Lock()
		tunnelRoutes = nil
		tunnelRoutesMu.Unlock()
	})

	if !tunnelRouteAllowed("8.8.8.8") {
		t.Fatal("empty allowlist must permit all destinations")
	}

	var add, rem TaskResult
	handleTunnelAddRoute(Task{Command: "10.0.0.0/8"}, &add)
	if add.Error != "" {
		t.Fatalf("add: %s", add.Error)
	}
	if !tunnelRouteAllowed("10.1.2.3") {
		t.Fatal("10.1.2.3 should match 10.0.0.0/8")
	}
	if tunnelRouteAllowed("8.8.8.8") {
		t.Fatal("8.8.8.8 must be denied once an allowlist exists")
	}

	handleTunnelRemoveRoute(Task{Command: "10.0.0.0/8"}, &rem)
	if rem.Error != "" {
		t.Fatalf("remove: %s", rem.Error)
	}
	if !tunnelRouteAllowed("8.8.8.8") {
		t.Fatal("allowlist empty again; 8.8.8.8 should pass")
	}
}

func TestTunnelAddRejectsBadCIDR(t *testing.T) {
	var res TaskResult
	handleTunnelAddRoute(Task{Command: "not-a-cidr"}, &res)
	if res.Error == "" {
		t.Fatal("expected invalid CIDR error")
	}
}
