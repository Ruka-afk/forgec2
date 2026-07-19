package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TimelineEvent represents a unified timeline event
type TimelineEvent struct {
	ID        uint      `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // audit, task, login, logout
	User      string    `json:"user"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	AgentID   string    `json:"agent_id,omitempty"`
	AgentName string    `json:"agent_name,omitempty"`
	IP        string    `json:"ip,omitempty"`
	Success   bool      `json:"success"`
}

// handleTimelinePage renders the action timeline page
func (s *Server) handleTimelinePage(c *gin.Context) {
	stats := s.getNavStats()

	// Get filter parameters
	filterType := c.Query("type")
	filterUser := c.Query("user")
	filterAgent := c.Query("agent")

	data := gin.H{
		"Title":       "ForgeC2 - Action Timeline",
		"ActiveNav":   "timeline",
		"Stats":       stats,
		"FilterType":  filterType,
		"FilterUser":  filterUser,
		"FilterAgent": filterAgent,
	}
	s.renderPageOrJSON(c, data)
}

// handleTimelineData returns timeline events as JSON
func (s *Server) handleTimelineData(c *gin.Context) {
	filterType := c.Query("type")
	filterUser := c.Query("user")
	filterAgent := c.Query("agent")
	dateFrom := c.Query("from")
	dateTo := c.Query("to")
	limit := 200

	// Get audit logs
	var auditLogs []struct {
		ID        uint
		Timestamp time.Time
		User      string
		Action    string
		Details   string
		AgentID   string
		IP        string
		Success   bool
	}

	query := s.db.Table("audit_logs").Select("id, created_at as timestamp, user, action, details, agent_id, ip, success")

	if filterType != "" && filterType != "audit" {
		// Skip audit logs if filtering by other types
	} else {
		if filterUser != "" {
		query = query.Where("user LIKE ? ESCAPE '\\'", "%"+escapeLike(filterUser)+"%")
		}
		if filterAgent != "" {
		query = query.Where("agent_id LIKE ? ESCAPE '\\'", "%"+escapeLike(filterAgent)+"%")
		}
		if dateFrom != "" {
			query = query.Where("created_at >= ?", dateFrom)
		}
		if dateTo != "" {
			query = query.Where("created_at <= ?", dateTo)
		}
	}

	query.Order("created_at desc").Limit(limit).Find(&auditLogs)

	// Get tasks
	var tasks []struct {
		ID        uint
		Timestamp time.Time
		AgentID   string
		Type      string
		Command   string
		Result    string
		Status    string
	}

	taskQuery := s.db.Table("tasks").Select("id, created_at as timestamp, agent_id, type, command, result, status")

	if filterType == "" || filterType == "task" {
		if filterAgent != "" {
			taskQuery = taskQuery.Where("agent_id LIKE ? ESCAPE '\\'", "%"+escapeLike(filterAgent)+"%")
		}
		if dateFrom != "" {
			taskQuery = taskQuery.Where("created_at >= ?", dateFrom)
		}
		if dateTo != "" {
			taskQuery = taskQuery.Where("created_at <= ?", dateTo)
		}
		taskQuery.Order("created_at desc").Limit(limit).Find(&tasks)
	}

	// Build unified timeline
	events := make([]TimelineEvent, 0)

	// Batch-load agent hostnames to avoid N+1 queries
	agentIDs := make(map[string]bool)
	for _, log := range auditLogs {
		if log.AgentID != "" {
			agentIDs[log.AgentID] = true
		}
	}
	for _, task := range tasks {
		if task.AgentID != "" {
			agentIDs[task.AgentID] = true
		}
	}
	agentNames := make(map[string]string)
	if len(agentIDs) > 0 {
		ids := make([]string, 0, len(agentIDs))
		for id := range agentIDs {
			ids = append(ids, id)
		}
		var agents []struct {
			ID       string
			Hostname string
		}
		s.db.Table("agents").Select("id, hostname").Where("id IN ?", ids).Find(&agents)
		for _, a := range agents {
			agentNames[a.ID] = a.Hostname
		}
	}

	// Add audit logs
	for _, log := range auditLogs {
		agentName := agentNames[log.AgentID]

		events = append(events, TimelineEvent{
			ID:        log.ID,
			Timestamp: log.Timestamp,
			Type:      "audit",
			User:      log.User,
			Action:    log.Action,
			Details:   log.Details,
			AgentID:   log.AgentID,
			AgentName: agentName,
			IP:        log.IP,
			Success:   log.Success,
		})
	}

	// Add tasks
	for _, task := range tasks {
		agentName := agentNames[task.AgentID]

		statusIcon := "[OK]"
		if task.Status == "failed" || task.Status == "error" {
			statusIcon = "[FAIL]"
		} else if task.Status == "pending" {
			statusIcon = "[...]"
		}

		events = append(events, TimelineEvent{
			ID:        task.ID,
			Timestamp: task.Timestamp,
			Type:      "task",
			Action:    fmt.Sprintf("[%s] %s %s", statusIcon, task.Type, task.Command),
			Details:   task.Result,
			AgentID:   task.AgentID,
			AgentName: agentName,
			Success:   task.Status == "completed",
		})
	}

	// Sort by timestamp (newest first)
	// Already sorted by individual queries, but could merge sort here if needed

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// handleTimelineExport exports timeline as CSV
func (s *Server) handleTimelineExport(c *gin.Context) {
	// Get events
	filterUser := c.Query("user")
	filterAgent := c.Query("agent")

	// Reuse handleTimelineData logic
	// For simplicity, just get recent audit logs
	var auditLogs []struct {
		Timestamp time.Time
		User      string
		Action    string
		Details   string
		AgentID   string
		IP        string
		Success   bool
	}

	query := s.db.Table("audit_logs").Select("created_at as timestamp, user, action, details, agent_id, ip, success")
	if filterUser != "" {
		query = query.Where("user LIKE ? ESCAPE '\\'", "%"+escapeLike(filterUser)+"%")
	}
	if filterAgent != "" {
		query = query.Where("agent_id LIKE ? ESCAPE '\\'", "%"+escapeLike(filterAgent)+"%")
	}
	query.Order("created_at desc").Limit(1000).Find(&auditLogs)

	// Generate CSV
	c.Header("Content-Disposition", "attachment; filename=timeline_export.csv")
	c.Header("Content-Type", "text/csv")
	c.Writer.WriteString("Timestamp,User,Action,Details,Agent ID,IP,Success\n")

	for _, log := range auditLogs {
		c.Writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%t\n",
			log.Timestamp.Format("2006-01-02 15:04:05"),
			csvSanitize(log.User),
			csvSanitize(log.Action),
			csvSanitize(log.Details),
			csvSanitize(log.AgentID),
			csvSanitize(log.IP),
			log.Success,
		))
	}
}

