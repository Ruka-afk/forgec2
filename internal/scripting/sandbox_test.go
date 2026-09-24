package scripting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScriptRequireCannotReadHostFiles proves the script VM's require() cannot
// load or evaluate host modules. Before this, the default goja_nodejs source
// loader resolved relative/absolute paths, so a script could read server-side
// JS/JSON and step outside the capability bridge.
func TestScriptRequireCannotReadHostFiles(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(secret, []byte(`{"token":"super-secret-value"}`), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	e := NewScriptEngine()
	caller := Caller{Username: "tester", Role: "user"}

	cases := []struct {
		name string
		code string
	}{
		{"absolute path", `require("` + filepath.ToSlash(secret) + `")`},
		{"relative traversal", `require("secret.json")`},
		{"bare builtin guess", `require("fs")`},
		{"node prefix", `require("node:fs")`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := e.ExecuteCode(tc.code, nil, caller)
			if res.Success {
				t.Fatalf("require() of a host module succeeded: output=%q", res.Output)
			}
			// File-backed modules are refused by the deny-all loader; unknown
			// built-ins fail earlier in resolution. Either way nothing loads.
			refused := strings.Contains(res.Error, "host modules is disabled") ||
				strings.Contains(res.Error, "No such built-in module")
			if !refused {
				t.Fatalf("unexpected error (want a module refusal): %v", res.Error)
			}
			if strings.Contains(res.Error, "super-secret-value") {
				t.Fatal("fixture contents leaked into the script error")
			}
		})
	}
}

// TestScriptConsoleStillWorks guards the capability change: console is a Go
// core module and must keep resolving even though file-backed modules do not.
func TestScriptConsoleStillWorks(t *testing.T) {
	e := NewScriptEngine()
	res := e.ExecuteCode(`console.log("hello from script"); "done"`, nil, Caller{Username: "tester", Role: "user"})
	if !res.Success {
		t.Fatalf("console-enabled script failed: %v", res.Error)
	}
	if res.Output != "done" {
		t.Fatalf("output = %q, want %q", res.Output, "done")
	}
}

// TestScriptSourceLimit proves oversized scripts are refused before they reach
// the parser (memory/parse-time DoS on the in-process VM).
func TestScriptSourceLimit(t *testing.T) {
	e := NewScriptEngine()
	huge := "// " + strings.Repeat("x", scriptSourceLimit) + "\n"
	res := e.ExecuteCode(huge, nil, Caller{Username: "tester", Role: "user"})
	if res.Success {
		t.Fatal("oversized script was executed")
	}
	if !strings.Contains(res.Error, "script too large") {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if err := e.LoadScript(1, "huge", huge); err == nil {
		t.Fatal("LoadScript accepted an oversized script")
	}
}

// TestScriptParseTimeout proves a top-level infinite loop cannot wedge
// LoadScript forever: the reparse VM is interrupted on the event timeout.
func TestScriptParseTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("timeout test")
	}
	e := NewScriptEngine()
	err := e.LoadScript(1, "spin", "while(true){}")
	if err == nil {
		t.Fatal("expected the spinning script to be rejected")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
}
