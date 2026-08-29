package server

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// Metric alert bridge: threshold checks over the teamserver's Prometheus
// gauges, bridged into the existing notification + webhook pipelines.
//
// The bridge is opt-in (monitoring.alerts_enabled) because alerting is an
// outbound-sensitive capability. Each rule carries its own cooldown so a
// sustained condition produces one notification per window instead of one
// per evaluation tick.

const (
	metricAlertNotificationType = "metric_alert"
	defaultAlertCooldown        = 600 * time.Second
	defaultAlertEvalInterval    = 30 * time.Second
)

// metricAlertHit is one rule that crossed its threshold.
type metricAlertHit struct {
	Rule   string
	Detail string
}

// metricAlertEngine evaluates rules and enforces per-rule cooldowns. now is
// injectable so tests can drive the clock deterministically.
type metricAlertEngine struct {
	mu        sync.Mutex
	lastFired map[string]time.Time
	now       func() time.Time
	cooldown  time.Duration
}

func newMetricAlertEngine(cooldown time.Duration) *metricAlertEngine {
	if cooldown <= 0 {
		cooldown = defaultAlertCooldown
	}
	return &metricAlertEngine{
		lastFired: make(map[string]time.Time),
		now:       time.Now,
		cooldown:  cooldown,
	}
}

// gaugeValue reads a prometheus gauge value; the testutil helper is the
// sanctioned runtime accessor for single-metric collection.
func gaugeValue(v prometheus.Gauge) float64 {
	return testutil.ToFloat64(v)
}

// evaluate runs every rule against the server's current metrics and returns
// the hits whose cooldown has expired, recording their fire time.
func (e *metricAlertEngine) evaluate(s *Server) []metricAlertHit {
	if s == nil || s.metrics == nil || s.cfg == nil {
		return nil
	}
	mon := &s.cfg.Monitoring

	total := gaugeValue(s.metrics.AgentsTotal)
	online := gaugeValue(s.metrics.AgentsOnline)
	pending := gaugeValue(s.metrics.TasksPending)

	var candidates []metricAlertHit
	if total > 0 && online <= float64(mon.AgentsOnlineMin) {
		candidates = append(candidates, metricAlertHit{
			Rule:   "agents_online_min",
			Detail: fmt.Sprintf("agents online %d <= threshold %d (%d registered)", int(online), mon.AgentsOnlineMin, int(total)),
		})
	}
	if pending > float64(mon.TasksPendingMax) {
		candidates = append(candidates, metricAlertHit{
			Rule:   "tasks_pending_max",
			Detail: fmt.Sprintf("pending tasks %d > threshold %d", int(pending), mon.TasksPendingMax),
		})
	}
	if hit, ok := s.evaluateTaskFailureRate(s.cfg); ok {
		candidates = append(candidates, hit)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	now := e.now()
	var fired []metricAlertHit
	for _, hit := range candidates {
		if last, seen := e.lastFired[hit.Rule]; seen && now.Sub(last) < e.cooldown {
			continue
		}
		e.lastFired[hit.Rule] = now
		fired = append(fired, hit)
	}
	return fired
}

// evaluateAndFire evaluates the rules and delivers each hit through the
// notification table, the event bus and the webhook pipeline.
func (e *metricAlertEngine) evaluateAndFire(s *Server) {
	hits := e.evaluate(s)
	for _, hit := range hits {
		slog.Warn("Metric alert triggered", "rule", hit.Rule, "detail", hit.Detail)
		if s.db != nil {
			// DispatchNotification persists the row and fans out to any
			// configured external routes (Discord/Telegram/webhook).
			s.DispatchNotification(&db.Notification{
				Type:     metricAlertNotificationType,
				Title:    "Metric alert: " + hit.Rule,
				Message:  hit.Detail,
				Severity: "warning",
			})
		}
		s.eventManager.Emit(Event{
			Type:      EventAlertTriggered,
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"rule": hit.Rule, "detail": hit.Detail},
		})
		s.triggerWebhooks(Event{
			Type:      EventAlertTriggered,
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"rule": hit.Rule, "detail": hit.Detail},
		})
	}
}

// evaluateTaskFailureRate computes the failed share of terminal tasks inside
// the configured look-back window. Returns ok=false when below the minimum
// sample count (silence beats noise) or on query errors.
func (s *Server) evaluateTaskFailureRate(cfg *config.Config) (metricAlertHit, bool) {
	if cfg == nil || cfg.Monitoring.TaskFailureRateMaxPct <= 0 || s.db == nil {
		return metricAlertHit{}, false
	}
	window := time.Duration(cfg.Monitoring.FailureWindowMinutes) * time.Minute
	if window <= 0 {
		window = 10 * time.Minute
	}
	minSamples := cfg.Monitoring.FailureMinSamples
	if minSamples <= 0 {
		minSamples = 20
	}

	var total, failed int64
	cutoff := time.Now().Add(-window)
	row := s.db.Model(&db.Task{}).Select(
		"COUNT(*) as total, SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed",
	).Where("status IN ? AND updated_at > ?", []string{"completed", "failed"}, cutoff).Row()
	if err := row.Scan(&total, &failed); err != nil {
		slog.Debug("failure-rate query skipped", "error", err)
		return metricAlertHit{}, false
	}
	if total < int64(minSamples) {
		return metricAlertHit{}, false
	}
	pct := float64(failed) / float64(total) * 100
	if pct <= cfg.Monitoring.TaskFailureRateMaxPct {
		return metricAlertHit{}, false
	}
	return metricAlertHit{
		Rule:   "task_failure_rate",
		Detail: fmt.Sprintf("task failure rate %.1f%% (%d/%d over %dm) > threshold %.0f%%",
			pct, failed, total, cfg.Monitoring.FailureWindowMinutes, cfg.Monitoring.TaskFailureRateMaxPct),
	}, true
}

// startMetricAlertLoop launches the periodic evaluator when the bridge is
// enabled and a metrics collector exists. Called from the server lifecycle.
func (s *Server) startMetricAlertLoop() {	if s.cfg == nil || !s.cfg.Monitoring.AlertsEnabled || s.metrics == nil {
		return
	}
	if s.ctx == nil {
		// No lifecycle context (unit-test stub): the loop would have nothing
		// to stop on. Skip rather than panic on ctx.Done().
		return
	}
	interval := defaultAlertEvalInterval
	if secs := s.cfg.Monitoring.EvalSeconds; secs > 0 {
		interval = time.Duration(secs) * time.Second
	}
	engine := newMetricAlertEngine(time.Duration(s.cfg.Monitoring.CooldownSeconds) * time.Second)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				engine.evaluateAndFire(s)
			}
		}
	}()
	slog.Info("Metric alert bridge started", "interval", interval,
		"agents_online_min", s.cfg.Monitoring.AgentsOnlineMin,
		"tasks_pending_max", s.cfg.Monitoring.TasksPendingMax)
}
