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

// TestMonitorNetworkChangesWakesOnChange verifies the monitor calls wake once
// when the fingerprint changes and then goes quiet while it stays put.
func TestMonitorNetworkChangesWakesOnChange(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	defer func() {
		close(stop)
		<-done
	}()

	var calls atomic.Int64
	woken := make(chan struct{}, 4)
	go func() {
		defer close(done)
		monitorNetworkChanges(2*time.Millisecond, func() string {
			if calls.Add(1) < 3 {
				return "iface-a|up"
			}
			return "iface-b|up"
		}, func() { woken <- struct{}{} }, stop)
	}()

	select {
	case <-woken:
	case <-time.After(2 * time.Second):
		t.Fatal("network change did not wake the agent")
	}

	// Stable again: no second wake.
	select {
	case <-woken:
		t.Fatal("monitor woke more than once for a single network change")
	case <-time.After(30 * time.Millisecond):
	}
}

// TestMonitorNetworkChangesIgnoresUnknownFingerprint ensures a transient OS
// error (empty fingerprint) never counts as a change.
func TestMonitorNetworkChangesIgnoresUnknownFingerprint(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	defer func() {
		close(stop)
		<-done
	}()

	woken := make(chan struct{}, 1)
	go func() {
		defer close(done)
		monitorNetworkChanges(time.Millisecond, func() string { return "" },
			func() { woken <- struct{}{} }, stop)
	}()

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
