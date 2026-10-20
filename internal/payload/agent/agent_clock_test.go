package main

import (
	"net/http"
	"testing"
	"time"
)

// TestNoteServerClockLearnsAndResetsOffset pins the two properties the beacon
// relies on: a skewed host learns the server's clock from a rejection's Date
// header, and a later NTP correction brings the offset back to zero instead of
// accumulating.
func TestNoteServerClockLearnsAndResetsOffset(t *testing.T) {
	prev := serverClockOffsetSeconds.Load()
	defer serverClockOffsetSeconds.Store(prev)
	serverClockOffsetSeconds.Store(0)

	h := http.Header{}
	h.Set("Date", time.Now().Add(2*time.Hour).UTC().Format(http.TimeFormat))
	noteServerClock(h)
	got := beaconTimestamp() - time.Now().Unix()
	if got < 7197 || got > 7203 {
		t.Fatalf("beacon timestamp offset = %d, want ~7200", got)
	}

	// A host clock that is ahead of the server must correct downwards.
	serverClockOffsetSeconds.Store(0)
	h.Set("Date", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
	noteServerClock(h)
	if off := serverClockOffsetSeconds.Load(); off > -3597 || off < -3603 {
		t.Fatalf("negative skew offset = %d, want ~-3600", off)
	}

	// After the host clock is fixed the next response resets the offset.
	h.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	noteServerClock(h)
	if off := serverClockOffsetSeconds.Load(); off != 0 {
		t.Fatalf("offset after clock correction = %d, want 0", off)
	}
}

// TestNoteServerClockIgnoresInvalidHeader keeps a hostile or broken proxy from
// poisoning the timestamp with an unparsable Date.
func TestNoteServerClockIgnoresInvalidHeader(t *testing.T) {
	prev := serverClockOffsetSeconds.Load()
	defer serverClockOffsetSeconds.Store(prev)
	serverClockOffsetSeconds.Store(0)

	noteServerClock(http.Header{})
	noteServerClock(http.Header{"Date": []string{"not-a-date"}})
	if off := serverClockOffsetSeconds.Load(); off != 0 {
		t.Fatalf("invalid Date changed the offset to %d", off)
	}
}
