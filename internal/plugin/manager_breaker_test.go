package plugin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type stubHookPlugin struct {
	name  string
	calls *atomic.Int32
	err   error
}

func (s *stubHookPlugin) Description() string { return "test stub" }
func (s *stubHookPlugin) Version() string     { return "test" }
func (s *stubHookPlugin) Name() string        { return s.name }
func (s *stubHookPlugin) Type() string        { return "hook" }
func (s *stubHookPlugin) Init(context.Context, map[string]interface{}) error {
	return nil
}
func (s *stubHookPlugin) Shutdown() error { return nil }
func (s *stubHookPlugin) IsEnabled() bool { return true }
func (s *stubHookPlugin) SubscribedEvents() []string {
	return []string{"agent.connect"}
}
func (s *stubHookPlugin) OnEvent(context.Context, Event) error {
	s.calls.Add(1)
	return s.err
}

func TestHookBreakerTripsAfterConsecutiveFailures(t *testing.T) {
	m := NewManager(nil)
	var calls atomic.Int32
	bad := &stubHookPlugin{name: "bad", calls: &calls, err: errors.New("exit 9009")}
	m.mu.Lock()
	m.plugins["bad"] = bad
	m.mu.Unlock()

	evt := Event{Type: "agent.connect"}
	for i := 0; i < hookBreakerThreshold+2; i++ {
		_ = m.ExecuteHook(context.Background(), evt)
	}
	if got := calls.Load(); got != hookBreakerThreshold {
		t.Fatalf("expected exactly %d deliveries before trip, got %d", hookBreakerThreshold, got)
	}
	// Tripped: further events are skipped without delivery.
	_ = m.ExecuteHook(context.Background(), evt)
	if got := calls.Load(); got != hookBreakerThreshold {
		t.Fatalf("tripped breaker still delivered: got %d", got)
	}
}

func TestHookBreakerResetsOnSuccess(t *testing.T) {
	m := NewManager(nil)
	var calls atomic.Int32
	flaky := &stubHookPlugin{name: "flaky", calls: &calls, err: errors.New("boom")}
	m.mu.Lock()
	m.plugins["flaky"] = flaky
	m.mu.Unlock()

	evt := Event{Type: "agent.connect"}
	for i := 0; i < hookBreakerThreshold-1; i++ {
		_ = m.ExecuteHook(context.Background(), evt)
	}
	flaky.err = nil
	_ = m.ExecuteHook(context.Background(), evt)
	m.mu.RLock()
	n := m.hookFails["flaky"]
	m.mu.RUnlock()
	if n != 0 {
		t.Fatalf("success should reset failure count, got %d", n)
	}
}

func TestHookBreakerProbesAfterCooldown(t *testing.T) {
	m := NewManager(nil)
	var calls atomic.Int32
	bad := &stubHookPlugin{name: "bad", calls: &calls, err: errors.New("boom")}
	m.mu.Lock()
	m.plugins["bad"] = bad
	m.mu.Unlock()

	evt := Event{Type: "agent.connect"}
	for i := 0; i < hookBreakerThreshold; i++ {
		_ = m.ExecuteHook(context.Background(), evt)
	}
	// Force the trip into the past so the cooldown has passed.
	m.mu.Lock()
	m.hookTrippedAt["bad"] = time.Now().Add(-2 * hookBreakerCooldown)
	m.mu.Unlock()
	_ = m.ExecuteHook(context.Background(), evt)
	if got := calls.Load(); got != hookBreakerThreshold+1 {
		t.Fatalf("expected one probe delivery after cooldown, got %d", got)
	}
}
