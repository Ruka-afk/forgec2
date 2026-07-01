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

type MonitorCollector struct {
	server         *Server
	mu             sync.Mutex
	lastMetrics    db.SystemMetric
	metricsHistory []db.SystemMetric
}

func NewMonitorCollector(s *Server) *MonitorCollector {
	return &MonitorCollector{
		server:         s,
		metricsHistory: make([]db.SystemMetric, 0, 60),
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
	go m.collectMetrics()
	go m.checkAlerts()
}

func (m *MonitorCollector) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		metrics := m.collectSystemMetrics()
		m.mu.Lock()
		m.lastMetrics = metrics
		m.metricsHistory = append(m.metricsHistory, metrics)
		if len(m.metricsHistory) > 60 {
			m.metricsHistory = m.metricsHistory[len(m.metricsHistory)-60:]
		}
		m.mu.Unlock()

		go m.server.db.Create(&metrics)
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

func (m *MonitorCollector) getCPULoad() float64 {
	return 0
}

func (m *MonitorCollector) getMemoryStats() struct{ used, total float64 } {
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	return struct{ used, total float64 }{float64(mstats.Alloc), float64(mstats.Sys)}
}

func (m *MonitorCollector) getDiskStats() struct{ used, total float64 } {
	// Return safe defaults to avoid division by zero
	// Real implementation would use platform-specific calls
	return struct{ used, total float64 }{0, 1} // Default: 0% used of 1 unit
}

func (m *MonitorCollector) checkAlerts() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.checkSystemAlerts()
		m.checkAgentAlerts()
	}
}

func (m *MonitorCollector) checkSystemAlerts() {
	m.mu.Lock()
	metrics := m.lastMetrics
	m.mu.Unlock()

	var rules []db.AlertRule
	m.server.db.Where("enabled = ? AND type IN ?", true, []string{"cpu_high", "memory_high", "disk_high"}).Find(&rules)

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
	m.server.db.Where("enabled = ? AND type = ?", true, "agent_offline").Find(&rules)

	if len(rules) == 0 {
		return
	}

	var agents []db.Implant
	m.server.db.Find(&agents)

	thresholdSeconds := int64(300)
	for _, rule := range rules {
		if rule.Threshold > 0 {
			thresholdSeconds = int64(rule.Threshold)
		}
	}

	threshold := time.Duration(thresholdSeconds) * time.Second

	for _, agent := range agents {
		if time.Since(agent.LastSeen) > threshold && agent.Status != "offline" {
			m.server.db.Model(&agent).Update("status", "offline")
			go m.server.broadcastAgentOffline(agent)
			if m.server.pluginManager != nil {
				go m.server.pluginManager.ExecuteHook(context.Background(), plugin.Event{
					Type:      plugin.EventAgentDisconnect,
					Timestamp: time.Now(),
					AgentID:   agent.ID,
					Payload: map[string]interface{}{
						"hostname":            agent.Hostname,
						"ip":                  agent.IP,
						"offline_for_seconds": time.Since(agent.LastSeen).Seconds(),
					},
				})
			}
			for _, rule := range rules {
				m.triggerAlert(&rule, agent.ID, agent.Hostname,
					time.Since(agent.LastSeen).String(),
					map[string]interface{}{"agent_id": agent.ID, "hostname": agent.Hostname})
			}
		}
	}
}

func (m *MonitorCollector) triggerAlert(rule *db.AlertRule, source, sourceName, value string, details map[string]interface{}) {
	var existingAlert db.Alert
	result := m.server.db.Where("rule_id = ? AND source = ? AND status = ?", rule.ID, source, "active").First(&existingAlert)

	if result.Error == nil {
		return
	}

	detailsJSON, _ := json.Marshal(details)

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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "monitor collector not initialized"})
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "monitor collector not initialized"})
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
	query.Preload("Rule").Order("created_at DESC").Limit(100).Find(&alerts)

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (s *Server) handleGetAlertStats(c *gin.Context) {
	var stats struct {
		Active   int64 `json:"active"`
		Critical int64 `json:"critical"`
		Warning  int64 `json:"warning"`
		Info     int64 `json:"info"`
	}

	s.db.Model(&db.Alert{}).Where("status = ?", "active").Count(&stats.Active)
	s.db.Model(&db.Alert{}).Where("status = ? AND severity = ?", "active", "critical").Count(&stats.Critical)
	s.db.Model(&db.Alert{}).Where("status = ? AND severity = ?", "active", "warning").Count(&stats.Warning)
	s.db.Model(&db.Alert{}).Where("status = ? AND severity = ?", "active", "info").Count(&stats.Info)

	c.JSON(http.StatusOK, stats)
}

func (s *Server) handleGetAlertRules(c *gin.Context) {
	var rules []db.AlertRule
	s.db.Order("created_at DESC").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func (s *Server) handleCreateAlertRule(c *gin.Context) {
	var rule db.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if rule.Name == "" || rule.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and type are required"})
		return
	}

	if err := s.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func (s *Server) handleUpdateAlertRule(c *gin.Context) {
	id := c.Param("id")
	var rule db.AlertRule

	if err := s.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	var updates struct {
		Name        string  `json:"name"`
		Threshold   float64 `json:"threshold"`
		Enabled     bool    `json:"enabled"`
		Description string  `json:"description"`
	}

	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "rule": rule})
}

func (s *Server) handleDeleteAlertRule(c *gin.Context) {
	id := c.Param("id")

	if err := s.db.Delete(&db.AlertRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	var alert db.Alert

	if err := s.db.First(&alert, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}

	alert.Status = "acknowledged"
	if err := s.db.Save(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "alert": alert})
}

func (s *Server) handleResolveAlert(c *gin.Context) {
	id := c.Param("id")
	var alert db.Alert

	if err := s.db.First(&alert, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}

	alert.Status = "resolved"
	if err := s.db.Save(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	s.db.Model(&db.Implant{}).Count(&stats.Total)

	threshold := s.offlineThreshold()
	s.db.Model(&db.Implant{}).Where("last_seen > ?", time.Now().Add(-threshold)).Count(&stats.Online)
	s.db.Model(&db.Implant{}).Where("last_seen <= ? AND last_seen > ?", time.Now().Add(-threshold), time.Now().Add(-30*time.Minute)).Count(&stats.Stale)
	s.db.Model(&db.Implant{}).Where("last_seen <= ?", time.Now().Add(-30*time.Minute)).Count(&stats.Offline)

	c.JSON(http.StatusOK, stats)
}

func (s *Server) TriggerAlertForEvent(evt Event) {
	if s.monitorCollector == nil {
		return
	}

	var rules []db.AlertRule
	switch evt.Type {
	case EventCredentialFound:
		s.db.Where("enabled = ? AND type = ?", true, "credential_found").Find(&rules)
	case EventImplantCheckin:
		s.db.Where("enabled = ? AND type = ?", true, "agent_online").Find(&rules)
	case EventImplantDisconnect:
		s.db.Where("enabled = ? AND type = ?", true, "agent_offline").Find(&rules)
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

	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Screen Monitoring",
		"Agent":     agent,
		"ActiveNav": "agents",
		"Online":    time.Since(agent.LastSeen) < s.offlineThreshold(),
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "screen_content", data)
}

func (s *Server) handleStartScreenMonitor(c *gin.Context) {
	id := c.Param("id")

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	s.screenMonitorMu.Lock()
	s.screenMonitorImplants[strings.ToLower(id)] = time.Now()
	s.screenMonitorMu.Unlock()

	task, err := s.createTask(id, "screen_stream_start", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Screen monitor: failed to create task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.LogAuditRecord(c, "screen_monitor_start", "agent", id, "Started screen monitoring", true, nil)
	slog.Info("Screen monitoring started", "agent", id, "task_id", task.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Screen stream started"})
}

func (s *Server) handleStopScreenMonitor(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create stop task"})
		return
	}
	s.broadcastTaskUpdate(id, *stopTask)

	s.LogAuditRecord(c, "screen_monitor_stop", "agent", id, "Stopped screen monitoring", true, nil)
	slog.Info("Screen monitoring stopped", "agent", id)
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

func (s *Server) handleScreenFrame(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid"`
		Data string `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if req.UUID == "" || req.Data == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	if s.IsScreenMonitoring(req.UUID) {
		s.BroadcastScreenshot(req.UUID, req.Data)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
