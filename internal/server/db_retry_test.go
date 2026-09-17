package server

import (
	"errors"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/testutil"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
)

// stubBusySleep replaces the retry backoff with a no-op so tests don't wait
// real time. It returns a counter of sleep calls (== retries performed).
func stubBusySleep(t *testing.T) *int {
	t.Helper()
	calls := 0
	old := busyRetrySleep
	busyRetrySleep = func(time.Duration) { calls++ }
	t.Cleanup(func() { busyRetrySleep = old })
	return &calls
}

func newRetryTestServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{db: testutil.SetupTestDB(t), agentPendingTasks: make(map[string]int)}
	s.metrics = NewMetricsCollector(s)
	return s
}

func TestWithBusyRetrySuccessFirstTry(t *testing.T) {
	s := newRetryTestServer(t)
	sleeps := stubBusySleep(t)
	calls := 0
	err := s.withBusyRetry("test", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 || *sleeps != 0 {
		t.Fatalf("calls=%d sleeps=%d, want 1/0", calls, *sleeps)
	}
	if n := promtestutil.CollectAndCount(s.metrics.DbBusyRetriesTotal); n != 0 {
		t.Fatalf("metric samples=%d, want 0", n)
	}
}

func TestWithBusyRetryRecoversAfterLock(t *testing.T) {
	s := newRetryTestServer(t)
	sleeps := stubBusySleep(t)
	calls := 0
	err := s.withBusyRetry("test", func() error {
		calls++
		if calls < 3 {
			return errors.New("database is locked")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 || *sleeps != 2 {
		t.Fatalf("calls=%d sleeps=%d, want 3/2", calls, *sleeps)
	}
	if n := promtestutil.CollectAndCount(s.metrics.DbBusyRetriesTotal); n == 0 {
		t.Fatal("expected retry metric samples")
	}
}

func TestWithBusyRetryExhausts(t *testing.T) {
	s := newRetryTestServer(t)
	sleeps := stubBusySleep(t)
	calls := 0
	err := s.withBusyRetry("test", func() error {
		calls++
		return errors.New("SQLITE_BUSY: database is locked")
	})
	if err == nil {
		t.Fatal("expected the lock error back")
	}
	if calls != dbRetryAttempts || *sleeps != dbRetryAttempts-1 {
		t.Fatalf("calls=%d sleeps=%d, want %d/%d", calls, *sleeps, dbRetryAttempts, dbRetryAttempts-1)
	}
}

func TestWithBusyRetryNoRetryOnOtherErrors(t *testing.T) {
	s := &Server{}
	sleeps := stubBusySleep(t)
	calls := 0
	sentinel := errors.New("unique violation")
	err := s.withBusyRetry("test", func() error {
		calls++
		return sentinel
	})
	if err != sentinel {
		t.Fatalf("err=%v, want sentinel", err)
	}
	if calls != 1 || *sleeps != 0 {
		t.Fatalf("calls=%d sleeps=%d, want 1/0", calls, *sleeps)
	}
}
