package server

import "testing"

func TestListenerHealthTrackerCountsConsecutiveFailures(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}

	first := &listenerHealth{ListenerID: 7, Error: "first failure"}
	tracker.observe(first)
	if first.Failures != 1 {
		t.Fatalf("first failure count = %d, want 1", first.Failures)
	}

	second := &listenerHealth{ListenerID: 7, Error: "second failure"}
	tracker.observe(second)
	if second.Failures != listenerFailThreshold {
		t.Fatalf("second failure count = %d, want %d", second.Failures, listenerFailThreshold)
	}

	recovered := &listenerHealth{ListenerID: 7, OK: true}
	tracker.observe(recovered)
	if recovered.Failures != 0 {
		t.Fatalf("successful probe retained %d failures", recovered.Failures)
	}

	afterRecovery := &listenerHealth{ListenerID: 7, Error: "new sequence"}
	tracker.observe(afterRecovery)
	if afterRecovery.Failures != 1 {
		t.Fatalf("failure after recovery count = %d, want 1", afterRecovery.Failures)
	}
}

func TestListenerHealthTrackerSkippedProbeResetsFailures(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 3, Failures: 9})

	skipped := &listenerHealth{ListenerID: 3, OK: true, Skipped: true, Failures: 9}
	tracker.observe(skipped)
	if skipped.Failures != 0 {
		t.Fatalf("skipped probe retained %d failures", skipped.Failures)
	}
}

func TestListenerHealthTrackerPrunesAndSortsSnapshot(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 9, OK: true})
	tracker.observe(&listenerHealth{ListenerID: 2, OK: true})
	tracker.observe(&listenerHealth{ListenerID: 5, OK: true})

	tracker.prune(map[uint]struct{}{9: {}, 2: {}})
	snapshot := tracker.snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot length = %d, want 2", len(snapshot))
	}
	if snapshot[0].ListenerID != 2 || snapshot[1].ListenerID != 9 {
		t.Fatalf("snapshot order = [%d, %d], want [2, 9]", snapshot[0].ListenerID, snapshot[1].ListenerID)
	}
}
