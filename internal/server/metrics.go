package server

import (
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsCollector struct {
	AgentsTotal     prometheus.Gauge
	AgentsOnline    prometheus.Gauge
	TasksTotal      prometheus.Counter
	TasksPending    prometheus.Gauge
	ListenersTotal  prometheus.Gauge
	CredsTotal      prometheus.Gauge
	UptimeSeconds   prometheus.GaugeFunc
	RequestDuration *prometheus.HistogramVec
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
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "forgec2_request_duration_seconds",
			Help:    "Histogram of API request durations.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
	}
}

func (mc *MetricsCollector) Register(reg prometheus.Registerer) {
	reg.MustRegister(
		mc.AgentsTotal,
		mc.AgentsOnline,
		mc.TasksTotal,
		mc.TasksPending,
		mc.ListenersTotal,
		mc.CredsTotal,
		mc.UptimeSeconds,
		mc.RequestDuration,
	)
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
	s.db.Model(&db.Implant{}).Count(&total)
	s.metrics.AgentsTotal.Set(float64(total))

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	var online int64
	s.db.Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&online)
	s.metrics.AgentsOnline.Set(float64(online))

	var pending int64
	s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pending)
	s.metrics.TasksPending.Set(float64(pending))

	var listeners int64
	s.db.Model(&db.Listener{}).Where("enabled = ?", true).Count(&listeners)
	s.metrics.ListenersTotal.Set(float64(listeners))

	var creds int64
	s.db.Model(&db.CredentialEntry{}).Count(&creds)
	s.metrics.CredsTotal.Set(float64(creds))
}
