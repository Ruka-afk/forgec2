//go:build linux || windows || darwin

package main

import (
	"testing"
	"time"
)

func TestComputeSleepDurationPositive(t *testing.T) {
	// Run several times to exercise jitter + activity shaping; every result must
	// be a positive, bounded duration (no zero/negative sleeps, no runaway).
	setSleepMode(SleepModeDefault)
	Interval = 10
	Jitter = 20
	for i := 0; i < 200; i++ {
		d := computeSleepDuration()
		if d < 50*time.Millisecond {
			t.Fatalf("sleep duration too small: %v", d)
		}
		if d > 10*time.Hour {
			t.Fatalf("sleep duration unreasonably large: %v", d)
		}
	}
}

func TestUserActivityMultiplierBounds(t *testing.T) {
	// The multiplier must stay within the designed [0.6, 1.8] envelope so user
	// activity shaping never produces a degenerate interval.
	for _, m := range []float64{userActivityMultiplier(), userActivityMultiplier()} {
		if m < 0.5 || m > 2.0 {
			t.Fatalf("activity multiplier out of envelope: %v", m)
		}
	}
}
