//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"testing"
	"time"
)

// TestWaitForBeaconWakeInterruptible verifies the "sleep-by-delay" behaviour:
// a long nominal sleep must return promptly when beaconWake is signalled,
// so operator actions (e.g. beacon_now) are not blocked behind a full interval.
func TestWaitForBeaconWakeInterruptible(t *testing.T) {
	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		select {
		case beaconWake <- struct{}{}:
		default:
		}
	}()
	waitForBeaconWake(10 * time.Second)
	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Fatalf("waitForBeaconWake did not return on beaconWake promptly (elapsed=%v)", elapsed)
	}
}
