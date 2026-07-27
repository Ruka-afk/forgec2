package server

import (
	"encoding/json"
	"fmt"
	htmlesc "html"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleReportPage renders the report generator page
func (s *Server) handleReportPage(c *gin.Context) {
	stats := s.getNavStats()

	// Get summary data
	var totalAgents int64
	s.db.Model(&db.Implant{}).Count(&totalAgents)

	var onlineAgents int64
	s.db.Model(&db.Implant{}).Where("status = ?", "online").Count(&onlineAgents)

	var totalTasks int64
	s.db.Model(&db.Task{}).Count(&totalTasks)

	var completedTasks int64
	s.db.Model(&db.Task{}).Where("status = ?", "completed").Count(&completedTasks)

	var totalCredentials int64
	s.db.Model(&db.CredentialEntry{}).Count(&totalCredentials)

	var totalAudits int64
	s.db.Model(&db.AuditLog{}).Count(&totalAudits)

	// Get date range
	var firstAgent db.Implant
	var startDate time.Time
	if err := s.db.Order("created_at asc").First(&firstAgent).Error; err == nil {
		startDate = firstAgent.CreatedAt
	} else {
		startDate = time.Now()
	}

	data := gin.H{
		"Title":          "ForgeC2 - Report Generator",
		"ActiveNav":      "report",
		"Stats":          stats,
		"TotalAgents":    totalAgents,
		"OnlineAgents":   onlineAgents,
		"TotalTasks":     totalTasks,
		"CompletedTasks": completedTasks,
		"TotalCreds":     totalCredentials,
		"TotalAudits":    totalAudits,
		"StartDate":      startDate.Format("2006-01-02"),
		"EndDate":        time.Now().Format("2006-01-02"),
	}
	s.renderPageOrJSON(c, data)
}

// handleGenerateReport generates a comprehensive report
func (s *Server) handleGenerateReport(c *gin.Context) {
	var req struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Include   struct {
			Agents      bool `json:"agents"`
			Tasks       bool `json:"tasks"`
			Creds       bool `json:"creds"`
			Screenshots bool `json:"screenshots"`
			Audit       bool `json:"audit"`
		} `json:"include"`
		Format string `json:"format"` // html, json
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	// Validate format whitelist
	if req.Format == "" {
		req.Format = "html"
	}
	if req.Format != "html" && req.Format != "json" {
		respondError(c, http.StatusBadRequest, "invalid format: must be 'html' or 'json'")
		return
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		startDate = time.Now().AddDate(0, -1, 0) // Default: last month
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		endDate = time.Now()
	}
	endDate = endDate.Add(24*time.Hour - 1*time.Second) // End of day

	// Validate date range
	if startDate.After(endDate) {
		respondError(c, http.StatusBadRequest, "start_date must be before end_date")
		return
	}
	// Cap date range to 365 days
	if endDate.Sub(startDate) > MaxReportDateRange {
		respondError(c, http.StatusBadRequest, "date range cannot exceed 365 days")
		return
	}

	// Build report
	report := gin.H{
		"title":       "ForgeC2 Action Report",
		"generated":   time.Now().Format("2006-01-02 15:04:05"),
		"date_range":  fmt.Sprintf("%s to %s", req.StartDate, req.EndDate),
		"summary":     gin.H{},
		"agents":      []gin.H{},
		"tasks":       []gin.H{},
		"credentials": []gin.H{},
		"audit":       []gin.H{},
	}

	// Summary
	var agentCount, taskCount, credCount, auditCount int64
	if req.Include.Agents {
		s.db.Model(&db.Implant{}).Where("created_at BETWEEN ? AND ?", startDate, endDate).Count(&agentCount)
	}
	if req.Include.Tasks {
		s.db.Model(&db.Task{}).Where("created_at BETWEEN ? AND ?", startDate, endDate).Count(&taskCount)
	}
	if req.Include.Creds {
		s.db.Model(&db.CredentialEntry{}).Where("created_at BETWEEN ? AND ?", startDate, endDate).Count(&credCount)
	}
	if req.Include.Audit {
		s.db.Model(&db.AuditLog{}).Where("created_at BETWEEN ? AND ?", startDate, endDate).Count(&auditCount)
	}

	report["summary"] = gin.H{
		"total_agents": agentCount,
		"total_tasks":  taskCount,
		"total_creds":  credCount,
		"total_audits": auditCount,
		"success_rate": fmt.Sprintf("%.1f%%", float64(taskCount)/float64(agentCount+1)*100),
	}

	// Agents
	if req.Include.Agents {
		var agents []db.Implant
		s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(5000).Find(&agents)
		agentList := make([]gin.H, 0, len(agents))
		for _, a := range agents {
			agentList = append(agentList, gin.H{
				"id":       a.ID,
				"hostname": a.Hostname,
				"os":       a.OS,
				"ip":       a.IP,
				"user":     a.Username,
				"status":   a.Status,
				"created":  a.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["agents"] = agentList
	}

	// Tasks
	if req.Include.Tasks {
		var tasks []db.Task
		s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(100).Find(&tasks)
		taskList := make([]gin.H, 0, len(tasks))
		for _, t := range tasks {
			taskList = append(taskList, gin.H{
				"id":      t.ID,
				"agent":   t.AgentID,
				"type":    t.Type,
				"command": t.Command,
				"status":  t.Status,
				"created": t.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["tasks"] = taskList
	}

	// Credentials
	if req.Include.Creds {
		var creds []db.CredentialEntry
		s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(100).Find(&creds)
		credList := make([]gin.H, 0, len(creds))
		for _, c := range creds {
			credList = append(credList, gin.H{
				"id":       c.ID,
				"agent":    c.AgentID,
				"type":     c.Type,
				"username": c.Username,
				"source":   c.Source,
				"created":  c.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["credentials"] = credList
	}

	// Audit
	if req.Include.Audit {
		var audits []db.AuditLog
		s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(100).Find(&audits)
		auditList := make([]gin.H, 0, len(audits))
		for _, a := range audits {
			auditList = append(auditList, gin.H{
				"id":      a.ID,
				"user":    a.User,
				"action":  a.Action,
				"details": a.Details,
				"success": a.Success,
				"created": a.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["audit"] = auditList
	}

	// Generate output
	if req.Format == "json" {
		c.JSON(http.StatusOK, report)
		return
	}

	// Generate HTML report
	html := generateHTMLReport(report)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// generateHTMLReport creates a formatted HTML report
func generateHTMLReport(report gin.H) string {
	summary := report["summary"].(gin.H)
	agents := report["agents"].([]gin.H)
	tasks := report["tasks"].([]gin.H)
	creds := report["credentials"].([]gin.H)
	audits := report["audit"].([]gin.H)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; background: #f8fafc; color: #1e293b; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 40px; border-radius: 12px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        h1 { color: #4f46e5; border-bottom: 3px solid #4f46e5; padding-bottom: 10px; }
        h2 { color: #334155; margin-top: 40px; border-bottom: 2px solid #e2e8f0; padding-bottom: 8px; }
        .meta { color: #64748b; font-size: 14px; margin-bottom: 30px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 30px 0; }
        .stat-card { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 20px; border-radius: 10px; }
        .stat-value { font-size: 32px; font-weight: bold; }
        .stat-label { font-size: 12px; opacity: 0.9; text-transform: uppercase; letter-spacing: 1px; }
        table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
        th { background: #f1f5f9; color: #475569; font-weight: 600; text-align: left; padding: 12px; border-bottom: 2px solid #e2e8f0; }
        td { padding: 10px 12px; border-bottom: 1px solid #f1f5f9; }
        tr:hover { background: #f8fafc; }
        .badge { display: inline-block; padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
        .badge-success { background: #dcfce7; color: #166534; }
        .badge-failed { background: #fee2e2; color: #991b1b; }
        .badge-pending { background: #fef3c7; color: #92400e; }
    </style>
</head>
<body>
    <div class="container">
        <h1>馃洝锔?%s</h1>
        <div class="meta">
            Generated: %s | Date Range: %s
        </div>

        <h2>馃搳 Summary</h2>
        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Agents</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Tasks Executed</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Credentials Found</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Audit Events</div>
            </div>
        </div>

`, report["title"], report["title"], report["generated"], report["date_range"],
		summary["total_agents"].(int64), summary["total_tasks"].(int64),
		summary["total_creds"].(int64), summary["total_audits"].(int64))

	// Agents table
	if len(agents) > 0 {
		html += `<h2>馃 Agents</h2>
<table>
    <tr><th>Hostname</th><th>OS</th><th>IP</th><th>User</th><th>Status</th><th>Created</th></tr>
`
		for _, a := range agents {
			html += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><span class=\"badge badge-%s\">%s</span></td><td>%s</td></tr>\n",
				htmlesc.EscapeString(fmt.Sprint(a["hostname"])), htmlesc.EscapeString(fmt.Sprint(a["os"])), htmlesc.EscapeString(fmt.Sprint(a["ip"])), htmlesc.EscapeString(fmt.Sprint(a["user"])),
				func() string {
					if a["status"] == "online" {
						return "success"
					}
					return "pending"
				}(),
				htmlesc.EscapeString(fmt.Sprint(a["status"])), htmlesc.EscapeString(fmt.Sprint(a["created"])))
		}
		html += "</table>\n"
	}

	// Tasks table
	if len(tasks) > 0 {
		html += `<h2>馃搵 Tasks</h2>
<table>
    <tr><th>Type</th><th>Command</th><th>Agent</th><th>Status</th><th>Created</th></tr>
`
		for _, t := range tasks {
			statusBadge := "pending"
			if t["status"] == "completed" {
				statusBadge = "success"
			} else if t["status"] == "failed" || t["status"] == "error" {
				statusBadge = "failed"
			}
			html += fmt.Sprintf("<tr><td>%s</td><td><code>%s</code></td><td>%s</td><td><span class=\"badge badge-%s\">%s</span></td><td>%s</td></tr>\n",
				htmlesc.EscapeString(fmt.Sprint(t["type"])), htmlesc.EscapeString(fmt.Sprint(t["command"])), htmlesc.EscapeString(fmt.Sprint(t["agent"])), statusBadge, htmlesc.EscapeString(fmt.Sprint(t["status"])), htmlesc.EscapeString(fmt.Sprint(t["created"])))
		}
		html += "</table>\n"
	}

	// Credentials table
	if len(creds) > 0 {
		html += `<h2>馃攽 Credentials</h2>
<table>
    <tr><th>Type</th><th>Username</th><th>Source</th><th>Agent</th><th>Created</th></tr>
`
		for _, c := range creds {
			html += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
				htmlesc.EscapeString(fmt.Sprint(c["type"])), htmlesc.EscapeString(fmt.Sprint(c["username"])), htmlesc.EscapeString(fmt.Sprint(c["source"])), htmlesc.EscapeString(fmt.Sprint(c["agent"])), htmlesc.EscapeString(fmt.Sprint(c["created"])))
		}
		html += "</table>\n"
	}

	// Audit table
	if len(audits) > 0 {
		html += `<h2>馃摑 Audit Log</h2>
<table>
    <tr><th>User</th><th>Action</th><th>Details</th><th>Success</th><th>Created</th></tr>
`
		for _, a := range audits {
			successBadge := "success"
			if !a["success"].(bool) {
				successBadge = "failed"
			}
			html += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td><span class=\"badge badge-%s\">%v</span></td><td>%s</td></tr>\n",
				htmlesc.EscapeString(fmt.Sprint(a["user"])), htmlesc.EscapeString(fmt.Sprint(a["action"])), htmlesc.EscapeString(fmt.Sprint(a["details"])), successBadge, a["success"], htmlesc.EscapeString(fmt.Sprint(a["created"])))
		}
		html += "</table>\n"
	}

	html += `
        <div style="margin-top: 40px; padding-top: 20px; border-top: 2px solid #e2e8f0; color: #64748b; font-size: 12px; text-align: center;">
            Generated by ForgeC2 Professional Red Team Framework
        </div>
    </div>
</body>
</html>`

	return html
}

func parseReportDates(c *gin.Context) (time.Time, time.Time, error) {
	startStr := c.Query("start")
	endStr := c.Query("end")
	var startDate, endDate time.Time
	var parseErr error
	if startStr != "" {
		startDate, parseErr = time.Parse("2006-01-02", startStr)
		if parseErr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start date: %w", parseErr)
		}
	}
	if endStr != "" {
		endDate, parseErr = time.Parse("2006-01-02", endStr)
		if parseErr != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end date: %w", parseErr)
		}
		endDate = endDate.Add(24*time.Hour - time.Second)
	}
	if startDate.IsZero() {
		startDate = time.Now().AddDate(0, -1, 0)
	}
	if endDate.IsZero() {
		endDate = time.Now()
	}
	return startDate, endDate, nil
}

func (s *Server) handleAPIGetReportAgents(c *gin.Context) {
	startDate, endDate, err := parseReportDates(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	var agents []db.Implant
	s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Find(&agents)
	agentList := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		agentList = append(agentList, gin.H{
			"id": a.ID, "hostname": a.Hostname, "os": a.OS, "ip": a.IP,
			"user": a.Username, "status": a.Status, "created": a.CreatedAt.Format("2006-01-02 15:04:05"),
			"version": a.Version, "integrity": a.Integrity,
		})
	}
	c.JSON(http.StatusOK, gin.H{"agents": agentList})
}

func (s *Server) handleAPIGetReportTasks(c *gin.Context) {
	startDate, endDate, err := parseReportDates(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	var tasks []db.Task
	s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(200).Find(&tasks)
	var completed, failed, pending int
	taskList := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		entry := gin.H{
			"id": t.ID, "agent": t.AgentID, "type": t.Type,
			"command": t.Command, "status": t.Status, "created": t.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		taskList = append(taskList, entry)
		switch t.Status {
		case "completed":
			completed++
		case "failed", "error":
			failed++
		default:
			pending++
		}
	}
	c.JSON(http.StatusOK, gin.H{"stats": gin.H{
		"total":     len(tasks),
		"completed": completed,
		"failed":    failed,
		"pending":   pending,
	}, "tasks": taskList})
}

func (s *Server) handleAPIGetReportCredentials(c *gin.Context) {
	startDate, endDate, err := parseReportDates(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	var creds []db.CredentialEntry
	s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(100).Find(&creds)
	credList := make([]gin.H, 0, len(creds))
	for _, c := range creds {
		credList = append(credList, gin.H{
			"id": c.ID, "agent": c.AgentID, "type": c.Type,
			"username": c.Username, "source": c.Source, "host": c.Domain,
			"created": c.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"credentials": credList})
}

func (s *Server) handleAPIGetReportNetwork(c *gin.Context) {
	var hosts []db.NetworkHost
	s.db.Order("last_seen desc").Limit(100).Find(&hosts)
	hostList := make([]gin.H, 0, len(hosts))
	for _, h := range hosts {
		hostList = append(hostList, gin.H{
			"id": h.ID, "agent": h.AgentID, "ip": h.IP,
			"hostname": h.Hostname, "os": h.OS, "services": h.Services,
			"last_seen": h.LastSeen.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"listeners": hostList})
}

func (s *Server) handleAPIGetReportFindings(c *gin.Context) {
	startDate, endDate, err := parseReportDates(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	var creds []db.CredentialEntry
	s.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc").Limit(50).Find(&creds)
	findings := make([]gin.H, 0)
	for _, c := range creds {
		findings = append(findings, gin.H{
			"severity": "medium",
			"title":    fmt.Sprintf("Credential Found: %s", c.Type),
			"detail":   fmt.Sprintf("Username %s found on %s via %s", c.Username, c.Domain, c.Source),
			"source":   c.AgentID,
			"created":  c.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	var tasks []db.Task
	s.db.Where("status = ? AND created_at BETWEEN ? AND ?", "failed", startDate, endDate).Order("created_at desc").Limit(50).Find(&tasks)
	for _, t := range tasks {
		findings = append(findings, gin.H{
			"severity": "low",
			"title":    fmt.Sprintf("Task Failed: %s", t.Type),
			"detail":   fmt.Sprintf("Task %s on agent %s: %s", t.Type, t.AgentID, t.Error),
			"source":   t.AgentID,
			"created":  t.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"findings": findings})
}

func (s *Server) handleAPIGetReportHistory(c *gin.Context) {
	var reports []db.GeneratedReport
	s.db.Order("created_at desc").Limit(20).Find(&reports)
	reportList := make([]gin.H, 0, len(reports))
	for _, r := range reports {
		var sections []string
		if r.Sections != "" {
			json.Unmarshal([]byte(r.Sections), &sections)
		}
		reportList = append(reportList, gin.H{
			"id": r.ID, "name": r.Name, "template": r.Template,
			"format": r.Format, "sections": sections,
			"created": r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"reports": reportList})
}

func (s *Server) handleAPIExportReportPDF(c *gin.Context) {
	startDate, endDate, err := parseReportDates(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	template := c.DefaultQuery("template", "technical")

	req := struct {
		StartDate string   `json:"start_date"`
		EndDate   string   `json:"end_date"`
		Sections  []string `json:"sections"`
		Template  string   `json:"template"`
		Format    string   `json:"format"`
	}{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		Template:  template,
		Format:    "html",
	}

	switch template {
	case "technical":
		req.Sections = []string{"summary", "agents", "tasks", "credentials", "network", "findings", "recommendations"}
	case "executive":
		req.Sections = []string{"overview", "recommendations"}
	default:
		req.Sections = []string{"summary", "agents", "tasks", "credentials", "network"}
	}

	report := s.buildReportData(req.StartDate, req.EndDate, req.Sections)
	html := generateHTMLReport(report)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

func (s *Server) handleAPIDeleteReport(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid report id")
		return
	}
	if err := s.db.Delete(&db.GeneratedReport{}, id).Error; err != nil {
		slog.Error("Failed to delete report", "id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to delete report")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// buildReportData constructs a gin.H report matching the format expected by generateHTMLReport
func (s *Server) buildReportData(startDate, endDate string, sections []string) gin.H {
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	end = end.Add(24*time.Hour - time.Second)

	report := gin.H{
		"title":       "ForgeC2 Action Report",
		"generated":   time.Now().Format("2006-01-02 15:04:05"),
		"date_range":  fmt.Sprintf("%s to %s", startDate, endDate),
		"summary":     gin.H{},
		"agents":      []gin.H{},
		"tasks":       []gin.H{},
		"credentials": []gin.H{},
		"audit":       []gin.H{},
	}

	sectionSet := make(map[string]bool, len(sections))
	for _, s := range sections {
		sectionSet[s] = true
	}

	var agentCount, taskCount, credCount, auditCount int64
	if sectionSet["agents"] || sectionSet["summary"] {
		s.db.Model(&db.Implant{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&agentCount)
	}
	if sectionSet["tasks"] || sectionSet["summary"] {
		s.db.Model(&db.Task{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&taskCount)
	}
	if sectionSet["credentials"] || sectionSet["summary"] {
		s.db.Model(&db.CredentialEntry{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&credCount)
	}
	if sectionSet["audit"] || sectionSet["summary"] {
		s.db.Model(&db.AuditLog{}).Where("created_at BETWEEN ? AND ?", start, end).Count(&auditCount)
	}

	report["summary"] = gin.H{
		"total_agents": agentCount,
		"total_tasks":  taskCount,
		"total_creds":  credCount,
		"total_audits": auditCount,
		"success_rate": fmt.Sprintf("%.1f%%", float64(taskCount)/float64(agentCount+1)*100),
	}

	if sectionSet["agents"] {
		var agents []db.Implant
		s.db.Where("created_at BETWEEN ? AND ?", start, end).Order("created_at desc").Find(&agents)
		agentList := make([]gin.H, 0, len(agents))
		for _, a := range agents {
			agentList = append(agentList, gin.H{
				"id": a.ID, "hostname": a.Hostname, "os": a.OS, "ip": a.IP,
				"user": a.Username, "status": a.Status, "created": a.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["agents"] = agentList
	}

	if sectionSet["tasks"] {
		var tasks []db.Task
		s.db.Where("created_at BETWEEN ? AND ?", start, end).Order("created_at desc").Limit(100).Find(&tasks)
		taskList := make([]gin.H, 0, len(tasks))
		for _, t := range tasks {
			taskList = append(taskList, gin.H{
				"id": t.ID, "agent": t.AgentID, "type": t.Type,
				"command": t.Command, "status": t.Status, "created": t.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["tasks"] = taskList
	}

	if sectionSet["credentials"] {
		var creds []db.CredentialEntry
		s.db.Where("created_at BETWEEN ? AND ?", start, end).Order("created_at desc").Limit(100).Find(&creds)
		credList := make([]gin.H, 0, len(creds))
		for _, c := range creds {
			credList = append(credList, gin.H{
				"id": c.ID, "agent": c.AgentID, "type": c.Type,
				"username": c.Username, "source": c.Source, "created": c.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["credentials"] = credList
	}

	if sectionSet["audit"] {
		var audits []db.AuditLog
		s.db.Where("created_at BETWEEN ? AND ?", start, end).Order("created_at desc").Limit(100).Find(&audits)
		auditList := make([]gin.H, 0, len(audits))
		for _, a := range audits {
			auditList = append(auditList, gin.H{
				"id": a.ID, "user": a.User, "action": a.Action,
				"details": a.Details, "success": a.Success, "created": a.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		report["audit"] = auditList
	}

	return report
}
