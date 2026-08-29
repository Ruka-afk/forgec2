package main

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestAliasDispatchRewritesType pins the dispatch behaviour: a task sent with
// an alias is executed under its canonical type and the result echoes the
// canonical name (renderers key off it).
func TestAliasDispatchRewritesType(t *testing.T) {
	res := executeTask(Task{ID: 900, Type: "hi", Command: "security"})
	if res.Type != protocol.TaskTypeHostInfo {
		t.Fatalf("result type = %q, want canonical %q", res.Type, protocol.TaskTypeHostInfo)
	}
	if res.Error != "" {
		t.Fatalf("aliased hostinfo failed: %s", res.Error)
	}
}

// TestHelpTaskListsCatalogue runs the help builtin and checks the catalogue
// carries canonical types, aliases and parameter hints.
func TestHelpTaskListsCatalogue(t *testing.T) {
	res := TaskResult{}
	handleHelp(Task{ID: 901, Type: "help"}, &res)
	if res.Error != "" {
		t.Fatalf("help errored: %s", res.Error)
	}
	out := res.Output
	for _, want := range []string{
		"commands available",
		protocol.TaskTypeShell,
		protocol.TaskTypeHostInfo,
		"alias: hi",
		"params:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q", want)
		}
	}
}

// TestRegisterTaskExpandsAliases proves RegisterTask binds every alias to
// the same handler without touching other entries.
func TestRegisterTaskExpandsAliases(t *testing.T) {
	const probe = "zz-probe-cmd"
	defer func() { delete(taskHandlers, probe); delete(taskHandlers, "zz-probe-alias") }()
	RegisterTask(protocol.TaskSpec{Type: probe, Name: "Probe", Aliases: []string{"zz-probe-alias"}},
		func(task Task, res *TaskResult) { res.Output = "ok" })
	if taskHandlers[probe] == nil || taskHandlers["zz-probe-alias"] == nil {
		t.Fatal("RegisterTask did not expand aliases")
	}
}

// TestUnknownTaskMessagePointsToHelp keeps the failure path discoverable.
func TestUnknownTaskMessagePointsToHelp(t *testing.T) {
	res := executeTask(Task{ID: 902, Type: "no-such-task"})
	if !strings.Contains(res.Error, `"help"`) {
		t.Fatalf("unknown-type error should point at help, got: %s", res.Error)
	}
}

// TestSortedSpecTypesDeterministic guards the test helper itself.
func TestSortedSpecTypesDeterministic(t *testing.T) {
	a := sortedSpecTypes()
	b := sortedSpecTypes()
	if len(a) != len(b) || len(a) == 0 {
		t.Fatalf("unstable/empty spec list: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("order differs at %d: %q vs %q", i, a[i], b[i])
		}
	}
}
