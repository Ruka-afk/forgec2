package main

import (
	"strings"
	"testing"
)

// The module bookkeeping is the honest half of the module system: a generated
// build must answer excluded commands by name, and must report what it was
// built with. The generator-injected modules_excluded.go (see
// internal/payload/generator_modules.go) is what these tests impersonate.

// TestAgentModulesSummary pins the compiled-set wording. nil is the hand-build
// fallback (everything in), an empty non-nil list is a generated core-only
// build, and a list is a partial build — the generator must therefore emit
// []string{} for core-only, never a nil declaration.
func TestAgentModulesSummary(t *testing.T) {
	savedModules := compiledAgentModules
	defer func() { compiledAgentModules = savedModules }()

	compiledAgentModules = nil
	if got := agentModulesSummary(); got != "all modules (hand-built agent)" {
		t.Errorf("hand-build summary = %q", got)
	}
	compiledAgentModules = []string{}
	if got := agentModulesSummary(); got != "core only" {
		t.Errorf("core-only summary = %q", got)
	}
	compiledAgentModules = []string{"screen", "ad"}
	if got := agentModulesSummary(); got != "core, screen, ad" {
		t.Errorf("partial summary = %q", got)
	}
}

// TestExcludedModuleLookup covers the generated-map lookups dispatch and help
// are built on.
func TestExcludedModuleLookup(t *testing.T) {
	savedTasks := excludedModuleTasks
	defer func() { excludedModuleTasks = savedTasks }()
	excludedModuleTasks = map[string]string{"gossip_discover": "p2p", "screenshot": "screen"}

	if got := excludedModuleFor("gossip_discover"); got != "p2p" {
		t.Errorf("excludedModuleFor(gossip_discover) = %q", got)
	}
	if got := excludedModuleFor("shell"); got != "" {
		t.Errorf("excludedModuleFor(shell) = %q, want empty", got)
	}
	if got := countExcludedTasks([]string{"shell", "screenshot", "gossip_discover"}); got != 2 {
		t.Errorf("countExcludedTasks = %d, want 2", got)
	}
}

// TestExecuteTaskHonestExclusion pins the exact operator-facing error: an
// excluded task names itself and its module instead of falling through to the
// generic "unknown task type" message.
func TestExecuteTaskHonestExclusion(t *testing.T) {
	savedTasks := excludedModuleTasks
	defer func() { excludedModuleTasks = savedTasks }()
	excludedModuleTasks = map[string]string{"gossip_discover": "p2p"}

	res := executeTask(Task{ID: 1234, Type: "gossip_discover"})
	want := `task type "gossip_discover" is not compiled into this build: agent module "p2p" was excluded at generation time`
	if res.Error != want {
		t.Fatalf("honest exclusion error = %q, want %q", res.Error, want)
	}
}

// TestHelpMarksExcludedModules pins the help surface: the header counts the
// exclusions and names the compiled set, and each excluded command carries its
// module marker.
func TestHelpMarksExcludedModules(t *testing.T) {
	savedTasks := excludedModuleTasks
	savedModules := compiledAgentModules
	defer func() {
		excludedModuleTasks = savedTasks
		compiledAgentModules = savedModules
	}()
	excludedModuleTasks = map[string]string{"gossip_discover": "p2p"}
	compiledAgentModules = []string{}

	res := TaskResult{}
	handleHelp(Task{ID: 901, Type: "help"}, &res)
	out := res.Output
	header := out
	if i := strings.IndexByte(out, '\n'); i >= 0 {
		header = out[:i]
	}
	if !strings.Contains(header, "not compiled into this build") {
		t.Errorf("help header must count the exclusions: %s", header)
	}
	if !strings.Contains(header, "agent modules: core only") {
		t.Errorf("help header must report the compiled set: %s", header)
	}
	if !strings.Contains(out, `[not in this build: module "p2p"]`) {
		t.Error("help must mark the excluded command with its module")
	}
}
