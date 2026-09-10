//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "testing"

// TestAPIHashVectors pins the FNV-1a digests embedded at windows call sites.
// If the hash function ever changes, windows resolution silently breaks, so
// vectors are tested on every platform.
func TestAPIHashVectors(t *testing.T) {
	vectors := map[string]uint32{
		"NtQueryInformationProcess": 0xea2dda8a,
		"NtSetInformationThread":    0xf19e1321,
		"NtClose":                   0x6b372c05,
		"GetCurrentThread":          0x9bc5a608,
		"QueueUserAPC":              0x890bb4fb,
	}
	for name, want := range vectors {
		if got := fnv1a32(name); got != want {
			t.Fatalf("fnv1a32(%q) = 0x%08x, want 0x%08x", name, got, want)
		}
	}
}
