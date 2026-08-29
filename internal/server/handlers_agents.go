package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) handleAgents(c *gin.Context) {
	search := c.Query("search")
	statusFilter := c.Query("status")
	osFilter := c.Query("os")
	p := parsePagination(c, DefaultPageSize, MaxPageSize)
	order := agentSortOrder(c.Query("sort_key"), c.Query("sort_dir"))

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
	if err := query.Order(order).Offset(p.Offset).Limit(p.PageSize).Find(&agents).Error; err != nil {
		handleQueryError(c, err, "Failed to list agents")
		return
	}

	for i := range agents {
		agents[i].Status = s.agentStatus(agents[i]).Status
	}

	stats := s.getNavStats(c)
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

// agentSortColumn maps a whitelisted sort key to its DB column. "status" is
// derived from last_seen (offline < stale < online monotonic in recency), so
// ordering by last_seen yields the same status ordering.
var agentSortColumn = map[string]string{
	"hostname":  "hostname",
	"username":  "username",
	"os":        "os",
	"ip":        "ip",
	"last_seen": "last_seen",
	"version":   "version",
	"status":    "last_seen",
}

func agentSortOrder(sortKey, sortDir string) string {
	col, ok := agentSortColumn[sortKey]
	if !ok {
		return "last_seen desc"
	}
	dir := "asc"
	if sortDir == "desc" {
		dir = "desc"
	}
	return col + " " + dir
}

func (s *Server) handleAgentDetail(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	q := s.db.Model(&db.Implant{})
	q = s.tenantScope(q, c)
	if err := q.First(&agent, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "agent not found")
		return
	}

	agent.Status = s.agentStatus(agent).Status

	var tasks []db.Task
	if err := s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(AgentDetailTaskLimit).Find(&tasks).Error; err != nil {
		handleQueryError(c, err, "Failed to query agent detail tasks")
		return
	}

	var screenshots []string
	if c.Query("include_screenshots") != "false" {
		screenshots, _ = s.listAgentScreenshots(id)
	}

	var totalTaskCount int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", id).Count(&totalTaskCount).Error; err != nil {
		handleQueryError(c, err, "Failed to count agent tasks")
		return
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
	if err := s.db.Where("parent_id = ?", id).Limit(500).Find(&children).Error; err != nil {
		handleQueryError(c, err, "Failed to query agent children")
		return
	}

	// Fetch unlinked agents (for linking dropdown) - optimized
	var unlinkedAgents []db.Implant
	if c.Query("include_unlinked") != "false" {
		if err := s.db.Select("id", "hostname", "ip", "os").
			Where("(parent_id = '' OR parent_id IS NULL) AND id != ?", id).Order("hostname asc").Limit(500).Find(&unlinkedAgents).Error; err != nil {
			handleQueryError(c, err, "Failed to query unlinked agents")
			return
		}
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
	}
	if c.Query("include_unlinked") != "false" {
		data["UnlinkedAgents"] = unlinkedAgents
	}

	stats := s.getNavStats(c)
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

	task, err := s.createTask(id, "kill", "exit", "", "", "", 0, 0, callerOpts(c)...)
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
				if len(req.Notes) > MaxNotesLength {
					respondError(c, http.StatusBadRequest, fmt.Sprintf("notes too long (max %d characters)", MaxNotesLength))
					return
				}
				updates["notes"] = req.Notes
			}
			if req.Tags != "" {
				if len(req.Tags) > MaxNotesLength {
					respondError(c, http.StatusBadRequest, fmt.Sprintf("tags too long (max %d characters)", MaxNotesLength))
					return
				}
				updates["tags"] = req.Tags
			}
		}
	} else {
		note := c.PostForm("notes")
		tags := c.PostForm("tags")
		if note != "" {
			if len(note) > MaxNotesLength {
				respondError(c, http.StatusBadRequest, fmt.Sprintf("notes too long (max %d characters)", MaxNotesLength))
				return
			}
			updates["notes"] = note
		}
		if tags != "" {
			if len(tags) > MaxNotesLength {
				respondError(c, http.StatusBadRequest, fmt.Sprintf("tags too long (max %d characters)", MaxNotesLength))
				return
			}
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

// deleteAgentRecordTx deletes an agent and all of its dependent rows within the
// given transaction. It does not commit or roll back -- the caller owns the
// transaction lifecycle (single-agent or whole-batch atomicity).
func (s *Server) deleteAgentRecordTx(tx *gorm.DB, id string) error {
	if err := tx.Where("agent_id = ?", id).Delete(&db.Task{}).Error; err != nil {
		return fmt.Errorf("delete tasks: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.CredentialEntry{}).Error; err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.TokenEntry{}).Error; err != nil {
		return fmt.Errorf("delete tokens: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.SocksSession{}).Error; err != nil {
		return fmt.Errorf("delete socks sessions: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.ScanResult{}).Error; err != nil {
		return fmt.Errorf("delete scan results: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.NetworkHost{}).Error; err != nil {
		return fmt.Errorf("delete network hosts: %w", err)
	}
	if err := tx.Where("implant_id = ?", id).Delete(&db.AgentTagAssignment{}).Error; err != nil {
		return fmt.Errorf("delete tag assignments: %w", err)
	}
	if err := tx.Where("implant_id = ?", id).Delete(&db.AgentGroupAssignment{}).Error; err != nil {
		return fmt.Errorf("delete group assignments: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.Notification{}).Error; err != nil {
		return fmt.Errorf("delete notifications: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.AgentLock{}).Error; err != nil {
		return fmt.Errorf("delete agent locks: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.OpsecHistory{}).Error; err != nil {
		return fmt.Errorf("delete opsec history: %w", err)
	}
	if err := tx.Where("agent_id = ?", id).Delete(&db.BloodHoundResult{}).Error; err != nil {
		return fmt.Errorf("delete bloodhound results: %w", err)
	}
	// Mesh peer rows: without this a deleted agent lived forever in the mesh
	// topology graph (nodes/edges built straight from mesh_peers).
	if err := tx.Where("agent_id = ? OR peer_id = ?", id, id).Delete(&db.MeshPeer{}).Error; err != nil {
		return fmt.Errorf("delete mesh peers: %w", err)
	}
	// Hard delete the implant (Unscoped) so a removed beacon is physically
	// purged and never resurrects via the soft-delete/reconcile path.
	if err := tx.Unscoped().Delete(&db.Implant{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	// Drop the in-memory pending-task counter and the ECDH session key —
	// the key must not outlive the implant row it authenticated.
	s.decPendingTasks(id)
	s.sessionManager.RemoveSession(id)
	return nil
}

func (s *Server) deleteAgentRecord(id string) bool {
	tx := s.db.Begin()
	if err := s.deleteAgentRecordTx(tx, id); err != nil {
		tx.Rollback()
		slog.Error("Failed to delete agent", "agent_id", id, "err", err)
		return false
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		slog.Error("Failed to commit agent deletion", "agent_id", id, "err", err)
		return false
	}
	screenshotsDir := safeJoin(filepath.Join(s.cfg.Server.DataDir, "screenshots"), id)
	if screenshotsDir != "" {
		if err := os.RemoveAll(screenshotsDir); err != nil {
			slog.Warn("Failed to remove agent screenshots", "agent_id", id, "err", err)
		}
	} else {
		slog.Warn("Invalid agent ID for screenshot cleanup", "agent_id", id)
	}
	return true
}

// implantHostKey matches the frontend host aggregation key:
// lowercase hostname, then IP / public IP, then session id.
func implantHostKey(a db.Implant) string {
	host := strings.ToLower(strings.TrimSpace(a.Hostname))
	if host != "" {
		return "h:" + host
	}
	ip := strings.TrimSpace(a.IP)
	if ip == "" {
		ip = strings.TrimSpace(a.PublicIP)
	}
	if ip != "" {
		return "ip:" + ip
	}
	return "id:" + a.ID
}

func (s *Server) implantListQuery(c *gin.Context) *gorm.DB {
	query := s.db.Model(&db.Implant{})
	// Multi-tenant isolation: operators only see agents in their tenant.
	query = s.tenantScope(query, c)
	if search := c.Query("search"); search != "" {
		like := "%" + escapeLike(search) + "%"
		query = query.Where("(hostname LIKE ? ESCAPE '\\' OR username LIKE ? ESCAPE '\\' OR ip LIKE ? ESCAPE '\\')", like, like, like)
	}
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-s.staleThreshold())
	switch c.Query("status") {
	case "online":
		query = query.Where("last_seen > ?", offlineCutoff)
	case "stale":
		query = query.Where("last_seen <= ? AND last_seen > ?", offlineCutoff, staleCutoff)
	case "offline":
		query = query.Where("last_seen <= ?", staleCutoff)
	}
	if osFilter := c.Query("os"); osFilter != "" {
		query = query.Where("LOWER(os) LIKE ? ESCAPE '\\'", "%"+escapeLike(osFilter)+"%")
	}
	switch c.Query("linked") {
	case "direct":
		query = query.Where("(parent_id = '' OR parent_id IS NULL)")
	case "chained":
		query = query.Where("parent_id != '' AND parent_id IS NOT NULL")
	}
	if tagID := c.Query("tag_id"); tagID != "" {
		query = query.Joins("JOIN agent_tag_assignments ON agent_tag_assignments.implant_id = implants.id").
			Where("agent_tag_assignments.agent_tag_id = ?", tagID)
	}
	return query
}

type implantHostBucket struct {
	key     string
	agents  []db.Implant
	sortAt  time.Time
	sortStr string
}

func groupImplantsByHost(matched []db.Implant) []*implantHostBucket {
	order := make([]*implantHostBucket, 0)
	index := make(map[string]*implantHostBucket, len(matched))
	for _, a := range matched {
		key := implantHostKey(a)
		g, ok := index[key]
		if !ok {
			g = &implantHostBucket{key: key, sortStr: key}
			index[key] = g
			order = append(order, g)
		}
		g.agents = append(g.agents, a)
		if a.LastSeen.After(g.sortAt) {
			g.sortAt = a.LastSeen
		}
		if hn := strings.ToLower(strings.TrimSpace(a.Hostname)); hn != "" {
			g.sortStr = hn
		}
	}
	return order
}

func sortHostBuckets(order []*implantHostBucket, sortKey, sortDir string) {
	desc := sortDir != "asc"
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		switch sortKey {
		case "hostname":
			if a.sortStr == b.sortStr {
				return a.sortAt.After(b.sortAt)
			}
			if desc {
				return a.sortStr > b.sortStr
			}
			return a.sortStr < b.sortStr
		default:
			if a.sortAt.Equal(b.sortAt) {
				return a.sortStr < b.sortStr
			}
			if desc {
				return a.sortAt.After(b.sortAt)
			}
			return a.sortAt.Before(b.sortAt)
		}
	})
}

func paginateHostBuckets(order []*implantHostBucket, page, pageSize int) ([]db.Implant, int64) {
	total := int64(len(order))
	if page < 1 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start > len(order) {
		start = len(order)
	}
	end := start + pageSize
	if end > len(order) {
		end = len(order)
	}
	var out []db.Implant
	for _, g := range order[start:end] {
		out = append(out, g.agents...)
	}
	return out, total
}

func (s *Server) listAgentsGroupedByHost(query *gorm.DB, page, pageSize int, sortKey, sortDir string) ([]db.Implant, int64, error) {
	var matched []db.Implant
	if err := query.Find(&matched).Error; err != nil {
		return nil, 0, err
	}
	order := groupImplantsByHost(matched)
	sortHostBuckets(order, sortKey, sortDir)
	out, total := paginateHostBuckets(order, page, pageSize)
	return out, total, nil
}

// handleListAgents returns agents as JSON. group=host paginates distinct hosts
// and returns every session on those hosts so the console can group without
// splitting a machine across pages.
func (s *Server) handleListAgents(c *gin.Context) {
	s.writeAgentListJSON(c)
}

type agentListItem struct {
	db.Implant
	TaskStats *db.TaskStats `json:"taskStats,omitempty"`
}

func parseAgentListPage(c *gin.Context) (page, pageSize int) {
	p := parsePagination(c, DefaultPageSize, MaxPageSize)
	return p.Page, p.PageSize
}

// writeAgentListJSON is the single operator list contract for GET /api/agents
// and GET /api/v1/agents: success + snake_case agents/data + pagination.
func (s *Server) writeAgentListJSON(c *gin.Context) {
	page, pageSize := parseAgentListPage(c)
	query := s.implantListQuery(c)
	var agents []db.Implant
	var total int64
	var err error
	groupHost := c.Query("group") == "host"
	if groupHost {
		agents, total, err = s.listAgentsGroupedByHost(query, page, pageSize, c.Query("sort_key"), c.Query("sort_dir"))
	} else if err = query.Count(&total).Error; err == nil {
		err = query.Order(agentSortOrder(c.Query("sort_key"), c.Query("sort_dir"))).
			Offset((page - 1) * pageSize).Limit(pageSize).Find(&agents).Error
	}
	if err != nil {
		handleQueryError(c, err, "Failed to list agents")
		return
	}

	ids := make([]string, len(agents))
	for i := range agents {
		ids[i] = agents[i].ID
		agents[i].Status = s.agentStatus(agents[i]).Status
	}
	stats := computeTaskStats(s.db, ids)
	resp := make([]agentListItem, len(agents))
	for i, a := range agents {
		resp[i] = agentListItem{Implant: a, TaskStats: stats[a.ID]}
	}
	out := gin.H{
		"success":   true,
		"agents":    resp,
		"data":      resp,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}
	if groupHost {
		out["group"] = "host"
	}
	c.JSON(http.StatusOK, out)
}

// handleListUnlinkedAgents returns agents without a parent for linking dropdown
func (s *Server) handleListUnlinkedAgents(c *gin.Context) {
	var agents []db.Implant
	q := s.db.Model(&db.Implant{})
	q = s.tenantScope(q, c)
	if err := q.Where("parent_id = '' OR parent_id IS NULL").Order("hostname asc").Limit(500).Find(&agents).Error; err != nil {
		handleQueryError(c, err, "Failed to list unlinked agents")
		return
	}
	respondSuccess(c, agents)
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
	newTrusted := !agent.Trusted
	// Targeted update: Save() here rewrote every column from a stale read and
	// reverted concurrent beacon writes (last_seen/status/sleep settings).
	if err := s.db.Model(&db.Implant{}).Where("id = ?", id).Update("trusted", newTrusted).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update agent trust status")
		return
	}
	slog.Info("Agent trust toggled", "agent_id", id, "trusted", newTrusted)
	respond(c, gin.H{"success": true, "trusted": newTrusted})
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
		// PushUpdateKey injects the server's update-signing public key into the
		// pushed config so the agent can verify signed self_update envelopes.
		PushUpdateKey bool `json:"push_update_key,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate every field first, then persist the changes in a single write
	// so concurrent pushes cannot overwrite each other between saves.
	updates := make(map[string]interface{})
	if req.Sleep != nil {
		if *req.Sleep < 1 || *req.Sleep > MaxSleepSeconds {
			respondError(c, http.StatusBadRequest, "sleep must be between 1 and 86400 seconds")
			return
		}
		updates["current_interval"] = *req.Sleep
	}
	if req.Jitter != nil {
		if *req.Jitter < 0 || *req.Jitter > MaxJitterPercent {
			respondError(c, http.StatusBadRequest, "jitter must be between 0 and 100")
			return
		}
		updates["current_jitter"] = *req.Jitter
	}
	if len(updates) > 0 {
		if err := s.db.Model(&agent).Updates(updates).Error; err != nil {
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "Update agent"))
			return
		}
	}

	taskData, ok := marshalJSONSafe(req)
	if !ok {
		respondError(c, http.StatusInternalServerError, "failed to marshal request")
		return
	}
	if req.PushUpdateKey {
		pubHex, err := payload.UpdateSigningPublicKeyHex()
		if err != nil {
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "update signing"))
			return
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(taskData, &cfg); err != nil {
			respondError(c, http.StatusInternalServerError, "failed to encode config push")
			return
		}
		cfg["update_pub_key"] = pubHex
		newData, merr := json.Marshal(cfg)
		if merr != nil {
			respondError(c, http.StatusInternalServerError, "failed to encode config push")
			return
		}
		taskData = newData
		s.LogAuditRecord(c, "push_update_key", "agent", id,
			"update signing key pushed (fingerprint "+pubHex[:16]+")", true, nil)
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	var req struct {
		KillDate string `json:"kill_date"` // YYYY-MM-DD
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	kt, err := time.ParseInLocation("2006-01-02", req.KillDate, time.Local)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	// Targeted update only -- a full-row Save() reverted concurrent beacon
	// writes on this hot row.
	if err := s.db.Model(&db.Implant{}).Where("id = ?", id).Update("kill_date", &kt).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to save kill date")
		return
	}

	// Create a task for the agent to pick up the kill date at next beacon
	task, err := s.createTask(id, "set_kill_date", req.KillDate, "", "", "", 0, 0, callerOpts(c)...)
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	// Targeted update only -- a full-row Save() reverted concurrent beacon
	// writes on this hot row.
	if err := s.db.Model(&db.Implant{}).Where("id = ?", id).Update("kill_date", nil).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to clear kill date")
		return
	}

	task, err := s.createTask(id, "clear_kill_date", "", "", "", "", 0, 0, callerOpts(c)...)
	if err != nil {
		slog.Error("Failed to create clear kill date task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create clear kill date task")
		return
	}

	s.dispatchTask(c, task, "clear_kill_date", "")
}
