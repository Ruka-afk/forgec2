package main

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestNetworkFingerprintStable guards against a wake storm: repeated polls of
// an unchanging host must produce the identical fingerprint.
func TestNetworkFingerprintStable(t *testing.T) {
	first := networkFingerprint()
	if first == "" {
		t.Skip("host has no non-loopback interfaces to fingerprint")
	}
	if second := networkFingerprint(); second != first {
		t.Fatalf("fingerprint changed without a network change:\n%q\n%q", first, second)
	}
}

// runMonitor starts the monitor and returns a channel closed when it exits.
func runMonitor(t *testing.T, interval time.Duration, fingerprint func() string, wake func()) {
	t.Helper()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitorNetworkChanges(interval, 0, fingerprint, wake, stop)
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})
}

// TestMonitorNetworkChangesWakesOnStableChange verifies the monitor wakes once
// a change has been observed twice, and then goes quiet while it stays put.
func TestMonitorNetworkChangesWakesOnStableChange(t *testing.T) {
	var calls atomic.Int64
	woken := make(chan struct{}, 4)
	runMonitor(t, 2*time.Millisecond, func() string {
		if calls.Add(1) < 4 {
			return "iface-a|up"
		}
		return "iface-b|up"
	}, func() { woken <- struct{}{} })

	select {
	case <-woken:
	case <-time.After(2 * time.Second):
		t.Fatal("stable network change did not wake the agent")
	}

	// Stable again: no second wake.
	select {
	case <-woken:
		t.Fatal("monitor woke more than once for a single network change")
	case <-time.After(30 * time.Millisecond):
	}
}

// TestMonitorNetworkChangesIgnoresFlapping ensures a link that alternates
// between two states on every poll never looks like a confirmed move.
func TestMonitorNetworkChangesIgnoresFlapping(t *testing.T) {
	var calls atomic.Int64
	woken := make(chan struct{}, 1)
	runMonitor(t, time.Millisecond, func() string {
		if calls.Add(1)%2 == 0 {
			return "iface-a|up"
		}
		return "iface-b|up"
	}, func() { woken <- struct{}{} })

	select {
	case <-woken:
		t.Fatal("flapping interface must not wake the agent")
	case <-time.After(60 * time.Millisecond):
	}
}

// TestMonitorNetworkChangesRateLimitsWakes proves a sequence of genuine moves
// is coalesced to one wake inside the minimum interval.
func TestMonitorNetworkChangesRateLimitsWakes(t *testing.T) {
	states := []string{"a", "b", "b", "c", "c", "d", "d"}
	var calls atomic.Int64
	var wakes atomic.Int64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		monitorNetworkChanges(time.Millisecond, time.Hour, func() string {
			n := int(calls.Add(1)) - 1
			if n >= len(states) {
				n = len(states) - 1
			}
			return states[n]
		}, func() { wakes.Add(1) }, stop)
	}()
	defer func() {
		close(stop)
		<-done
	}()

	// One confirmed wake (b) in the first hour; c and d are suppressed.
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) && calls.Load() < int64(len(states))+2 {
		time.Sleep(time.Millisecond)
	}
	if got := wakes.Load(); got != 1 {
		t.Fatalf("wake count = %d, want 1 (min interval must coalesce moves)", got)
	}
}

// TestMonitorNetworkChangesIgnoresUnknownFingerprint ensures a transient OS
// error (empty fingerprint) never counts as a change.
func TestMonitorNetworkChangesIgnoresUnknownFingerprint(t *testing.T) {
	woken := make(chan struct{}, 1)
	runMonitor(t, time.Millisecond, func() string { return "" },
		func() { woken <- struct{}{} })

	select {
	case <-woken:
		t.Fatal("empty fingerprint must not wake the agent")
	case <-time.After(30 * time.Millisecond):
	}
}

// TestWakeBeaconLoopDrainsBothChannels pins that either sleep path is
// interruptible and that repeated wakes never block.
func TestWakeBeaconLoopDrainsBothChannels(t *testing.T) {
	select {
	case <-beaconWake:
	default:
	}
	select {
	case <-beaconWakeCh:
	default:
	}

	wakeBeaconLoop()
	wakeBeaconLoop() // must not block on the full capacity-1 channels

	select {
	case <-beaconWake:
	default:
		t.Fatal("beaconWake was not signalled")
	}
	select {
	case <-beaconWakeCh:
	default:
		t.Fatal("beaconWakeCh was not signalled")
	}
}
