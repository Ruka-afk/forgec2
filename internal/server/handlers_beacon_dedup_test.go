package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/testutil"
)

// TestBeaconDedupKeyedBySeq verifies that two beacons with identical payloads
// but different sequence numbers are NOT treated as duplicates (otherwise a
// legitimate retry with a new seq whose response was dropped would lose its
// tasks), while the same (uuid, seq) repeated within the window IS deduped.
func TestBeaconDedupKeyedBySeq(t *testing.T) {
	s := &Server{
		db:               testutil.SetupTestDB(t),
		beaconDedupCache: make(map[string]time.Time),
	}
	base := beaconRequest{
		UUID: "dedup-seq-test-uuid",
		Results: []taskResult{
			{TaskID: 1, Output: "same output"},
		},
	}

	// Distinct seq numbers must not collide.
	a := base
	a.Seq = 10
	if s.isDuplicateBeacon(a) {
		t.Fatalf("first beacon (seq=10) must not be a duplicate")
	}
	b := base
	b.Seq = 11
	if s.isDuplicateBeacon(b) {
		t.Fatalf("different seq (11) with identical payload must not be a duplicate")
	}

	// Same (uuid, seq) within the window must be deduped.
	c := base
	c.Seq = 11
	if !s.isDuplicateBeacon(c) {
		t.Fatalf("repeated (uuid, seq=11) must be a duplicate")
	}

	// A third distinct seq is again not a duplicate.
	d := base
	d.Seq = 12
	if s.isDuplicateBeacon(d) {
		t.Fatalf("new seq (12) must not be a duplicate")
	}

	// Sanity: the cache key includes the uuid so different agents don't collide.
	other := base
	other.UUID = "another-agent"
	other.Seq = 12
	if s.isDuplicateBeacon(other) {
		t.Fatalf("different agent with same seq must not collide")
	}
}
