package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// requireWriteAccounting skips on platforms without plugin write accounting
// (everything but Linux and Windows, see io_other.go).
func requireWriteAccounting(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("plugin write accounting is unsupported on %s", runtime.GOOS)
	}
}

// TestPluginWriteQuotaKillsRunawayWriter proves a plugin that streams to disk
// is killed once it spends its write budget instead of filling the volume for
// the rest of its wall-clock window, and that the failure is reported as a
// write-budget kill (not as a timeout, and not as a generic crash).
func TestPluginWriteQuotaKillsRunawayWriter(t *testing.T) {
	requireWriteAccounting(t)
	interp := requireHostInterpreter(t)

	dir := t.TempDir()
	writeTempPlugin(t, dir, "flood.py", strings.Join([]string{
		"f = open('out.bin', 'wb')",
		"chunk = b'x' * (1 << 20)",
		"while True:",
		"    f.write(chunk)",
		"    f.flush()",
	}, "\n")+"\n")
	m := &Manifest{Name: "flood", Interpreter: interp, Entry: "flood.py"}

	e := &executor{
		cpuQuota:          120 * time.Second, // keep the CPU watchdog out of the way
		writeQuota:        8 << 20,           // 8 MiB
		quotaPollInterval: 20 * time.Millisecond,
	}
	start := time.Now()
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 300)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a write-budget error")
	}
	if !strings.Contains(err.Error(), "write budget") {
		t.Fatalf("expected a write-budget error, got %v", err)
	}
	if res != nil && res.TimedOut {
		t.Fatal("run must be reported as a write kill, not as a wall-clock timeout")
	}
	if elapsed > 30*time.Second {
		t.Fatalf("write-budget enforcement took %s; process tree was not stopped", elapsed)
	}
	// The point of the budget: the file stops growing near the budget instead
	// of at whatever the disk can absorb in the full wall-clock window.
	if info, statErr := os.Stat(filepath.Join(dir, "out.bin")); statErr == nil && info.Size() > 256<<20 {
		t.Fatalf("runaway writer wrote %d bytes; not stopped at the budget", info.Size())
	}
}

// TestPluginWriteQuotaLeavesSmallWritesAlone pins the other side of the
// budget: a plugin whose writes stay inside it keeps succeeding.
func TestPluginWriteQuotaLeavesSmallWritesAlone(t *testing.T) {
	interp := requireHostInterpreter(t)

	dir := t.TempDir()
	writeTempPlugin(t, dir, "small.py", strings.Join([]string{
		"open('small.bin', 'wb').write(b'y' * (256 * 1024))",
		"print('{\"success\": true, \"output\": \"wrote\"}')",
	}, "\n")+"\n")
	m := &Manifest{Name: "small", Interpreter: interp, Entry: "small.py"}

	e := &executor{
		cpuQuota:          2 * time.Second,
		writeQuota:        4 << 20,
		quotaPollInterval: 50 * time.Millisecond,
	}
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 30)
	if err != nil {
		t.Fatalf("run within budget failed: %v", err)
	}
	if res == nil || !strings.Contains(string(res.Stdout), "wrote") {
		t.Fatalf("unexpected output: %+v", res)
	}
}
