package server

import (
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
)

// seedTerminalTask inserts one completed/failed task inside or outside the
// failure-rate look-back window.
func seedTerminalTask(t *testing.T, s *Server, status string, ageMinutes int) {
	t.Helper()
	task := db.Task{AgentID: "a1", Type: "shell", Status: status}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed %s task: %v", status, err)
	}
	if ageMinutes > 0 {
		back := time.Now().Add(-time.Duration(ageMinutes) * time.Minute)
		if err := s.db.Model(&db.Task{}).Where("id = ?", task.ID).
			Update("updated_at", back).Error; err != nil {
			t.Fatalf("backdate task %d: %v", task.ID, err)
		}
	}
}
func failureRateCfg(pct float64, minSamples int) *config.Config {
	cfg := &config.Config{}
	cfg.Monitoring.TaskFailureRateMaxPct = pct
	cfg.Monitoring.FailureWindowMinutes = 10
	cfg.Monitoring.FailureMinSamples = minSamples
	return cfg
}

// TestFailureRateRuleThresholds covers the decisive boundary: a ratio above
// threshold fires with an actionable detail line.
func TestFailureRateRuleThresholds(t *testing.T) {
	s := newTasksTestServer(t)

	// 25% of 24 terminal tasks = 6 failures.
	for i := 0; i < 18; i++ {
		seedTerminalTask(t, s, "completed", 0)
	}
	for i := 0; i < 6; i++ {
		seedTerminalTask(t, s, "failed", 0)
	}
	s.cfg = failureRateCfg(20, 10) // threshold below actual → must fire

	hit, ok := s.evaluateTaskFailureRate(s.cfg)
	if !ok || hit.Rule != "task_failure_rate" {
		t.Fatalf("expected breach hit, got ok=%v hit=%+v", ok, hit)
	}
	for _, want := range []string{"25.0%", "24", "threshold 20%"} {
		if !strings.Contains(hit.Detail, want) {
			t.Fatalf("detail %q missing %q", hit.Detail, want)
		}
	}

	// Healthy: no failures at all → silent.
	s2 := newTasksTestServer(t)
	for i := 0; i < 12; i++ {
		seedTerminalTask(t, s2, "completed", 0)
	}
	s2.cfg = failureRateCfg(20, 10)
	if _, ok := s2.evaluateTaskFailureRate(s2.cfg); ok {
		t.Fatal("healthy state fired the failure-rate rule")
	}
}

// TestFailureRateMinSamplesGate ensures small noisy samples stay silent even
// at a 100% failure rate.
func TestFailureRateMinSamplesGate(t *testing.T) {
	s := newTasksTestServer(t)
	for i := 0; i < 5; i++ {
		seedTerminalTask(t, s, "failed", 0)
	}
	s.cfg = failureRateCfg(20, 20) // min_samples=20 > 5 seeded
	if _, ok := s.evaluateTaskFailureRate(s.cfg); ok {
		t.Fatal("min-samples gate failed to suppress tiny sample")
	}
}

// TestFailureRateWindowExcludesOldTasks pins the look-back semantics: old
// failures outside the window must not count toward today's rate.
func TestFailureRateWindowExcludesOldTasks(t *testing.T) {
	s := newTasksTestServer(t)
	for i := 0; i < 15; i++ {
		seedTerminalTask(t, s, "failed", 60)
	}
	for i := 0; i < 12; i++ {
		seedTerminalTask(t, s, "completed", 0)
	}
	s.cfg = failureRateCfg(20, 10)
	if _, ok := s.evaluateTaskFailureRate(s.cfg); ok {
		t.Fatal("out-of-window failures leaked into the rate")
	}
}
