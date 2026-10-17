//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "privesc" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["privesc_check"] = handlePrivescCheck
	taskHandlers["elevate"] = handleElevate
	taskHandlers["elevate_printnightmare"] = handleElevatePrintNightmare
	taskHandlers["uac_bypass"] = handleUACBypass
	taskHandlers["fodhelper"] = handleFodhelper
	taskHandlers["slui"] = handleSluiUAC
	taskHandlers["eventvwr"] = handleEventvwrUAC
	taskHandlers["computerdefaults"] = handleComputerDefaultsUAC
	taskHandlers["juicy_potato"] = handleJuicyPotato
}
