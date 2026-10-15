package plugin

import (
	"path/filepath"
	"strings"
	"testing"
)

// hasDiag reports whether the report carries a finding with that level and code.
func hasDiag(rep *CheckReport, level, code string) bool {
	for _, d := range rep.Diagnostics {
		if d.Level == level && d.Code == code {
			return true
		}
	}
	return false
}

// diagText returns every message for a code, joined, for hint assertions.
func diagText(rep *CheckReport, code string) string {
	var out []string
	for _, d := range rep.Diagnostics {
		if d.Code == code {
			out = append(out, d.Message)
		}
	}
	return strings.Join(out, "\n")
}

// scaffoldHostInterpreter returns an interpreter from ValidInterpreters that
// actually runs on this host, or skips: the tests must not fail on machines
// whose python is a WindowsApps alias or that have no interpreter at all.
func scaffoldHostInterpreter(t *testing.T) string {
	t.Helper()
	for _, candidate := range scaffoldInterpreterCandidates() {
		if !isValidValue(candidate, ValidInterpreters) {
			continue
		}
		if err := probeInterpreter(candidate); err == nil {
			return candidate
		}
	}
	t.Skip("no interpreter from ValidInterpreters runs on this host")
	return ""
}

// TestScaffoldPluginProducesCheckablePackages is the core promise of the
// authoring pipeline: a fresh scaffold passes the checker for every type and
// interpreter combination, so the author starts from green. The WASM tier is
// the one exception by design — its entry only exists after a build step, and
// the checker must say exactly that, with the build command to run.
func TestScaffoldPluginProducesCheckablePackages(t *testing.T) {
	for _, interpreter := range []string{"python3", "sh", "go", "wasm"} {
		for _, typ := range ValidPluginTypes {
			name := "demo-" + interpreter + "-" + typ
			dir := filepath.Join(t.TempDir(), name)
			res, err := ScaffoldPlugin(ScaffoldOptions{
				Name:        name,
				Type:        typ,
				Interpreter: interpreter,
				Dir:         dir,
			})
			if err != nil {
				t.Fatalf("scaffold %s/%s: %v", interpreter, typ, err)
			}
			if len(res.Files) == 0 {
				t.Fatalf("scaffold %s/%s wrote no files", interpreter, typ)
			}
			rep := CheckPlugin(CheckOptions{Dir: dir})

			if interpreter == "wasm" {
				if res.BuildHint == "" {
					t.Errorf("wasm scaffold must return a build hint")
				}
				if rep.Errors != 1 || !hasDiag(rep, DiagError, "package.entry_missing") {
					t.Errorf("unbuilt wasm scaffold must fail with exactly package.entry_missing, got %v", rep.Diagnostics)
				}
				if !strings.Contains(diagText(rep, "package.entry_missing"), "build.sh") {
					t.Errorf("entry_missing must name the build script, got %q", diagText(rep, "package.entry_missing"))
				}
				continue
			}
			// tier.* findings depend on this host: a machine whose python3 is
			// the broken WindowsApps alias fails the interpreter probe, and the
			// checker is right to say so. The template itself must still pass.
			for _, d := range rep.Diagnostics {
				if d.Level == DiagError && !strings.HasPrefix(d.Code, "tier.") {
					t.Errorf("scaffold %s/%s fails its own checker: %s: %s", interpreter, typ, d.Code, d.Message)
				}
			}
		}
	}
}

// TestScaffoldPluginAutoDetectsAWorkingInterpreter covers the detection path:
// the manifest must name an interpreter that actually runs here, so the
// generated package is runnable the moment it is written.
func TestScaffoldPluginAutoDetectsAWorkingInterpreter(t *testing.T) {
	scaffoldHostInterpreter(t) // skips when this host has no usable interpreter

	dir := filepath.Join(t.TempDir(), "demo-auto")
	res, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo-auto", Type: "command", Dir: dir})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	m, err := LoadManifest(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if err := probeInterpreter(m.Interpreter); err != nil {
		t.Fatalf("auto-detected interpreter %q does not run: %v", m.Interpreter, err)
	}
	if len(res.Notes) == 0 {
		t.Errorf("an auto-detected interpreter must be disclosed in Notes")
	}
}

// TestScaffoldPluginRejectsInvalidInput keeps typos from producing packages the
// server would reject at load time.
func TestScaffoldPluginRejectsInvalidInput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pkg")
	cases := []struct {
		name string
		opts ScaffoldOptions
		want string
	}{
		{"missing name", ScaffoldOptions{Type: "command", Interpreter: "sh", Dir: dir}, "name is required"},
		{"bad name", ScaffoldOptions{Name: "Bad Name", Type: "command", Interpreter: "sh", Dir: dir}, "lowercase"},
		{"bad type", ScaffoldOptions{Name: "demo", Type: "daemon", Interpreter: "sh", Dir: dir}, "plugin type"},
		{"bad interpreter", ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "perl", Dir: dir}, "interpreter"},
		{"missing dir", ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh"}, "directory is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ScaffoldPlugin(tc.opts); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error mentioning %q, got %v", tc.want, err)
			}
		})
	}
}

// TestScaffoldPluginRefusesToOverwrite protects hand-written plugins from being
// replaced by a repeated scaffold; -force is the explicit escape hatch.
func TestScaffoldPluginRefusesToOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh", Dir: dir}); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh", Dir: dir}); err == nil {
		t.Fatalf("second scaffold into a non-empty directory must fail without -force")
	}
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh", Dir: dir, Force: true}); err != nil {
		t.Fatalf("forced scaffold: %v", err)
	}
}
