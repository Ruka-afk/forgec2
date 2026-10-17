//go:build !windows

package main

import (
	"bytes"
	"testing"
)

func resetShadowState(t *testing.T) {
	t.Helper()
	smAltMu.Lock()
	smAltBufInit = false
	smAltBuf = nil
	smAltKey = nil
	smAltIdx = 0
	smAltMu.Unlock()
	t.Cleanup(func() {
		smAltMu.Lock()
		smAltBufInit = false
		smAltBuf = nil
		smAltKey = nil
		smAltIdx = 0
		smAltMu.Unlock()
	})
}

// TestShadowMaskRoundTrip proves the non-Windows config shadow is scrambled on
// the way into sleep and restored byte for byte on the way out.
func TestShadowMaskRoundTrip(t *testing.T) {
	resetShadowState(t)
	if !InitSleepMask() {
		t.Fatal("InitSleepMask() should report the shadow mask ready off-Windows")
	}
	sleepMaskEncrypt()
	if !smAltBufInit {
		t.Fatal("the first encrypt pass must build the shadow buffer")
	}
	plain := buildAltShadow()
	if len(smAltBuf) != len(plain) {
		t.Fatalf("shadow length = %d, want %d", len(smAltBuf), len(plain))
	}
	if bytes.Equal(smAltBuf, plain) {
		t.Fatal("the encrypt pass must actually scramble the shadow")
	}
	sleepMaskDecrypt()
	if !bytes.Equal(smAltBuf, plain) {
		t.Fatal("the decrypt pass must restore the shadow exactly")
	}
}

func TestSleepMaskPolicyDisabledOffWindows(t *testing.T) {
	if sleepMaskPolicyEnabled() {
		t.Fatal("off-Windows there is no EDR-strategy opt-in; activation is operator-only")
	}
}
