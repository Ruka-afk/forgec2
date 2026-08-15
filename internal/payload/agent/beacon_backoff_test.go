package main

import "testing"

// TestBeaconBackoffSecNeverOverflows verifies the backoff never goes negative
// or overflows, even at absurd consecutive-failure counts (the original
// implementation panicked via a negative time.Sleep at 64 failures).
func TestBeaconBackoffSecNeverOverflows(t *testing.T) {
	for failures := 1; failures <= 1000; failures++ {
		sec := beaconBackoffSec(failures)
		if sec <= 0 {
			t.Fatalf("backoff must stay positive at failures=%d (got %d)", failures, sec)
		}
		if sec > 300 {
			t.Fatalf("backoff must be capped at 300 at failures=%d (got %d)", failures, sec)
		}
	}
}

// TestBeaconBackoffSecProgression checks the expected 1,2,4,8...64,128,256,300
// progression and that the ceiling is hit once the exponent is clamped to 8.
func TestBeaconBackoffSecProgression(t *testing.T) {
	want := []struct {
		failures int
		backoff  int
	}{
		{1, 1}, {2, 2}, {3, 4}, {4, 8}, {5, 16},
		{6, 32}, {7, 64}, {8, 128}, {9, 256}, {10, 300}, {11, 300}, {64, 300},
	}
	for _, c := range want {
		if got := beaconBackoffSec(c.failures); got != c.backoff {
			t.Errorf("beaconBackoffSec(%d) = %d, want %d", c.failures, got, c.backoff)
		}
	}
}
