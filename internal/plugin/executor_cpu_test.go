package plugin

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// requireCPUAccounting skips on platforms without plugin CPU accounting
// (everything but Linux and Windows, see cpu_other.go).
func requireCPUAccounting(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("plugin CPU accounting is unsupported on %s", runtime.GOOS)
	}
}

// shortCPUQuotaPollInterval shrinks the watchdog's sampling interval for the
// duration of a test so budget enforcement lands quickly.
func shortCPUQuotaPollInterval(t *testing.T, d time.Duration) {
	t.Helper()
	old := cpuQuotaPollInterval
	cpuQuotaPollInterval = d
	t.Cleanup(func() { cpuQuotaPollInterval = old })
}

// TestPluginCPUQuotaKillsRunawayPlugin proves a plugin that spins CPU is
// killed once it burns its budget instead of running out the wall-clock
// window, and that the failure is reported as a CPU-budget kill (not as a
// timeout, and not as a generic crash).
func TestPluginCPUQuotaKillsRunawayPlugin(t *testing.T) {
	requireCPUAccounting(t)
	interp := requireHostInterpreter(t)
	shortCPUQuotaPollInterval(t, 50*time.Millisecond)

	dir := t.TempDir()
	writeTempPlugin(t, dir, "spin.py", "while True:\n    pass\n")
	m := &Manifest{Name: "spin", Interpreter: interp, Entry: "spin.py"}

	e := &executor{cpuQuota: 1 * time.Second}
	start := time.Now()
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 120)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a CPU-budget error")
	}
	if !strings.Contains(err.Error(), "CPU budget") {
		t.Fatalf("expected a CPU-budget error, got %v", err)
	}
	if res != nil && res.TimedOut {
		t.Fatal("run must be reported as a CPU kill, not as a wall-clock timeout")
	}
	if elapsed > 30*time.Second {
		t.Fatalf("CPU-budget enforcement took %s; process tree was not stopped", elapsed)
	}
}

// TestPluginCPUQuotaLeavesNormalRunsAlone pins the other side of the budget: a
// plugin that finishes well within it keeps succeeding.
func TestPluginCPUQuotaLeavesNormalRunsAlone(t *testing.T) {
	interp := requireHostInterpreter(t)
	shortCPUQuotaPollInterval(t, 50*time.Millisecond)

	dir := t.TempDir()
	writeTempPlugin(t, dir, "ok.py", "print('{\"success\": true, \"output\": \"ok\"}')\n")
	m := &Manifest{Name: "ok", Interpreter: interp, Entry: "ok.py"}

	e := &executor{cpuQuota: 2 * time.Second}
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 30)
	if err != nil {
		t.Fatalf("run within budget failed: %v", err)
	}
	if res == nil || !strings.Contains(string(res.Stdout), "\"ok\"") {
		t.Fatalf("unexpected output: %+v", res)
	}
}
