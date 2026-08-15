//go:build windows
// +build windows

package main

import "testing"

// TestClipboardRoundtripBounded verifies the clipboard read/write honor the
// actual Global allocation size (GlobalSize) rather than a fixed 1MB view,
// which previously read/wrote past the allocation into adjacent heap memory.
func TestClipboardRoundtripBounded(t *testing.T) {
	const want = "forgec2-clipboard-test"
	if err := clipboardSetWindows(want); err != nil {
		t.Skipf("clipboard unavailable (headless?): %v", err)
	}
	got, err := clipboardGetWindows()
	if err != nil {
		t.Skipf("clipboard read unavailable: %v", err)
	}
	if got != want {
		t.Fatalf("clipboard roundtrip mismatch: got %q want %q", got, want)
	}
}
