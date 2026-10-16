package server

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestEventDropsCounted proves formerly-silent internal-bus sheds are visible
// in EventDroppedTotal: a saturated handler pool surfaces as handler_saturated
// and a full ingest queue surfaces as queue_full.
func TestEventDropsCounted(t *testing.T) {
	em := NewEventManager(nil)
	defer em.Shutdown()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_event_dropped_total"}, []string{"reason"})
	em.SetDropCounter(counter)

	// One handler that blocks forever: workerSem (32 slots) saturates, the
	// reader loop then sheds every further event as handler_saturated, and
	// the ingest queue (256) backfills until Emit starts reporting
	// queue_full. Emitting a multiple of the queue size forces both paths.
	release := make(chan struct{})
	em.On(EventTaskComplete, func(Event) { <-release })
	defer close(release)

	evt := Event{Type: EventTaskComplete, User: "metric-test"}
	for i := 0; i < eventQueueSize*3; i++ {
		em.Emit(evt)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		queueFull := testutil.ToFloat64(counter.WithLabelValues("queue_full"))
		saturated := testutil.ToFloat64(counter.WithLabelValues("handler_saturated"))
		if queueFull > 0 && saturated > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("drop metrics never fired: queue_full=%v handler_saturated=%v", queueFull, saturated)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
