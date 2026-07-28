package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAgents(c *gin.Context) {
	search := c.Query("search")
	statusFilter := c.Query("status")
	osFilter := c.Query("os")
	p := parsePagination(c, DefaultPageSize, MaxPageSize)

	query := s.db.Model(&db.Implant{})
	if search != "" {
		query = query.Where("(hostname LIKE ? ESCAPE '\\' OR username LIKE ? ESCAPE '\\' OR ip LIKE ? ESCAPE '\\')",
			"%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%")
	}
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-s.staleThreshold())
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

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to count agents")
		return
	}

	var agents []db.Implant
	query.Order("last_seen desc").Offset(p.Offset).Limit(p.PageSize).Find(&agents)

	for i := range agents {
		agents[i].Status = s.agentStatus(agents[i]).Status
	}

	stats := s.getNavStats()
	totalPages := (int(total) + p.PageSize - 1) / p.PageSize
	prevPage := p.Page - 1
	nextPage := p.Page + 1
	data := gin.H{
		"Title":      "ForgeC2 - Agents",
		"ActiveNav":  "agents",
		"Agents":     agents,
		"Search":     search,
		"Status":     statusFilter,
		"FilterOS":   osFilter,
		"Page":       p.Page,
		"PrevPage":   prevPage,
		"NextPage":   nextPage,
		"PageSize":   p.PageSize,
		"Total":      int(total),
		"TotalPages": totalPages,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

func (s *Server) handleAgentDetail(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "agent not found")
		return
	}

	agent.Status = s.agentStatus(agent).Status

	var tasks []db.Task
	s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(AgentDetailTaskLimit).Find(&tasks)

	var screenshots []string
	if c.Query("include_screenshots") != "false" {
		screenshots, _ = s.listAgentScreenshots(id)
	}

	var totalTaskCount int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", id).Count(&totalTaskCount).Error; err != nil {
		slog.Error("Failed to count agent tasks", "agent_id", id, "error", err)
	}
	taskStats := computeTaskStats(s.db, []string{id})[id]
	if taskStats == nil {
		taskStats = &db.TaskStats{}
	}
	totalTasks := int(totalTaskCount)
	completedTasks := taskStats.Completed
	pendingTasks := taskStats.Pending
	failedTasks := taskStats.Failed
	totalResponseTime := time.Duration(0)
	shellTasks := 0
	screenshotTasks := 0
	psTasks := 0
	killTasks := 0

	for _, t := range tasks {
		switch t.Status {
		case "completed":
			totalResponseTime += t.UpdatedAt.Sub(t.CreatedAt)
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
	terminalTasks := completedTasks + failedTasks
	if terminalTasks > 0 {
		successRate = (completedTasks * 100) / terminalTasks
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
	s.db.Where("parent_id = ?", id).Limit(500).Find(&children)

	// Fetch unlinked agents (for linking dropdown) - optimized
	var unlinkedAgents []db.Implant
	if err := s.db.Select("id", "hostname", "ip", "os").
		Where("(parent_id = '' OR parent_id IS NULL) AND id != ?", id).Order("hostname asc").Limit(500).Find(&unlinkedAgents).Error; err != nil {
		slog.Error("Failed to query unlinked agents", "error", err)
	}

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

	stats := s.getNavStats()
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

func (s *Server) listAgentScreenshots(agentID string) ([]string, error) {
	screenshotDir := filepath.Join(s.cfg.Server.DataDir, "screenshots", agentID)
	files, err := os.ReadDir(screenshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	screenshots := make([]string, 0, len(files))
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".png") {
			screenshots = append(screenshots, f.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(screenshots)))
	return screenshots, nil
}

func (s *Server) handleListAgentScreenshots(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.Select("id").First(&agent, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "agent not found")
		return
	}

	screenshots, err := s.listAgentScreenshots(id)
	if err != nil {
		slog.Error("Failed to list agent screenshots", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to list screenshots")
		return
	}

	p := parsePagination(c, DefaultPageSize, MaxPageSize)
	start := p.Offset
	if start > len(screenshots) {
		start = len(screenshots)
	}
	end := start + p.PageSize
	if end > len(screenshots) {
		end = len(screenshots)
	}
	respondSuccess(c, gin.H{
		"screenshots": screenshots[start:end],
		"total":       len(screenshots),
		"page":        p.Page,
		"page_size":   p.PageSize,
	})
}

func (s *Server) handleKillAgent(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "kill", "exit", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create kill task", "agent_id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create kill task")
		return
	}

	slog.Info("Kill task created", "agent_id", id)
	s.LogAuditRecord(c, "kill_agent", "agent", id, "kill command", true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": "Kill command sent. Agent will exit on next beacon."})
}

func (s *Server) handleUpdateNote(c *gin.Context) {
	id := c.Param("id")
	updates := map[string]interface{}{}

	if strings.Contains(c.ContentType(), "application/json") {
		var req struct {
			Notes string `json:"notes"`
			Tags  string `json:"tags"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			if req.Notes != "" {
				updates["notes"] = req.Notes
			}
			if req.Tags != "" {
				updates["tags"] = req.Tags
			}
		}
	} else {
		note := c.PostForm("notes")
		tags := c.PostForm("tags")
		if note != "" {
			updates["notes"] = note
		}
		if tags != "" {
			updates["tags"] = tags
		}
	}

	if len(updates) > 0 {
		if err := s.db.Model(&db.Implant{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "failed to update agent notes/tags")
			return
		}
	}
	s.LogAuditRecord(c, "update_notes", "agent", id, fmt.Sprintf("notes/tags updated"), true, nil)
	s.broadcastAgentDataUpdate(id, updates)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleDeleteAgent(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "agent not found")
		return
	}
	s.LogAuditRecord(c, "agent_delete", "agent", id, fmt.Sprintf("Deleted agent %s", agent.Hostname), true, nil)
	if !s.deleteAgentRecord(id) {
		respondError(c, http.StatusInternalServerError, "failed to delete agent")
		return
	}
	slog.Warn("Agent deleted", "agent_id", id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) deleteAgentRecord(id string) bool {
	tx := s.db.Begin()
	if err := tx.Where("agent_id = ?", id).Delete(&db.Task{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete tasks", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.CredentialEntry{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete credentials", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.TokenEntry{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete tokens", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.SocksSession{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete socks sessions", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.ScanResult{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete scan results", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.NetworkHost{}).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete network hosts", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Delete(&db.Implant{}, "id = ?", id).Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to delete agent", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Commit().Error; err != nil {
		slog.Error("Failed to commit agent deletion", "agent_id", id, "err", err)
		return false
	}
	if err := os.RemoveAll(filepath.Join(s.cfg.Server.DataDir, "screenshots", id)); err != nil {
		slog.Warn("Failed to remove agent screenshots", "agent_id", id, "err", err)
	}
	return true
}

// handleListAgents returns all agents as JSON for dropdowns
func (s *Server) handleListAgents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "0"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	var total int64
	s.db.Model(&db.Implant{}).Count(&total)

	var agents []db.Implant
	offset := (page - 1) * pageSize
	if err := s.db.Order("hostname asc").Offset(offset).Limit(pageSize).Find(&agents).Error; err != nil {
		slog.Error("Failed to list agents", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to list agents")
		return
	}
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
			Status:   s.agentStatus(a).Status,
			OS:       a.OS,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"agents":   results,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// handleListUnlinkedAgents returns agents without a parent for linking dropdown
func (s *Server) handleListUnlinkedAgents(c *gin.Context) {
	var agents []db.Implant
	if err := s.db.Where("parent_id = '' OR parent_id IS NULL").Order("hostname asc").Limit(500).Find(&agents).Error; err != nil {
		slog.Error("Failed to list unlinked agents", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to list agents")
		return
	}
	c.JSON(http.StatusOK, agents)
}

// handleToggleAgentTrust toggles the trusted status of an agent.
// POST /agents/:id/trust
func (s *Server) handleToggleAgentTrust(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "agent not found")
		return
	}
	agent.Trusted = !agent.Trusted
	if err := s.db.Save(&agent).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update agent trust status")
		return
	}
	slog.Info("Agent trust toggled", "agent_id", id, "trusted", agent.Trusted)
	respond(c, gin.H{"success": true, "trusted": agent.Trusted})
}

// handleGetAgentConfig returns the effective config for a specific agent.
// GET /agents/:id/config
func (s *Server) handleGetAgentConfig(c *gin.Context) {
	id := c.Param("id")
	agent, ok := s.getAgentOrFail(c, id)
	if !ok {
		return
	}

	sleep := s.cfg.Implant.DefaultInterval
	if agent.CurrentInterval > 0 {
		sleep = agent.CurrentInterval
	}
	jitter := s.cfg.Implant.DefaultJitter
	if agent.CurrentJitter > 0 {
		jitter = agent.CurrentJitter
	}
	ua := s.cfg.Implant.DefaultUA

	var pendingTask db.Task
	hasPending := false
	var pendingTaskID uint
	if err := s.db.Where("agent_id = ? AND status = 'pending' AND type = 'config_push'", id).First(&pendingTask).Error; err == nil {
		hasPending = true
		pendingTaskID = pendingTask.ID
	}

	respond(c, gin.H{
		"success": true,
		"data": gin.H{
			"agent_id":        id,
			"effective":       gin.H{"sleep": sleep, "jitter": jitter, "user_agent": ua, "headers": nil, "beacon_uri": "/beacon", "method": "POST"},
			"has_pending":     hasPending,
			"pending_task_id": pendingTaskID,
		},
	})
}

// handlePushAgentConfig queues a config_push task for the agent.
// POST /agents/:id/config
func (s *Server) handlePushAgentConfig(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	agent, ok := s.getAgentOrFail(c, id)
	if !ok {
		return
	}

	var req struct {
		Sleep     *int              `json:"sleep"`
		Jitter    *int              `json:"jitter"`
		UserAgent string            `json:"user_agent"`
		Headers   map[string]string `json:"headers"`
		BeaconURI string            `json:"beacon_uri"`
		Method    string            `json:"method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Sleep != nil {
		if *req.Sleep < 1 || *req.Sleep > MaxSleepSeconds {
			respondError(c, http.StatusBadRequest, "sleep must be between 1 and 86400 seconds")
			return
		}
		agent.CurrentInterval = *req.Sleep
		if err := s.db.Save(&agent).Error; err != nil {
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "Update agent"))
			return
		}
	}
	if req.Jitter != nil {
		if *req.Jitter < 0 || *req.Jitter > MaxJitterPercent {
			respondError(c, http.StatusBadRequest, "jitter must be between 0 and 100")
			return
		}
		agent.CurrentJitter = *req.Jitter
		if err := s.db.Save(&agent).Error; err != nil {
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "Update agent"))
			return
		}
	}

	taskData, ok := marshalJSONSafe(req)
	if !ok {
		respondError(c, http.StatusInternalServerError, "failed to marshal request")
		return
	}
	task := db.Task{
		AgentID:   id,
		Type:      "config_push",
		Command:   "config_push",
		Data:      string(taskData),
		Status:    "pending",
		CreatedBy: "operator",
	}
	if err := s.db.Create(&task).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create config push task")
		return
	}

	s.broadcastTaskUpdate(id, task)
	slog.Info("Config push task created", "agent_id", id, "task_id", task.ID)
	respond(c, gin.H{"success": true, "task_id": task.ID, "message": "Config push task queued for next beacon"})
}

func (s *Server) handleSetKillDate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	agent, ok := s.getAgentOrFail(c, id)
	if !ok {
		return
	}

	var req struct {
		KillDate string `json:"kill_date"` // YYYY-MM-DD
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	kt, err := time.Parse("2006-01-02", req.KillDate)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	agent.KillDate = &kt
	if err := s.db.Save(&agent).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to save kill date")
		return
	}

	// Create a task for the agent to pick up the kill date at next beacon
	task, err := s.createTask(id, "set_kill_date", req.KillDate, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create kill date task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create kill date task")
		return
	}

	s.dispatchTask(c, task, "set_kill_date", req.KillDate)
}

func (s *Server) handleClearKillDate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	agent, ok := s.getAgentOrFail(c, id)
	if !ok {
		return
	}

	agent.KillDate = nil
	if err := s.db.Save(&agent).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to clear kill date")
		return
	}

	task, err := s.createTask(id, "clear_kill_date", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create clear kill date task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create clear kill date task")
		return
	}

	s.dispatchTask(c, task, "clear_kill_date", "")
}
