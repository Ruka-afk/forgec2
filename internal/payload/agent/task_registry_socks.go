//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "socks_relay" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["socks"] = handleSocks
	taskHandlers["rportfwd_start"] = handleRPortFwdStart
	taskHandlers["rportfwd_stop"] = handleRPortFwdStop
	taskHandlers["lportfwd_start"] = handleLPortFwdStart
	taskHandlers["lportfwd_stop"] = handleLPortFwdStop
	taskHandlers["tunnel_add_route"] = handleTunnelAddRoute
	taskHandlers["tunnel_remove_route"] = handleTunnelRemoveRoute
}
