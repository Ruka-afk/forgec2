package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// ── REST API: Agents ──

func (s *Server) apiListAgents(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
	staleCutoff := time.Now().Add(-s.staleThreshold())

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
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to count agents")
		return
	}

	var agents []db.Implant
	if err := query.Order("last_seen desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&agents).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to query agents")
		return
	}

	// Compute task stats for the returned agents
	agentIDs := make([]string, len(agents))
	for i, a := range agents {
		agentIDs[i] = a.ID
		agents[i].Status = s.agentStatus(a).Status
	}
	taskStatsMap := computeTaskStats(s.db, agentIDs)

	type agentResponse struct {
		db.Implant
		TaskStats *db.TaskStats `json:"taskStats,omitempty"`
	}
	resp := make([]agentResponse, len(agents))
	for i, a := range agents {
		ts := taskStatsMap[a.ID]
		resp[i] = agentResponse{Implant: a, TaskStats: ts}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      resp,
		"total":     total,
		"page":      pageNum,
		"page_size": pageSize,
	})
}

// computeTaskStats returns task status counts per agent ID using a single GROUP BY query.
func computeTaskStats(database *gorm.DB, ids []string) map[string]*db.TaskStats {
	if len(ids) == 0 {
		return nil
	}
	type row struct {
		AgentID string
		Status  string
		Count   int
	}
	var rows []row
	database.Raw(
		"SELECT agent_id, status, COUNT(*) as count FROM tasks WHERE agent_id IN ? GROUP BY agent_id, status",
		ids,
	).Scan(&rows)

	out := make(map[string]*db.TaskStats, len(ids))
	for _, r := range rows {
		ts, ok := out[r.AgentID]
		if !ok {
			ts = &db.TaskStats{}
			out[r.AgentID] = ts
		}
		switch r.Status {
		case "pending":
			ts.Pending = r.Count
		case "running":
			ts.Running = r.Count
		case "completed":
			ts.Completed = r.Count
		case "failed":
			ts.Failed = r.Count
		}
	}
	return out
}

func (s *Server) apiGetAgent(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
	if !s.requireOperator(c) {
		return
	}
	var tasks []db.Task
	query := s.db.Order("created_at desc").Preload("Agent")

	if agentID := c.Query("agent_id"); agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if taskType := c.Query("type"); taskType != "" {
		query = query.Where("type = ?", taskType)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	limit := APITaskListLimit
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n < limit {
			limit = n
		}
	}

	if err := query.Limit(limit).Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tasks, "total": len(tasks)})
}

func (s *Server) apiGetTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var task db.Task
	if err := s.db.Preload("Agent").First(&task, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "task not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

func (s *Server) apiCreateTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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

	if !IsKnownTaskType(req.Type) && !protocol.ValidTaskType(req.Type) {
		respondError(c, http.StatusBadRequest, "unknown task type: "+req.Type)
		return
	}

	task, err := s.createTask(req.AgentID, req.Type, req.Command, req.Shell, "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "task creation failed")
		return
	}

	s.broadcastTaskUpdate(req.AgentID, *task)
	slog.Info("Task created via API", "task_id", task.ID, "agent_id", task.AgentID, "type", task.Type)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": task})
}

// ── REST API: Credentials ──

func (s *Server) apiListCredentials(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
	if !s.requireOperator(c) {
		return
	}
	s.handleListListeners(c)
}

// ── REST API: Dashboard Stats ──

func (s *Server) apiDashboardStats(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	staleCutoff := now.Add(-s.staleThreshold())

	type agentCounts struct {
		Total   int64
		Online  int64
		Stale   int64
		Offline int64
	}
	type taskCounts struct {
		Total   int64
		Today   int64
		Pending int64
		Failed  int64
	}

	g, ctx := errgroup.WithContext(c.Request.Context())

	var ac agentCounts
	g.Go(func() error {
		return s.db.WithContext(ctx).Raw(`
			SELECT
				COUNT(*) as total,
				COALESCE(SUM(CASE WHEN last_seen > ? THEN 1 ELSE 0 END), 0) as online,
				COALESCE(SUM(CASE WHEN last_seen > ? AND last_seen <= ? THEN 1 ELSE 0 END), 0) as stale,
				COALESCE(SUM(CASE WHEN last_seen <= ? THEN 1 ELSE 0 END), 0) as offline
			FROM implants`, offlineCutoff, offlineCutoff, staleCutoff, offlineCutoff,
		).Scan(&ac).Error
	})

	var tc taskCounts
	g.Go(func() error {
		return s.db.WithContext(ctx).Raw(`
			SELECT
				COUNT(*) as total,
				COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0) as today,
				COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending,
				COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) as failed
			FROM tasks`, todayStart,
		).Scan(&tc).Error
	})

	var totalCreds, totalTokens, totalListeners, totalAudits int64
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.CredentialEntry{}).Count(&totalCreds).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.TokenEntry{}).Count(&totalTokens).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.Listener{}).Count(&totalListeners).Error })
	g.Go(func() error { return s.db.WithContext(ctx).Model(&db.AuditLog{}).Count(&totalAudits).Error })
	var onlineUsersList []UserSession
	g.Go(func() error {
		onlineUsersList = s.getOnlineUsers()
		return nil
	})
	if err := g.Wait(); err != nil {
		slog.Error("API: failed to count dashboard stats", "error", err)
	}

	onlineUsers := int64(len(onlineUsersList))

	var recentTasks []db.Task
	if err := s.db.Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(DashboardRecentTasks).Find(&recentTasks).Error; err != nil {
		slog.Error("API: failed to query recent tasks", "err", err)
	}
	if len(recentTasks) > 0 {
		agentIDs := make([]string, 0, len(recentTasks))
		for _, t := range recentTasks {
			if t.AgentID != "" {
				agentIDs = append(agentIDs, t.AgentID)
			}
		}
		if len(agentIDs) > 0 {
			var agents []db.Implant
			if err := s.db.Where("id IN ?", agentIDs).Find(&agents).Error; err != nil {
				slog.Error("API: failed to query agents for recent tasks", "err", err)
			}
			agentMap := make(map[string]db.Implant, len(agents))
			for _, a := range agents {
				agentMap[a.ID] = a
			}
			for i := range recentTasks {
				if a, ok := agentMap[recentTasks[i].AgentID]; ok {
					recentTasks[i].Agent = a
				}
			}
		}
	}

	stats := gin.H{
		"total_agents":    ac.Total,
		"online_agents":   ac.Online,
		"today_tasks":     tc.Today,
		"pending_tasks":   tc.Pending,
		"failed_tasks":    tc.Failed,
		"total_tasks":     tc.Total,
		"total_creds":     totalCreds,
		"total_tokens":    totalTokens,
		"total_listeners": totalListeners,
		"total_audits":    totalAudits,
		"online_count":    ac.Online,
		"stale_count":     ac.Stale,
		"offline_count":   ac.Offline,
		"listener_count":  totalListeners,
		"pending_count":   tc.Pending,
		"online_users":    onlineUsers,
		"completed_tasks": tc.Total - tc.Failed - tc.Pending,
		"server_version":  ServerVersion,
		"recent_tasks":    recentTasks,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// ── REST API: Audit Logs ──

func (s *Server) apiListAuditLogs(c *gin.Context) {
	role, _ := c.Get("user_role")
	if role != "admin" {
		respondError(c, http.StatusForbidden, "admin required")
		return
	}
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
	role, _ := c.Get("user_role")
	roleStr, _ := role.(string)
	if roleStr != "admin" && !db.RoleHasPermission(roleStr, db.PermTasksRead) {
		respondError(c, http.StatusForbidden, "insufficient permissions")
		return
	}

	s.bulkHistoryMu.Lock()
	results := make([]BulkResult, len(s.bulkHistory))
	copy(results, s.bulkHistory)
	s.bulkHistoryMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"success": true, "results": results, "total": len(results)})
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

	agentsRead := api.Group("/")
	agentsRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		agentsRead.GET("/dashboard", s.apiDashboardStats)
		agentsRead.GET("/agents", s.apiListAgents)
		agentsRead.GET("/agents/:id", s.apiGetAgent)
	}

	agentsWrite := api.Group("/")
	agentsWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		agentsWrite.POST("/tasks", s.apiCreateTask)
	}

	agentsDelete := api.Group("/")
	agentsDelete.Use(middleware.RequirePermission(db.PermAgentsDelete))
	{
		agentsDelete.DELETE("/agents/:id", s.apiDeleteAgent)
	}

	tasksRead := api.Group("/")
	tasksRead.Use(middleware.RequirePermission(db.PermTasksRead))
	{
		tasksRead.GET("/tasks", s.apiListTasks)
		tasksRead.GET("/tasks/:id", s.apiGetTask)
		tasksRead.GET("/task-types", s.apiListTaskTypes)
		tasksRead.POST("/tasks/status", s.apiBulkTaskStatus)
	}

	credsRead := api.Group("/")
	credsRead.Use(middleware.RequirePermission(db.PermCredsRead))
	{
		credsRead.GET("/credentials", s.apiListCredentials)
	}

	listenersRead := api.Group("/")
	listenersRead.Use(middleware.RequirePermission(db.PermListenersRead))
	{
		listenersRead.GET("/listeners", s.apiListListeners)
	}

	// These handlers enforce their own checks (admin / PermTasksRead).
	api.GET("/audit", s.apiListAuditLogs)
	api.GET("/bulk/results", s.apiBulkResults)
}
