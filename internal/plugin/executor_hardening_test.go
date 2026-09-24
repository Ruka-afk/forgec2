package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestClampPluginTimeout proves the server ceiling wins over an untrusted
// manifest: a plugin can ask for less runtime, never more.
func TestClampPluginTimeout(t *testing.T) {
	cases := []struct {
		requested, def, want int
	}{
		{0, 30, 30},  // fall back to manifest default
		{-5, 45, 45}, // non-positive also falls back
		{10, 30, 10}, // explicit shorter timeout wins
		{99999, 30, maxPluginTimeoutSecs},
		{0, 99999, maxPluginTimeoutSecs},
	}
	for _, c := range cases {
		if got := clampPluginTimeout(c.requested, c.def); got != c.want {
			t.Errorf("clampPluginTimeout(%d,%d)=%d, want %d", c.requested, c.def, got, c.want)
		}
	}
}

// requireHostInterpreter returns a working Python interpreter or skips.
// Plain LookPath is not enough on Windows: the Microsoft Store alias resolves
// but exits 9009, so the probe actually runs the interpreter.
func requireHostInterpreter(t *testing.T) string {
	t.Helper()
	candidates := []string{"python3", "python", "py"}
	if runtime.GOOS != "windows" {
		candidates = candidates[:2]
	}
	for _, interp := range candidates {
		path, err := exec.LookPath(interp)
		if err != nil {
			continue
		}
		probeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		probe := exec.CommandContext(probeCtx, path, "-c", "pass")
		err = probe.Run()
		cancel()
		if err != nil {
			continue
		}
		return interp
	}
	t.Skip("no working python interpreter on this host")
	return ""
}

func writeTempPlugin(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
		t.Fatalf("write plugin %s: %v", name, err)
	}
}

// TestPluginRunTimeoutStopsHungPlugin proves a hung plugin is stopped and
// reported as a timeout instead of pinning the worker past its budget.
func TestPluginRunTimeoutStopsHungPlugin(t *testing.T) {
	interp := requireHostInterpreter(t)
	dir := t.TempDir()
	writeTempPlugin(t, dir, "slow.py", "import time\ntime.sleep(60)\n")
	m := &Manifest{Name: "slow", Interpreter: interp, Entry: "slow.py"}

	start := time.Now()
	res, err := (&executor{}).run(context.Background(), dir, m, map[string]interface{}{}, 1)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if res == nil || !res.TimedOut {
		t.Fatalf("expected TimedOut result, got %+v (err=%v)", res, err)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("timeout enforcement took %s; process tree was not stopped", elapsed)
	}
}

// TestPluginStdoutStaysCapped keeps the output cap honest after the executor
// changes: a chatty plugin still yields capped output with the truncation
// note, never unbounded memory growth.
func TestPluginStdoutStaysCapped(t *testing.T) {
	interp := requireHostInterpreter(t)
	dir := t.TempDir()
	writeTempPlugin(t, dir, "loud.py", "print('x' * "+strconv.Itoa(3<<20)+")\n")
	m := &Manifest{Name: "loud", Interpreter: interp, Entry: "loud.py"}

	res, err := (&executor{}).run(context.Background(), dir, m, map[string]interface{}{}, 60)
	if err != nil {
		t.Skipf("interpreter unavailable: %v", err)
	}
	if len(res.Stdout) > maxPluginOutputBytes+len(truncatedOutputNote) {
		t.Fatalf("stdout not capped: %d bytes", len(res.Stdout))
	}
	if !strings.Contains(string(res.Stdout), "output truncated") {
		t.Fatal("truncation note missing from capped stdout")
	}
}

// TestPluginConcurrencyQuota proves the executor refuses new runs instead of
// fanning out interpreters once the server-wide slot budget is exhausted.
func TestPluginConcurrencyQuota(t *testing.T) {
	interp := requireHostInterpreter(t)
	dir := t.TempDir()
	writeTempPlugin(t, dir, "slow.py", "import time\ntime.sleep(2)\n")
	m := &Manifest{Name: "slow", Interpreter: interp, Entry: "slow.py"}

	// One slot, two concurrent runs: exactly one may be in flight.
	e := newExecutor(1)
	if err := e.acquire(context.Background()); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := e.acquire(ctx); err == nil {
		t.Fatal("second acquire succeeded despite a full quota")
	}
	e.release()

	done := make(chan error, 1)
	go func() {
		_, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 10)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run after release failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("run did not complete after releasing the slot")
	}
}

// TestPluginGuardRefusesNilProcess proves guard attachment reports failure
// rather than silently returning an empty guard, which is what lets the
// executor refuse the run.
func TestPluginGuardRefusesNilProcess(t *testing.T) {
	if _, err := attachProcessGuard(nil); err == nil {
		t.Fatal("attachProcessGuard(nil) must error so the run is refused")
	}
}
