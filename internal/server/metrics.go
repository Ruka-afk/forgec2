package server

import (
	"log/slog"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsCollector struct {
	AgentsTotal         prometheus.Gauge
	AgentsOnline        prometheus.Gauge
	TasksTotal          prometheus.Counter
	TasksPending        prometheus.Gauge
	ListenersTotal      prometheus.Gauge
	CredsTotal          prometheus.Gauge
	UptimeSeconds       prometheus.GaugeFunc
	SessionRekeysTotal  *prometheus.CounterVec // label agent_id; lives server-side, fed from crypto.SessionManager stats
	RequestDuration     *prometheus.HistogramVec
	BeaconDuration      *prometheus.HistogramVec
	TaskExecuteDuration *prometheus.HistogramVec
	AuditDroppedTotal  prometheus.Counter
	EventDroppedTotal  *prometheus.CounterVec
	VaultErrorsTotal   *prometheus.CounterVec
	TasksPendingByAgent *prometheus.GaugeVec
	OldestPendingSeconds *prometheus.GaugeVec
	SocksDroppedTotal *prometheus.CounterVec
	FileChainEventsTotal *prometheus.CounterVec
	AgentResultGapsTotal prometheus.Counter
}

func NewMetricsCollector(s *Server) *MetricsCollector {
	return &MetricsCollector{
		AgentsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "forgec2_agents_total",
			Help: "Total number of agents ever registered.",
		}),
		AgentsOnline: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "forgec2_agents_online",
			Help: "Current number of online agents.",
		}),
		TasksTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "forgec2_tasks_total",
			Help: "Total number of tasks created.",
		}),
		TasksPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "forgec2_tasks_pending",
			Help: "Current number of pending tasks.",
		}),
		ListenersTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "forgec2_listeners_total",
			Help: "Current number of enabled listeners.",
		}),
		CredsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "forgec2_credentials_total",
			Help: "Total number of stored credentials.",
		}),
		UptimeSeconds: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "forgec2_uptime_seconds",
			Help: "Server uptime in seconds.",
		}, func() float64 {
			return time.Since(s.startTime).Seconds()
		}),
		SessionRekeysTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "forgec2_session_rekeys_total",
			Help: "Total number of crypto session rekeys, per agent.",
		}, []string{"agent_id"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "forgec2_request_duration_seconds",
			Help:    "Histogram of API request durations.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
		BeaconDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "forgec2_beacon_duration_seconds",
			Help:    "Histogram of beacon processing durations.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"transport"}),
		TaskExecuteDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "forgec2_task_execute_duration_seconds",
			Help:    "Histogram of task execution durations (time between creation and completion).",
			Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300, 600},
		}, []string{"type"}),
		AuditDroppedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "forgec2_audit_dropped_total",
			Help: "Total number of audit log entries dropped (queue full or persist failure).",
		}),
		EventDroppedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "forgec2_event_dropped_total",
			Help: "Total number of internal bus events dropped, by reason.",
		}, []string{"reason"}),
		VaultErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "forgec2_vault_errors_total",
			Help: "Total number of vault crypto failures, by operation. An encrypt failure means output was dropped rather than stored as plaintext.",
		}, []string{"op"}),
		TasksPendingByAgent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forgec2_tasks_pending_by_agent",
			Help: "Pending (undelivered) tasks per agent.",
		}, []string{"agent_id"}),
		OldestPendingSeconds: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "forgec2_oldest_pending_seconds",
			Help: "Age of the oldest pending task per agent, in seconds.",
		}, []string{"agent_id"}),
		SocksDroppedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "forgec2_socks_dropped_total",
			Help: "Total number of SOCKS relay frames dropped, by reason.",
		}, []string{"reason"}),
		FileChainEventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "forgec2_filechain_events_total",
			Help: "File-transfer chain integrity events, by outcome (legacy_bypass, downgrade_rejected, mismatch).",
		}, []string{"outcome"}),
		AgentResultGapsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "forgec2_agent_result_gaps_total",
			Help: "Total task results the agent discarded before delivery (queue-full/oversized); per-task detail is in the agent_result_gap audit record.",
		}),
	}
}

func (mc *MetricsCollector) Register(reg prometheus.Registerer) {
	collectors := []prometheus.Collector{
		mc.AgentsTotal,
		mc.AgentsOnline,
		mc.TasksTotal,
		mc.TasksPending,
		mc.ListenersTotal,
		mc.CredsTotal,
		mc.UptimeSeconds,
		mc.SessionRekeysTotal,
		mc.RequestDuration,
		mc.BeaconDuration,
		mc.TaskExecuteDuration,
		mc.AuditDroppedTotal,
		mc.EventDroppedTotal,
		mc.VaultErrorsTotal,
		mc.TasksPendingByAgent,
		mc.OldestPendingSeconds,
		mc.SocksDroppedTotal,
		mc.FileChainEventsTotal,
		mc.AgentResultGapsTotal,
	}
	for _, c := range collectors {
		err := reg.Register(c)
		if err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				slog.Error("Failed to register metrics collector", "error", err)
			}
		}
	}
}

func metricsMiddleware(mc *MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := c.Writer.Status()
		mc.RequestDuration.WithLabelValues(c.Request.Method, c.Request.URL.Path, itoa(status)).Observe(duration)
	}
}

func metricsPromHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Server) updateMetricsFromDB() {
	var total int64
	if err := s.db.Model(&db.Implant{}).Count(&total).Error; err != nil {
		slog.Error("Failed to count total agents for metrics", "err", err)
	}
	s.metrics.AgentsTotal.Set(float64(total))

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	var online int64
	if err := s.db.Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&online).Error; err != nil {
		slog.Error("Failed to count online agents for metrics", "err", err)
	}
	s.metrics.AgentsOnline.Set(float64(online))

	var pending int64
	if err := s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pending).Error; err != nil {
		slog.Error("Failed to count pending tasks for metrics", "err", err)
	}
	s.metrics.TasksPending.Set(float64(pending))

	var listeners int64
	if err := s.db.Model(&db.Listener{}).Where("enabled = ?", true).Count(&listeners).Error; err != nil {
		slog.Error("Failed to count listeners for metrics", "err", err)
	}
	s.metrics.ListenersTotal.Set(float64(listeners))

	var creds int64
	if err := s.db.Model(&db.CredentialEntry{}).Count(&creds).Error; err != nil {
		slog.Error("Failed to count credentials for metrics", "err", err)
	}
	s.metrics.CredsTotal.Set(float64(creds))

	// Sync the per-agent rekey counters from the live session manager so the
	// exported series reflects reality (counters are process-local and reset
	// on server restart, so Reset+Add is safe and idempotent).
	if s.sessionManager != nil && s.metrics.SessionRekeysTotal != nil {
		st := s.sessionManager.Stats()
		s.metrics.SessionRekeysTotal.Reset()
		for _, entry := range st.RekeyCounts {
			c, err := s.metrics.SessionRekeysTotal.GetMetricWithLabelValues(entry.AgentID)
			if err == nil {
				c.Add(float64(entry.RekeyCount))
			}
		}
	}
}
