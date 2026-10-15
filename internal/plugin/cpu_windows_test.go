//go:build windows
// +build windows

package plugin

import (
	"os/exec"
	"testing"
	"time"
)

// TestWindowsJobCPUTimeAccountsTree proves the Job Object accounting used by
// the CPU watchdog reports the CPU its processes actually burned: a spinning
// child inside the job pushes cpuTime() above zero, and a released guard
// reports itself gone instead of lying with a stale number.
func TestWindowsJobCPUTimeAccountsTree(t *testing.T) {
	interp := requireHostInterpreter(t)
	cmd := exec.Command(interp, "-c", "while True: pass")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start spinner: %v", err)
	}
	guard, err := attachProcessGuard(cmd.Process)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("attach guard: %v", err)
	}
	defer func() {
		guard.release()
		_ = cmd.Wait()
	}()

	deadline := time.Now().Add(10 * time.Second)
	var used time.Duration
	for time.Now().Before(deadline) {
		used, err = guard.cpuTime()
		if err != nil {
			t.Fatalf("cpuTime: %v", err)
		}
		if used > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if used <= 0 {
		t.Fatalf("spinning child reported %s CPU; job accounting is not wired", used)
	}

	guard.release()
	if _, err := guard.cpuTime(); err == nil {
		t.Fatal("a released guard must not report CPU time")
	}
}
