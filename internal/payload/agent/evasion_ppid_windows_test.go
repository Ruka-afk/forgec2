//go:build windows
// +build windows

package main

import "testing"

// decodeBypassPatch must produce a valid function stub (xor eax,eax; ret) that
// the AMSI/ETW session-bypass patches jump to after stubbing the real routine.
// A corrupted stub would leave the patched API returning garbage / crashing.
func TestDecodeBypassPatchIsXorEaxEaxRet(t *testing.T) {
	got := decodeBypassPatch()
	want := []byte{0x31, 0xC0, 0xC3}
	if len(got) != len(want) {
		t.Fatalf("decodeBypassPatch len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("decodeBypassPatch[%d] = 0x%02x, want 0x%02x (not 'xor eax,eax; ret')", i, got[i], want[i])
		}
	}
}
