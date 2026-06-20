package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handlePrivescCheck handles privilege escalation reconnaissance
func (s *Server) handlePrivescCheck(c *gin.Context) {
	user := c.GetString("username")
	agentID := c.Param("id")

	var req struct {
		CheckType string `json:"check_type"` // all, windows, linux, cve_match
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		req.CheckType = c.PostForm("check_type")
		if req.CheckType == "" {
			req.CheckType = "all"
		}
	}

	// Build command
	command := fmt.Sprintf("privesc_check:%s", req.CheckType)

	// Create task
	task := db.Task{
		AgentID:   agentID,
		Type:      "privesc_check",
		Command:   command,
		Status:    "pending",
		CreatedBy: user,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create privesc check task"})
		return
	}

	// Log audit
	s.LogAuditRecord(c, "privilege_escalation_check",
		fmt.Sprintf("Privilege escalation check task %d created (type: %s)", task.ID, req.CheckType),
		agentID, fmt.Sprintf("Check type: %s", req.CheckType),
		true, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"message": fmt.Sprintf("Privilege escalation check started (type: %s)", req.CheckType),
	})
}

// handleProcessPrivescResult processes privesc check results from agent
func (s *Server) handleProcessPrivescResult(c *gin.Context) {
	var req struct {
		TaskID            uint            `json:"task_id"`
		AgentID           string          `json:"agent_id"`
		OS                string          `json:"os"`
		Vulnerabilities   []VulnInfo      `json:"vulnerabilities"`
		Misconfigurations []MisconfigInfo `json:"misconfigurations"`
		Suggestions       []string        `json:"suggestions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Format results
	result := fmt.Sprintf("=== Privilege Escalation Check Results ===\n\n")
	result += fmt.Sprintf("OS: %s\n\n", req.OS)

	result += fmt.Sprintf("Found %d potential vulnerabilities:\n", len(req.Vulnerabilities))
	for i, vuln := range req.Vulnerabilities {
		result += fmt.Sprintf("%d. [%s] %s - %s (CVE: %s)\n",
			i+1, vuln.Severity, vuln.Title, vuln.Description, vuln.CVE)
	}

	result += fmt.Sprintf("\nFound %d misconfigurations:\n", len(req.Misconfigurations))
	for i, misconfig := range req.Misconfigurations {
		result += fmt.Sprintf("%d. [%s] %s - %s\n",
			i+1, misconfig.Severity, misconfig.Title, misconfig.Description)
	}

	result += fmt.Sprintf("\nSuggestions:\n")
	for i, suggestion := range req.Suggestions {
		result += fmt.Sprintf("%d. %s\n", i+1, suggestion)
	}

	// Update task
	s.db.Model(&db.Task{}).Where("id = ?", req.TaskID).Updates(map[string]interface{}{
		"status": "completed",
		"result": result,
	})

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"vulnerabilities":   len(req.Vulnerabilities),
		"misconfigurations": len(req.Misconfigurations),
		"message":           "Privilege escalation check completed",
	})
}

// VulnInfo represents a potential privilege escalation vulnerability
type VulnInfo struct {
	CVE         string `json:"cve"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // critical, high, medium, low
	ExploitURL  string `json:"exploit_url"`
}

// MisconfigInfo represents a system misconfiguration
type MisconfigInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Remediation string `json:"remediation"`
}

// handlePrivescPage renders the privilege escalation page
func (s *Server) handlePrivescPage(c *gin.Context) {
	stats := s.getNavStats()

	// Get available agents
	var agents []db.Agent
	s.db.Where("status = 'online'").Find(&agents)

	data := gin.H{
		"Title":     "ForgeC2 - Privilege Escalation",
		"ActiveNav": "privesc",
		"Stats":     stats,
		"Agents":    agents,
	}
	s.addUserToData(c, data)

	s.renderPage(c, "privesc_content", data)
}

// handlePrivescHistory returns privesc check history
func (s *Server) handlePrivescHistory(c *gin.Context) {
	agentID := c.Param("id")

	var tasks []db.Task
	s.db.Where("agent_id = ? AND type = 'privesc_check'", agentID).
		Order("created_at desc").
		Limit(50).
		Find(&tasks)

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}
