//go:build linux || windows || darwin
// +build linux windows darwin

package main

// modules_default.go is the hand-build fallback: every optional module is
// compiled in, so nothing is excluded. The generator drops this file from the
// staged build tree and writes modules_excluded.go with the operator's actual
// selection (see internal/payload/agent_modules.go for the catalogue). The
// variables are consumed by task_modules.go.

// compiledAgentModules lists the optional modules compiled into this build.
// nil means "everything" (hand-built agent).
var compiledAgentModules []string

// excludedModuleTasks maps an excluded task type to the module that owns it.
var excludedModuleTasks = map[string]string{}
