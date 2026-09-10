//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"testing"
	"time"
)

func TestParseStartDelayWindow(t *testing.T) {
	cases := []struct {
		minStr, maxStr   string
		wantMin, wantMax int
	}{
		{"", "", 0, 0},
		{"5", "15", 5, 15},
		{"-3", "10", 0, 10},
		{"20", "10", 10, 10},       // min clamped to max
		{"9999", "9999", 600, 600}, // capped
		{"abc", "10", 0, 10},       // unparsable -> 0
	}
	for _, c := range cases {
		gotMin, gotMax := parseStartDelayWindow(c.minStr, c.maxStr)
		if gotMin != c.wantMin || gotMax != c.wantMax {
			t.Fatalf("parseStartDelayWindow(%q,%q) = (%d,%d), want (%d,%d)",
				c.minStr, c.maxStr, gotMin, gotMax, c.wantMin, c.wantMax)
		}
	}
}

func TestStartDelayDuration(t *testing.T) {
	if d := startDelayDuration(0, 0, func(uint32) uint32 { return 0 }); d != 0 {
		t.Fatalf("disabled window must yield 0, got %v", d)
	}
	if d := startDelayDuration(10, 10, func(uint32) uint32 { return 99 }); d != 10*time.Second {
		t.Fatalf("fixed window must yield exact duration, got %v", d)
	}
	// Range draw stays inside [min, max].
	for _, r := range []uint32{0, 5, 10} {
		d := startDelayDuration(5, 15, func(n uint32) uint32 {
			if n != 11 {
				t.Fatalf("span must be max-min+1=11, got %d", n)
			}
			return r
		})
		if d < 5*time.Second || d > 15*time.Second {
			t.Fatalf("draw out of window: %v", d)
		}
	}
}
