package protocol

import (
	"strings"
	"testing"
)

// TestEveryConstantHasSpec is the new drift guard: after the taskspec
// migration, the single-point registry must cover every declared task type
// constant. A missing spec means no server metadata and no agent help entry.
func TestEveryConstantHasSpec(t *testing.T) {
	all := AllTaskTypes()
	if len(all) == 0 {
		t.Fatal("AllTaskTypes empty")
	}
	for _, typ := range all {
		sp, ok := SpecOf(typ)
		if !ok {
			t.Errorf("constant %q has no TaskSpec", typ)
			continue
		}
		if sp.Type != typ {
			t.Errorf("spec type mismatch: %q != %q", sp.Type, typ)
		}
		if sp.Name == "" {
			t.Errorf("spec %q has empty Name", typ)
		}
	}
}

// TestSpecTypesAreDeclared pins the reverse direction: nothing may register a
// spec for an undeclared constant (would be dispatchable but unvalidatable).
func TestSpecTypesAreDeclared(t *testing.T) {
	declared := make(map[string]bool, len(AllTaskTypes()))
	for _, typ := range AllTaskTypes() {
		declared[typ] = true
	}
	for _, sp := range AllSpecs() {
		if !declared[sp.Type] {
			t.Errorf("spec %q has no matching constant in AllTaskTypes", sp.Type)
		}
	}
}

// TestAliasResolution covers alias→canonical mapping plus collision safety:
// aliases must never shadow a canonical type and must resolve uniquely.
func TestAliasResolution(t *testing.T) {
	canon, ok := ResolveAlias("hi")
	if !ok || canon != TaskTypeHostInfo {
		t.Fatalf("ResolveAlias(hi) = %q,%v; want %q,true", canon, ok, TaskTypeHostInfo)
	}
	// Canonical names resolve to themselves.
	got, _ := ResolveAlias(TaskTypeShell)
	if got != TaskTypeShell {
		t.Fatalf("canonical passthrough broken: %q", got)
	}
	// Every registered alias points at a live spec.
	for _, sp := range AllSpecs() {
		for _, a := range sp.Aliases {
			target, ok := ResolveAlias(a)
			if !ok || target != sp.Type {
				t.Errorf("alias %q -> %q,%v; want %q", a, target, ok, sp.Type)
			}
			if strings.EqualFold(a, sp.Type) {
				t.Errorf("alias %q duplicates its own canonical type", a)
			}
		}
	}
}

// TestRegisterTaskSpecRejectsDuplicates verifies the loud-failure contract.
func TestRegisterTaskSpecRejectsDuplicates(t *testing.T) {
	cases := []struct {
		name string
		spec TaskSpec
	}{
		{"dup type", TaskSpec{Type: TaskTypeShell, Name: "clash"}},
		{"alias shadows type", TaskSpec{Type: "x1", Name: "x", Aliases: []string{TaskTypeShell}}},
		{"empty type", TaskSpec{Name: "nope"}},
	}
	for _, tc := range cases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("%s: expected panic", tc.name)
				}
			}()
			RegisterTaskSpec(tc.spec)
		}()
	}
}

// TestMustSpecFailsLoudly ensures init-time binding typos surface at startup.
func TestMustSpecFailsLoudly(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unknown spec")
		}
	}()
	MustSpec("definitely-not-a-task")
}

// TestHelpBuiltinSpecExists guards the operator-facing catalogue command.
func TestHelpBuiltinSpecExists(t *testing.T) {
	sp, ok := SpecOf(TaskTypeHelp)
	if !ok {
		t.Fatal("help builtin missing from spec registry")
	}
	if !strings.Contains(strings.ToLower(sp.Description), "command") {
		t.Fatalf("help description unexpected: %q", sp.Description)
	}
}
