//go:build windows

package main

import (
	"strings"
	"testing"
)

// TestSetPowerShellHostAssembly verifies the runtime host-assembly installer
// wires into the global consumed by runPowerShellInProcess/powerPick.
func TestSetPowerShellHostAssembly(t *testing.T) {
	prev := powershellHostAssembly
	defer func() { powershellHostAssembly = prev }()

	SetPowerShellHostAssembly([]byte("FAKEHOST"))
	if len(powershellHostAssembly) != 8 {
		t.Fatalf("expected host assembly set, got len=%d", len(powershellHostAssembly))
	}
}

// TestRunPowerShellInProcessFallback exercises the managed powershell.exe
// fallback path (used when no unmanaged host assembly is configured). It
// confirms runPowerShellInProcess round-trips the script through powerPick and
// captures stdout, which is the last-resort behaviour when unmanaged PS is
// unavailable. The true unmanaged path is covered by live/runtime testing.
func TestRunPowerShellInProcessFallback(t *testing.T) {
	prev := powershellHostAssembly
	defer func() { powershellHostAssembly = prev }()
	powershellHostAssembly = nil

	out, err := runPowerShellInProcess("Write-Output TEST_UNMANAGED_PS")
	if err != nil {
		t.Fatalf("runPowerShellInProcess returned error: %v", err)
	}
	if !strings.Contains(out, "TEST_UNMANAGED_PS") {
		t.Fatalf("expected captured output to contain marker, got: %q", out)
	}
}
