package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/testutil"
)

func metricAlertTestServer(t *testing.T) (*Server, *metricAlertEngine, *time.Time) {
	t.Helper()
	s := &Server{db: testutil.SetupTestDB(t)}
	s.metrics = NewMetricsCollector(s)
	cfg := &config.Config{}
	cfg.Monitoring.AlertsEnabled = true
	cfg.Monitoring.AgentsOnlineMin = 0
	cfg.Monitoring.TasksPendingMax = 100
	s.cfg = cfg

	engine := newMetricAlertEngine(time.Minute)
	clock := time.Now()
	engine.now = func() time.Time { return clock }
	return s, engine, &clock
}

// TestMetricAlertEvaluation covers threshold crossing, healthy silence and
// per-rule cooldown suppression with a deterministic clock.
func TestMetricAlertEvaluation(t *testing.T) {
	s, engine, clock := metricAlertTestServer(t)

	// Healthy state: agents online, backlog low -> no alerts.
	s.metrics.AgentsTotal.Set(3)
	s.metrics.AgentsOnline.Set(3)
	s.metrics.TasksPending.Set(5)
	if hits := engine.evaluate(s); len(hits) != 0 {
		t.Fatalf("healthy state fired %v", hits)
	}

	// All agents vanish while registrations exist + backlog explodes.
	s.metrics.AgentsOnline.Set(0)
	s.metrics.TasksPending.Set(150)
	hits := engine.evaluate(s)
	if len(hits) != 2 {
		t.Fatalf("hits = %+v, want both rules", hits)
	}
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.Rule] = true
	}
	if !seen["agents_online_min"] || !seen["tasks_pending_max"] {
		t.Fatalf("missing expected rules: %+v", hits)
	}

	// Same instant again: cooldown suppresses everything.
	if hits := engine.evaluate(s); len(hits) != 0 {
		t.Fatalf("cooldown not honored: %+v", hits)
	}

	// Advance past half the window: still suppressed.
	*clock = clock.Add(30 * time.Second)
	if hits := engine.evaluate(s); len(hits) != 0 {
		t.Fatalf("cooldown expired early: %+v", hits)
	}

	// Advance past the full window: both fire again.
	*clock = clock.Add(31 * time.Second)
	if hits := engine.evaluate(s); len(hits) != 2 {
		t.Fatalf("rules did not re-fire after cooldown: %+v", hits)
	}

	// Recovery: healthy values never fire even after cooldown.
	s.metrics.AgentsOnline.Set(3)
	s.metrics.TasksPending.Set(1)
	*clock = clock.Add(time.Hour)
	if hits := engine.evaluate(s); len(hits) != 0 {
		t.Fatalf("recovered state fired: %+v", hits)
	}
}

// TestMetricAlertZeroAgentsNoAlert pins the guard against alerting on an
// empty deployment: zero online with zero registered agents is normal.
func TestMetricAlertZeroAgentsNoAlert(t *testing.T) {
	s, engine, _ := metricAlertTestServer(t)
	s.metrics.AgentsTotal.Set(0)
	s.metrics.AgentsOnline.Set(0)
	s.metrics.TasksPending.Set(0)
	if hits := engine.evaluate(s); len(hits) != 0 {
		t.Fatalf("empty deployment fired: %+v", hits)
	}
}

// TestMetricAlertLoopDisabledWithoutConfig pins the opt-in behavior: the
// background loop must not start when monitoring.alerts_enabled is false or
// when no metrics collector exists.
func TestMetricAlertLoopDisabledWithoutConfig(t *testing.T) {
	s := &Server{db: testutil.SetupTestDB(t)}
	s.cfg = &config.Config{} // AlertsEnabled=false
	s.startMetricAlertLoop() // must be a no-op, no panic

	s.metrics = NewMetricsCollector(s)
	s.cfg.Monitoring.AlertsEnabled = true
	s.startMetricAlertLoop() // starts cleanly
}
