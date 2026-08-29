package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
)

// depTestManager builds a Manager over a fresh in-memory DB.
func depTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(testutil.SetupTestDB(t))
}

// writeDepPlugin materialises a minimal plugin package on disk so
// LoadFromDisk can discover it. Entry files are not executed during load.
func writeDepPlugin(t *testing.T, root, name, requires string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	requiresLine := ""
	if requires != "" {
		requiresLine = "requires: [" + requires + "]\n"
	}
	manifest := "name: " + name + "\n" +
		"version: 1.0.0\n" +
		"type: command\n" +
		"description: " + name + "\n" +
		"entry: main.py\n" +
		"interpreter: python\n" +
		requiresLine
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest %s: %v", name, err)
	}
}

// TestLoadFromDiskDependencyOrdering verifies a batch whose members depend on
// each other loads completely (topological pass resolves the order).
func TestLoadFromDiskDependencyOrdering(t *testing.T) {
	m := depTestManager(t)
	root := t.TempDir()
	writeDepPlugin(t, root, "bbb", "aaa") // alphabetically later but depends on earlier-named
	writeDepPlugin(t, root, "aaa", "")

	if err := m.LoadFromDisk(root); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	for _, want := range []string{"aaa", "bbb"} {
		if _, err := m.Get(want); err != nil {
			t.Errorf("plugin %q not registered after load: %v", want, err)
		}
	}
}

// TestLoadFromDiskMissingDependency excludes only the broken plugin: its
// dependent must not register, but unrelated plugins still do.
func TestLoadFromDiskMissingDependency(t *testing.T) {
	m := depTestManager(t)
	root := t.TempDir()
	writeDepPlugin(t, root, "needy", "ghost")
	writeDepPlugin(t, root, "solo", "")

	if err := m.LoadFromDisk(root); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	if _, err := m.Get("needy"); err == nil {
		t.Fatal("plugin with missing dependency must not register")
	}
	if _, err := m.Get("solo"); err != nil {
		t.Fatalf("unrelated plugin blocked by neighbour's bad dependency: %v", err)
	}
}

// TestLoadFromDiskCycleDetection rejects a two-plugin cycle while letting an
// independent third plugin through.
func TestLoadFromDiskCycleDetection(t *testing.T) {
	m := depTestManager(t)
	root := t.TempDir()
	writeDepPlugin(t, root, "cycA", "cycB")
	writeDepPlugin(t, root, "cycB", "cycA")
	writeDepPlugin(t, root, "independent", "")

	if err := m.LoadFromDisk(root); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	for _, name := range []string{"cycA", "cycB"} {
		if _, err := m.Get(name); err == nil {
			t.Errorf("cycle member %q must not register", name)
		}
	}
	if _, err := m.Get("independent"); err != nil {
		t.Fatalf("independent plugin blocked by cycle: %v", err)
	}
}

// TestCheckRegistrationDirectPath exercises the direct-Register guard: self
// dependency and missing dependency are rejected before any side effects.
func TestCheckRegistrationDirectPath(t *testing.T) {
	m := depTestManager(t)
	self := &Manifest{Name: "selfdep", Version: "1.0.0", Type: "command",
		Entry: "main.py", Interpreter: "python", Requires: []string{"selfdep"}}
	if err := m.Register(self); err == nil || !strings.Contains(err.Error(), "depends on itself") {
		t.Fatalf("self dependency: %v", err)
	}

	missing := &Manifest{Name: "needsGhost", Version: "1.0.0", Type: "command",
		Entry: "main.py", Interpreter: "python", Requires: []string{"ghost"}}
	if err := m.Register(missing); err == nil || !strings.Contains(err.Error(), "not loaded") {
		t.Fatalf("missing dependency: %v", err)
	}

	// A satisfied dependency registers fine and then blocks a duplicate.
	ok := &Manifest{Name: "base", Version: "1.0.0", Type: "command",
		Entry: "main.py", Interpreter: "python"}
	if err := m.Register(ok); err != nil {
		t.Fatalf("base register: %v", err)
	}
	if err := m.Register(&Manifest{Name: "base", Version: "2.0.0", Type: "command",
		Entry: "main.py", Interpreter: "python"}); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate: %v", err)
	}
}
