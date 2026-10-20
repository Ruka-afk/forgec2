package main

import (
	"reflect"
	"testing"
	"time"
)

// fakeSleepMask records the mask lifecycle without touching real pages, so the
// chunking/interrupt logic can be tested without waiting out a sleep.
type fakeSleepMask struct {
	encryptions int
	decryptions int
	sleeps      []time.Duration
}

func (f *fakeSleepMask) Encrypt() { f.encryptions++ }
func (f *fakeSleepMask) Decrypt() { f.decryptions++ }
func (f *fakeSleepMask) BeforeSleep(durationMs uintptr) {
	f.sleeps = append(f.sleeps, time.Duration(durationMs)*time.Millisecond)
}
func (f *fakeSleepMask) AfterWake()   {}
func (f *fakeSleepMask) Name() string { return "fake" }

func drainBeaconWakeCh() {
	for {
		select {
		case <-beaconWakeCh:
		default:
			return
		}
	}
}

func withFakeSleepMask(t *testing.T) *fakeSleepMask {
	t.Helper()
	old := activeSleepMask
	f := &fakeSleepMask{}
	activeSleepMask = f
	drainBeaconWakeCh()
	t.Cleanup(func() {
		activeSleepMask = old
		drainBeaconWakeCh()
	})
	return f
}

func TestSleepWithMaskChunksFullDuration(t *testing.T) {
	f := withFakeSleepMask(t)

	sleepWithMaskInterruptible(11 * time.Second)

	want := []time.Duration{5 * time.Second, 5 * time.Second, 1 * time.Second}
	if !reflect.DeepEqual(f.sleeps, want) {
		t.Fatalf("mask chunks = %v, want %v", f.sleeps, want)
	}
	if f.encryptions != 1 || f.decryptions != 1 {
		t.Fatalf("encrypt/decrypt = %d/%d, want 1/1", f.encryptions, f.decryptions)
	}
}

func TestSleepWithMaskStopsOnWake(t *testing.T) {
	f := withFakeSleepMask(t)

	requestBeaconWake()
	sleepWithMaskInterruptible(11 * time.Second)

	if len(f.sleeps) != 1 || f.sleeps[0] != sleepMaskChunk {
		t.Fatalf("mask slept %v, want one %v chunk before the wake", f.sleeps, sleepMaskChunk)
	}
	if f.encryptions != 1 || f.decryptions != 1 {
		t.Fatalf("encrypt/decrypt = %d/%d, want 1/1", f.encryptions, f.decryptions)
	}
}
