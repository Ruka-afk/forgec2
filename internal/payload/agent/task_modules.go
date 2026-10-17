//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "strings"

// Agent module bookkeeping shared by every build. The data lives in
// modules_default.go (hand builds: nothing excluded) or, in a generated
// build, in the generator-injected modules_excluded.go. Keeping the helpers
// here means the generated file carries data only.

// agentModulesSummary renders the compiled module set for hostinfo and the
// help header.
func agentModulesSummary() string {
	if compiledAgentModules == nil {
		return "all modules (hand-built agent)"
	}
	if len(compiledAgentModules) == 0 {
		return "core only"
	}
	return "core, " + strings.Join(compiledAgentModules, ", ")
}

// excludedModuleFor reports the module that owns an excluded task type, or
// "" when the task is compiled in.
func excludedModuleFor(taskType string) string {
	return excludedModuleTasks[taskType]
}

// countExcludedTasks returns how many of the given task types are excluded.
func countExcludedTasks(types []string) int {
	n := 0
	for _, t := range types {
		if excludedModuleFor(t) != "" {
			n++
		}
	}
	return n
}
