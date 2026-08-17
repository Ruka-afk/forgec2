package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/gin-gonic/gin"
)

const (
	offlineHookSemSize = 8
	offlineHookTimeout = 30 * time.Second
)

type MonitorCollector struct {
	server         *Server
	mu             sync.Mutex
	lastMetrics    db.SystemMetric
	metricsHistory []db.SystemMetric
	hookSem        chan struct{}
}

func NewMonitorCollector(s *Server) *MonitorCollector {
	return &MonitorCollector{
		server:         s,
		metricsHistory: make([]db.SystemMetric, 0, 60),
		hookSem:        make(chan struct{}, offlineHookSemSize),
	}
}

func (m *MonitorCollector) Start() {
	// Collect initial metrics immediately
	metrics := m.collectSystemMetrics()
	m.mu.Lock()
	m.lastMetrics = metrics
	m.metricsHistory = append(m.metricsHistory, metrics)
	m.mu.Unlock()

	// Start periodic collection
	m.server.wg.Add(2)
	go m.collectMetrics()
	go m.checkAlerts()
}

func (m *MonitorCollector) collectMetrics() {
	defer m.server.wg.Done()
	ticker := time.NewTicker(MonitorMetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.server.ctx.Done():
			return
		case <-ticker.C:
			metrics := m.collectSystemMetrics()
			m.mu.Lock()
			m.lastMetrics = metrics
			m.metricsHistory = append(m.metricsHistory, metrics)
			if len(m.metricsHistory) > 60 {
				m.metricsHistory = m.metricsHistory[len(m.metricsHistory)-60:]
			}
			m.mu.Unlock()

			if err := m.server.db.Create(&metrics).Error; err != nil {
				slog.Error("Failed to persist system metrics", "error", err)
			}
		}
	}
}

func (m *MonitorCollector) collectSystemMetrics() db.SystemMetric {
	var metrics db.SystemMetric

	metrics.CPULoad = m.getCPULoad()
	memStats := m.getMemoryStats()
	metrics.MemoryUsed = memStats.used
	metrics.MemoryTotal = memStats.total
	diskStats := m.getDiskStats()
	metrics.DiskUsed = diskStats.used
	metrics.DiskTotal = diskStats.total

	hostname, _ := os.Hostname()
	metrics.Hostname = hostname
	metrics.CreatedAt = time.Now()

	return metrics
}

func (m *MonitorCollector) getMemoryStats() struct{ used, total float64 } {
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	return struct{ used, total float64 }{float64(mstats.Alloc), float64(mstats.Sys)}
}

func (m *MonitorCollector) checkAlerts() {
	defer m.server.wg.Done()
	ticker := time.NewTicker(MonitorAlertInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.server.ctx.Done():
			return
		case <-ticker.C:
			m.checkSystemAlerts()
			m.checkAgentAlerts()
		}
	}
}

func (m *MonitorCollector) checkSystemAlerts() {
	m.mu.Lock()
	metrics := m.lastMetrics
	m.mu.Unlock()

	var rules []db.AlertRule
	if err := m.server.db.Where("enabled = ? AND type IN ?", true, []string{"cpu_high", "memory_high", "disk_high"}).Limit(200).Find(&rules).Error; err != nil {
		slog.Error("Monitor: failed to query system alert rules", "err", err)
	}

	for _, rule := range rules {
		var trigger bool
		var value float64

		switch rule.Type {
		case "cpu_high":
			value = metrics.CPULoad
			trigger = value > rule.Threshold
		case "memory_high":
			if metrics.MemoryTotal > 0 {
				value = (metrics.MemoryUsed / metrics.MemoryTotal) * 100
				trigger = value > rule.Threshold
			}
		case "disk_high":
			if metrics.DiskTotal > 0 {
				value = (metrics.DiskUsed / metrics.DiskTotal) * 100
				trigger = value > rule.Threshold
			}
		}

		if trigger {
			m.triggerAlert(&rule, "system", metrics.Hostname,
				fmt.Sprintf("%.1f%%", value),
				map[string]interface{}{"value": value, "threshold": rule.Threshold})
		}
	}
}

func (m *MonitorCollector) checkAgentAlerts() {
	var rules []db.AlertRule
	if err := m.server.db.Where("enabled = ? AND type = ?", true, "agent_offline").Limit(200).Find(&rules).Error; err != nil {
		slog.Error("Monitor: failed to query agent offline rules", "err", err)
	}

	var agents []db.Implant
	if err := m.server.db.Where("status IN ?", []string{"online", "stale"}).Limit(5000).Find(&agents).Error; err != nil {
		slog.Error("Monitor: failed to query agents for alert check", "err", err)
	}

	now := time.Now()
	var staleIDs []string
	var offlineIDs []string
	for _, agent := range agents {
		offlineFor := now.Sub(agent.LastSeen)
		switch {
		case offlineFor > m.server.staleThreshold():
			offlineIDs = append(offlineIDs, agent.ID)
			m.server.broadcastAgentOffline(agent)
			m.server.recordAgentStatusEvent(agent.ID, "offline")
			if m.server.pluginManager != nil {
				select {
				case m.hookSem <- struct{}{}:
					go func(a db.Implant) {
						defer func() {
							<-m.hookSem
							if r := recover(); r != nil {
								slog.Error("Plugin hook panicked (agent offline)", "agent", a.ID, "recover", r)
							}
						}()
						ctx, cancel := context.WithTimeout(context.Background(), offlineHookTimeout)
						defer cancel()
						m.server.pluginManager.ExecuteHook(ctx, plugin.Event{
							Type:      plugin.EventAgentDisconnect,
							Timestamp: time.Now(),
							AgentID:   a.ID,
							Payload: map[string]interface{}{
								"hostname":            a.Hostname,
								"ip":                  a.IP,
								"offline_for_seconds": now.Sub(a.LastSeen).Seconds(),
							},
						})
					}(agent)
				default:
					slog.Warn("Monitor: offline hook backlog full, skipping agent", "agent", agent.ID)
				}
			}
		case offlineFor > m.server.offlineThreshold() && agent.Status == "online":
			staleIDs = append(staleIDs, agent.ID)
			m.server.recordAgentStatusEvent(agent.ID, "stale")
		}
		for _, rule := range rules {
			threshold := time.Duration(rule.Threshold) * time.Second
			if threshold <= 0 {
				threshold = m.server.offlineThreshold()
			}
			if offlineFor > threshold {
				m.triggerAlert(&rule, agent.ID, agent.Hostname,
					offlineFor.String(),
					map[string]interface{}{"agent_id": agent.ID, "hostname": agent.Hostname})
			}
		}
	}
	if len(staleIDs) > 0 {
		if err := m.server.db.Model(&db.Implant{}).Where("id IN ?", staleIDs).Update("status", "stale").Error; err != nil {
			slog.Error("Monitor: failed to flip agents to stale", "count", len(staleIDs), "err", err)
		}
	}
	if len(offlineIDs) > 0 {
		if err := m.server.db.Model(&db.Implant{}).Where("id IN ?", offlineIDs).Update("status", "offline").Error; err != nil {
			slog.Error("Monitor: failed to flip agents to offline", "count", len(offlineIDs), "err", err)
		}
	}
}

func (m *MonitorCollector) triggerAlert(rule *db.AlertRule, source, sourceName, value string, details map[string]interface{}) {
	var existingAlert db.Alert
	result := m.server.db.Where("rule_id = ? AND source = ? AND status = ?", rule.ID, source, "active").First(&existingAlert)

	if result.Error == nil {
		return
	}

	detailsJSON, ok := marshalJSONSafe(details)
	if !ok {
		slog.Error("Failed to marshal alert details", "rule", rule.ID, "source", source)
		return
	}

	alert := db.Alert{
		RuleID:     rule.ID,
		Type:       rule.Type,
		Severity:   m.getSeverity(rule.Type),
		Title:      rule.Name,
		Message:    fmt.Sprintf("%s: %s", rule.Name, value),
		Source:     source,
		SourceName: sourceName,
		Status:     "active",
		Details:    string(detailsJSON),
	}

	if err := m.server.db.Create(&alert).Error; err != nil {
		slog.Error("Failed to create alert", "err", err)
		return
	}

	m.server.triggerWebhooks(Event{
		Type:      EventType("alert." + rule.Type),
		AgentID:   source,
		AgentHost: sourceName,
		Timestamp: time.Now(),
		Data:      details,
	})

	slog.Warn("Alert triggered", "type", rule.Type, "source", source, "message", alert.Message)
}

func (m *MonitorCollector) getSeverity(ruleType string) string {
	switch ruleType {
	case "agent_offline", "cpu_high", "memory_high", "disk_high":
		return "critical"
	case "credential_found", "agent_online":
		return "info"
	default:
		return "warning"
	}
}

func (m *MonitorCollector) GetLatestMetrics() db.SystemMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastMetrics
}

func (m *MonitorCollector) GetMetricsHistory() []db.SystemMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]db.SystemMetric{}, m.metricsHistory...)
}

func (s *Server) handleGetSystemMetrics(c *gin.Context) {
	if s.monitorCollector == nil {
		respondError(c, http.StatusServiceUnavailable, "monitor collector not initialized")
		return
	}

	metrics := s.monitorCollector.GetLatestMetrics()

	// Safe percent calculations to avoid NaN/Infinity
	memPercent := 0.0
	if metrics.MemoryTotal > 0 {
		memPercent = math.Max(0, math.Min(100, (metrics.MemoryUsed/metrics.MemoryTotal)*100))
	}

	diskPercent := 0.0
	if metrics.DiskTotal > 0 {
		diskPercent = math.Max(0, math.Min(100, (metrics.DiskUsed/metrics.DiskTotal)*100))
	}

	c.JSON(http.StatusOK, gin.H{
		"cpu": metrics.CPULoad,
		"memory": map[string]float64{
			"used":    metrics.MemoryUsed,
			"total":   metrics.MemoryTotal,
			"percent": memPercent,
		},
		"disk": map[string]float64{
			"used":    metrics.DiskUsed,
			"total":   metrics.DiskTotal,
			"percent": diskPercent,
		},
		"network": map[string]float64{
			"in":  metrics.NetIn,
			"out": metrics.NetOut,
		},
		"hostname":  metrics.Hostname,
		"timestamp": metrics.CreatedAt,
	})
}

func (s *Server) handleGetMetricsHistory(c *gin.Context) {
	if s.monitorCollector == nil {
		respondError(c, http.StatusServiceUnavailable, "monitor collector not initialized")
		return
	}

	history := s.monitorCollector.GetMetricsHistory()
	c.JSON(http.StatusOK, gin.H{"data": history})
}

func (s *Server) handleGetAlerts(c *gin.Context) {
	status := c.DefaultQuery("status", "")
	severity := c.DefaultQuery("severity", "")

	query := s.db.Model(&db.Alert{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}

	var alerts []db.Alert
	if err := query.Preload("Rule").Order("created_at DESC").Limit(100).Find(&alerts).Error; err != nil {
		slog.Error("Failed to query alerts", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to query alerts")
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (s *Server) handleGetAlertStats(c *gin.Context) {
	var stats struct {
		Active   int64 `json:"active"`
		Critical int64 `json:"critical"`
		Warning  int64 `json:"warning"`
		Info     int64 `json:"info"`
	}

	err := s.db.Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0) as active,
			COALESCE(SUM(CASE WHEN status = 'active' AND severity = 'critical' THEN 1 ELSE 0 END), 0) as critical,
			COALESCE(SUM(CASE WHEN status = 'active' AND severity = 'warning' THEN 1 ELSE 0 END), 0) as warning,
			COALESCE(SUM(CASE WHEN status = 'active' AND severity = 'info' THEN 1 ELSE 0 END), 0) as info
		FROM alerts`,
	).Scan(&stats).Error
	if err != nil {
		slog.Error("Failed to query alert stats", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to query alert stats")
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (s *Server) handleGetAlertRules(c *gin.Context) {
	p := parsePagination(c, 50, 200)
	var total int64
	if err := s.db.Model(&db.AlertRule{}).Count(&total).Error; err != nil {
		slog.Error("Failed to count alert rules", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to count alert rules")
		return
	}
	var rules []db.AlertRule
	if err := s.db.Order("created_at DESC").Offset(p.Offset).Limit(p.PageSize).Find(&rules).Error; err != nil {
		slog.Error("Failed to query alert rules", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to query alert rules")
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules, "total": total, "page": p.Page, "page_size": p.PageSize})
}

func (s *Server) handleCreateAlertRule(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var rule db.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Alert rule operation"))
		return
	}

	if rule.Name == "" || rule.Type == "" {
		respondError(c, http.StatusBadRequest, "name and type are required")
		return
	}

	if err := s.db.Create(&rule).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Alert rule operation"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func (s *Server) handleUpdateAlertRule(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var rule db.AlertRule
	if !s.findOrFail(c, &rule, id, "rule") {
		return
	}

	var updates struct {
		Name        string  `json:"name"`
		Threshold   float64 `json:"threshold"`
		Enabled     bool    `json:"enabled"`
		Description string  `json:"description"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Alert rule operation"))
		return
	}

	if updates.Name != "" {
		rule.Name = updates.Name
	}
	if updates.Description != "" {
		rule.Description = updates.Description
	}
	rule.Threshold = updates.Threshold
	rule.Enabled = updates.Enabled

	if err := s.db.Save(&rule).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Alert rule operation"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func (s *Server) handleDeleteAlertRule(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")

	if err := s.db.Delete(&db.AlertRule{}, id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Alert rule operation"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAcknowledgeAlert(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var alert db.Alert
	if !s.findOrFail(c, &alert, id, "alert") {
		return
	}

	alert.Status = "acknowledged"
	if err := s.db.Save(&alert).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Alert rule operation"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "alert": alert})
}

func (s *Server) handleResolveAlert(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var alert db.Alert
	if !s.findOrFail(c, &alert, id, "alert") {
		return
	}

	alert.Status = "resolved"
	if err := s.db.Save(&alert).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Alert rule operation"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "alert": alert})
}

func (s *Server) handleGetAgentStatus(c *gin.Context) {
	var stats struct {
		Total   int64 `json:"total"`
		Online  int64 `json:"online"`
		Stale   int64 `json:"stale"`
		Offline int64 `json:"offline"`
	}

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-s.staleThreshold())

	if err := s.db.Raw(`
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN last_seen > ? THEN 1 ELSE 0 END), 0) as online,
			COALESCE(SUM(CASE WHEN last_seen > ? AND last_seen <= ? THEN 1 ELSE 0 END), 0) as stale,
			COALESCE(SUM(CASE WHEN last_seen <= ? THEN 1 ELSE 0 END), 0) as offline
		FROM implants`, offlineCutoff, offlineCutoff, staleCutoff, offlineCutoff,
	).Scan(&stats).Error; err != nil {
		slog.Error("Failed to query agent status stats", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to query agent status stats")
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (s *Server) TriggerAlertForEvent(evt Event) {
	if s.monitorCollector == nil {
		return
	}

	var rules []db.AlertRule
	switch evt.Type {
	case EventCredentialFound:
		if err := s.db.Where("enabled = ? AND type = ?", true, "credential_found").Limit(200).Find(&rules).Error; err != nil {
			slog.Error("Monitor: failed to query credential_found rules", "err", err)
		}
	case EventImplantCheckin:
		if err := s.db.Where("enabled = ? AND type = ?", true, "agent_online").Limit(200).Find(&rules).Error; err != nil {
			slog.Error("Monitor: failed to query agent_online rules", "err", err)
		}
	case EventImplantDisconnect:
		if err := s.db.Where("enabled = ? AND type = ?", true, "agent_offline").Limit(200).Find(&rules).Error; err != nil {
			slog.Error("Monitor: failed to query agent_offline rules", "err", err)
		}
	}

	for _, rule := range rules {
		s.monitorCollector.triggerAlert(&rule, evt.AgentID, evt.AgentHost,
			string(evt.Type), evt.Data)
	}
}

func (s *Server) handleScreenMonitorPage(c *gin.Context) {
	id := c.Param("id")

	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/agents")
		return
	}

	stats := s.getNavStats(c)
	data := gin.H{
		"Title":     "ForgeC2 - Screen Monitoring",
		"Agent":     agent,
		"ActiveNav": "agents",
		"Online":    time.Since(agent.LastSeen) < s.offlineThreshold(),
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

func (s *Server) handleStartScreenMonitor(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	s.screenMonitorMu.Lock()
	if len(s.screenMonitorImplants) >= MaxScreenMonitors {
		s.screenMonitorMu.Unlock()
		respondError(c, http.StatusTooManyRequests, "screen monitor limit reached")
		return
	}
	s.screenMonitorImplants[strings.ToLower(id)] = time.Now()
	s.screenMonitorMu.Unlock()

	interval := c.PostForm("interval")
	if interval == "" {
		interval = c.Query("interval")
	}
	quality := c.PostForm("quality")
	if quality == "" {
		quality = c.Query("quality")
	}
	if interval == "" {
		interval = "5"
	}
	if quality == "" {
		quality = "medium"
	}
	streamCmd := interval + "," + quality
	task, err := s.createTask(id, "screen_stream_start", streamCmd, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Screen monitor: failed to create task", "agent_id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	s.LogAuditRecord(c, "screen_monitor_start", "agent", id, "Started screen monitoring", true, nil)
	slog.Info("Screen monitoring started", "agent_id", id, "task_id", task.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Screen stream started"})
}

func (s *Server) handleStopScreenMonitor(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")

	s.screenMonitorMu.Lock()
	startTime, ok := s.screenMonitorImplants[strings.ToLower(id)]
	delete(s.screenMonitorImplants, strings.ToLower(id))
	s.screenMonitorMu.Unlock()

	if ok {
		s.db.Where("agent_id = ? AND created_at >= ? AND type IN (?)", id, startTime,
			[]string{"screenshot", "screen_stream_start"}).
			Delete(&db.Task{})
	}

	stopTask, err := s.createTask(id, "screen_stream_stop", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Screen monitor stop: failed to create task", "agent_id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create stop task")
		return
	}
	s.broadcastTaskUpdate(id, *stopTask)

	s.LogAuditRecord(c, "screen_monitor_stop", "agent", id, "Stopped screen monitoring", true, nil)
	slog.Info("Screen monitoring stopped", "agent_id", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) IsScreenMonitoring(agentID string) bool {
	s.screenMonitorMu.Lock()
	defer s.screenMonitorMu.Unlock()
	_, ok := s.screenMonitorImplants[strings.ToLower(agentID)]
	return ok
}

func (s *Server) BroadcastScreenshot(agentID string, base64Data string) {
	payload := map[string]string{
		"type":     "screenshot",
		"agent_id": agentID,
		"data":     base64Data,
	}
	message, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal screenshot payload", "err", err)
		return
	}

	s.broadcastToClients(message)
}

// handleAgentRemoteInput accepts remote desktop input events and queues a remote_input task.
// Payload JSON: {"type":"click|move|key","x":0,"y":0,"key":""}
// Agent-side dispatch: Windows (SendInput-equivalent win32 calls), Linux
// (xdotool/ydotool), macOS (osascript keystrokes).
func (s *Server) handleAgentRemoteInput(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	var req struct {
		Type string `json:"type"`
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Key  string `json:"key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid json: expected {type, x, y, key}")
		return
	}
	if req.Type == "" {
		respondError(c, http.StatusBadRequest, "type is required (click, move, key)")
		return
	}

	payload, ok := marshalJSONSafe(req)
	if !ok {
		slog.Error("Remote input: failed to marshal request", "agent_id", id, "type", req.Type)
		respondError(c, http.StatusInternalServerError, "failed to marshal request")
		return
	}
	task, err := s.createTask(id, "remote_input", string(payload), "", "", "", 0, 0)
	if err != nil {
		slog.Error("Remote input: failed to create task", "agent_id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	s.LogAuditRecord(c, "remote_input", "agent", id, fmt.Sprintf("Remote input: %s", req.Type), true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": "remote input queued"})
}

func (s *Server) handleScreenFrame(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to read body")
		return
	}

	env, req, kind := s.decodeBeaconEnvelope(raw)
	if kind != frameEncrypted {
		slog.Warn("Screen frame rejected: not a v2 encrypted frame", "agent_id", env.UUID)
		respondError(c, http.StatusUnauthorized, "authentication failed")
		return
	}

	for _, r := range req.Results {
		if r.Type == "screen_frame" && r.Output != "" {
			if s.IsScreenMonitoring(req.UUID) {
				s.BroadcastScreenshot(req.UUID, r.Output)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleGetRekeyStats reports crypto session rekey activity across live
// agents (counts and timestamps only — no key material).
func (s *Server) handleGetRekeyStats(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	if s.sessionManager == nil {
		respondError(c, http.StatusServiceUnavailable, "session manager not initialized")
		return
	}
	respond(c, s.sessionManager.Stats())
}
