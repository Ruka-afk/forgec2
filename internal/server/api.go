package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── REST API: Agents ──

func (s *Server) apiListAgents(c *gin.Context) {
	var agents []db.Implant
	query := s.db.Order("last_seen desc")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if os := c.Query("os"); os != "" {
		query = query.Where("os LIKE ?", "%"+os+"%")
	}
	if limit := c.Query("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			query = query.Limit(n)
		}
	}

	if err := query.Limit(500).Find(&agents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agents, "total": len(agents)})
}

func (s *Server) apiGetAgent(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent})
}

func (s *Server) apiDeleteAgent(c *gin.Context) {
	role, _ := c.Get("user_role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "admin required"})
		return
	}

	id := c.Param("id")
	if err := s.db.Delete(&db.Implant{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "delete failed"})
		return
	}
	s.LogAuditRecord(c, "delete_agent", "agent", id, "Agent deleted", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "agent deleted"})
}

// ── REST API: Tasks ──

func (s *Server) apiListTasks(c *gin.Context) {
	var tasks []db.Task
	query := s.db.Order("created_at desc")

	if agentID := c.Query("agent_id"); agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if taskType := c.Query("type"); taskType != "" {
		query = query.Where("type = ?", taskType)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if limit := c.Query("limit"); limit != "" {
		if n, err := strconv.Atoi(limit); err == nil {
			query = query.Limit(n)
		}
	}

	if err := query.Limit(200).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks, "total": len(tasks)})
}

func (s *Server) apiGetTask(c *gin.Context) {
	id := c.Param("id")
	var task db.Task
	if err := s.db.Preload("Agent").First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

func (s *Server) apiCreateTask(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id" binding:"required"`
		Type    string `json:"type" binding:"required"`
		Command string `json:"command"`
		Shell   string `json:"shell"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}

	task, err := s.createTask(req.AgentID, req.Type, req.Command, req.Shell, "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "task creation failed"})
		return
	}

	s.broadcastTaskUpdate(req.AgentID, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// ── REST API: Credentials ──

func (s *Server) apiListCredentials(c *gin.Context) {
	var creds []db.CredentialEntry
	query := s.db.Order("created_at desc")

	if agentID := c.Query("agent_id"); agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if credType := c.Query("type"); credType != "" {
		query = query.Where("type = ?", credType)
	}
	if domain := c.Query("domain"); domain != "" {
		query = query.Where("domain LIKE ?", "%"+domain+"%")
	}

	if err := query.Limit(500).Find(&creds).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": creds, "total": len(creds)})
}

// ── REST API: Listeners ──

func (s *Server) apiListListeners(c *gin.Context) {
	var listeners []db.Listener
	if err := s.db.Order("created_at desc").Limit(100).Find(&listeners).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": listeners, "total": len(listeners)})
}

// ── REST API: Dashboard Stats ──

func (s *Server) apiDashboardStats(c *gin.Context) {
	stats := s.getNavStats()

	var totalTasks, completedTasks, failedTasks, pendingTasks int64
	if err := s.db.Model(&db.Task{}).Count(&totalTasks).Error; err != nil {
		slog.Error("api: failed to count tasks", "error", err)
	}
	if err := s.db.Model(&db.Task{}).Where("status = ?", "completed").Count(&completedTasks).Error; err != nil {
		slog.Error("api: failed to count completed tasks", "error", err)
	}
	if err := s.db.Model(&db.Task{}).Where("status = ?", "failed").Count(&failedTasks).Error; err != nil {
		slog.Error("api: failed to count failed tasks", "error", err)
	}
	if err := s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks).Error; err != nil {
		slog.Error("api: failed to count pending tasks", "error", err)
	}

	stats["TotalTasks"] = totalTasks
	stats["CompletedTasks"] = completedTasks
	stats["FailedTasks"] = failedTasks
	stats["PendingTasks"] = pendingTasks

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// ── REST API: Audit Logs ──

func (s *Server) apiListAuditLogs(c *gin.Context) {
	var logs []db.AuditLog
	query := s.db.Order("created_at desc")

	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if user := c.Query("user"); user != "" {
		query = query.Where("user LIKE ?", "%"+user+"%")
	}

	if err := query.Limit(200).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs, "total": len(logs)})
}

// ── REST API: Health ──

func (s *Server) apiHealth(c *gin.Context) {
	health := gin.H{
		"status":    "ok",
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}

	// Check database
	if sqlDB, err := s.db.DB(); err == nil {
		if err := sqlDB.Ping(); err != nil {
			health["status"] = "degraded"
			health["database"] = "unreachable"
		} else {
			health["database"] = "connected"
		}
	}

	c.JSON(http.StatusOK, health)
}

// registerAPIRoutes registers all REST API routes under the given group
func (s *Server) registerAPIRoutes(api *gin.RouterGroup) {
	api.GET("/health", s.apiHealth)
	api.GET("/dashboard", s.apiDashboardStats)

	api.GET("/agents", s.apiListAgents)
	api.GET("/agents/:id", s.apiGetAgent)
	api.DELETE("/agents/:id", s.apiDeleteAgent)

	api.GET("/tasks", s.apiListTasks)
	api.GET("/tasks/:id", s.apiGetTask)
	api.POST("/tasks", s.apiCreateTask)

	api.GET("/credentials", s.apiListCredentials)
	api.GET("/listeners", s.apiListListeners)
	api.GET("/audit", s.apiListAuditLogs)
}


