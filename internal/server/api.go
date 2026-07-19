package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// ── REST API: Agents ──

func (s *Server) apiListAgents(c *gin.Context) {
	search := c.Query("search")
	statusFilter := c.Query("status")
	osFilter := c.Query("os")
	tagID := c.Query("tag_id")

	pageNum := 1
	if v := c.Query("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageNum = n
		}
	}
	pageSize := DefaultPageSize
	if v := c.Query("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-StaleThreshold)

	query := s.db.Model(&db.Implant{})

	if search != "" {
		query = query.Where("(hostname LIKE ? ESCAPE '\\' OR username LIKE ? ESCAPE '\\' OR ip LIKE ? ESCAPE '\\')",
			"%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%")
	}
	if statusFilter == "online" {
		query = query.Where("last_seen > ?", offlineCutoff)
	} else if statusFilter == "stale" {
		query = query.Where("last_seen <= ? AND last_seen > ?", offlineCutoff, staleCutoff)
	} else if statusFilter == "offline" {
		query = query.Where("last_seen <= ?", staleCutoff)
	}
	if osFilter != "" {
		query = query.Where("LOWER(os) LIKE ? ESCAPE '\\'", "%"+escapeLike(osFilter)+"%")
	}
	if tagID != "" {
		query = query.Joins("JOIN agent_tag_assignments ON agent_tag_assignments.implant_id = implants.id").
			Where("agent_tag_assignments.agent_tag_id = ?", tagID)
	}

	var total int64
	query.Count(&total)

	var agents []db.Implant
	query.Order("last_seen desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&agents)

	for i := range agents {
		agents[i].Status = s.agentStatus(agents[i]).Status
	}

	c.JSON(http.StatusOK, gin.H{
		"agents":    agents,
		"total":     total,
		"page":      pageNum,
		"page_size": pageSize,
	})
}

func (s *Server) apiGetAgent(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "agent not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": agent})
}

func (s *Server) apiDeleteAgent(c *gin.Context) {
	role, _ := c.Get("user_role")
	if role != "admin" {
		respondError(c, http.StatusForbidden, "admin required")
		return
	}

	id := c.Param("id")
	if err := s.db.Delete(&db.Implant{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "delete failed")
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

	if err := query.Limit(APITaskListLimit).Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks, "total": len(tasks)})
}

func (s *Server) apiGetTask(c *gin.Context) {
	id := c.Param("id")
	var task db.Task
	if err := s.db.Preload("Agent").First(&task, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "task not found")
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
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	task, err := s.createTask(req.AgentID, req.Type, req.Command, req.Shell, "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "task creation failed")
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
		query = query.Where("domain LIKE ? ESCAPE '\\'", "%"+escapeLike(domain)+"%")
	}

	if err := query.Limit(APICredentialListLimit).Find(&creds).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": creds, "total": len(creds)})
}

// ── REST API: Listeners ──

func (s *Server) apiListListeners(c *gin.Context) {
	s.handleListListeners(c)
}

// ── REST API: Dashboard Stats ──

func (s *Server) apiDashboardStats(c *gin.Context) {
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var totalAgents, onlineAgents, todayTasks, pendingTasks, failedTasks,
		totalCreds, totalTokens, totalListeners, totalTasks, totalAudits int64

	g, ctx := errgroup.WithContext(c.Request.Context())
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Implant{}).Count(&totalAgents).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&onlineAgents).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Task{}).Where("created_at >= ?", todayStart).Count(&todayTasks).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Task{}).Where("status = ?", "failed").Count(&failedTasks).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Task{}).Count(&totalTasks).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.CredentialEntry{}).Count(&totalCreds).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.TokenEntry{}).Count(&totalTokens).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Listener{}).Count(&totalListeners).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.AuditLog{}).Count(&totalAudits).Error })
	if err := g.Wait(); err != nil {
		slog.Error("api: failed to count dashboard stats", "error", err)
	}

	var staleCount int64
	staleCutoff := now.Add(-30 * time.Minute)
	if err := s.db.Model(&db.Implant{}).Where("last_seen > ? AND last_seen <= ?", offlineCutoff, staleCutoff).Count(&staleCount).Error; err != nil {
		slog.Error("api: failed to count stale agents", "error", err)
	}
	var offlineCount int64
	if err := s.db.Model(&db.Implant{}).Where("last_seen <= ?", offlineCutoff).Count(&offlineCount).Error; err != nil {
		slog.Error("api: failed to count offline agents", "error", err)
	}

	onlineUsers := int64(len(s.getOnlineUsers()))

	var recentTasks []db.Task
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(DashboardRecentTasks).Find(&recentTasks)

	stats := gin.H{
		"total_agents":     totalAgents,
		"online_agents":    onlineAgents,
		"today_tasks":      todayTasks,
		"pending_tasks":    pendingTasks,
		"failed_tasks":     failedTasks,
		"total_tasks":      totalTasks,
		"total_creds":      totalCreds,
		"total_tokens":     totalTokens,
		"total_listeners":  totalListeners,
		"total_audits":     totalAudits,
		"online_count":     onlineAgents,
		"stale_count":      staleCount,
		"offline_count":    offlineCount,
		"listener_count":   totalListeners,
		"pending_count":    pendingTasks,
		"online_users":     onlineUsers,
		"completed_tasks":  totalTasks - failedTasks - pendingTasks,
		"server_version":   ServerVersion,
		"recent_tasks":     recentTasks,
	}

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
		query = query.Where("user LIKE ? ESCAPE '\\'", "%"+escapeLike(user)+"%")
	}

	if err := query.Limit(APIAuditLogListLimit).Find(&logs).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs, "total": len(logs)})
}

// ── REST API: Bulk Results ──

func (s *Server) apiBulkResults(c *gin.Context) {
	s.bulkHistoryMu.Lock()
	results := make([]BulkResult, len(s.bulkHistory))
	copy(results, s.bulkHistory)
	s.bulkHistoryMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"results": results, "total": len(results)})
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
	api.GET("/bulk/results", s.apiBulkResults)
}


