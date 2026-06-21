package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleLateralPage renders the lateral movement page
func (s *Server) handleLateralPage(c *gin.Context) {
	stats := s.getNavStats()

	// Get available agents
	var agents []db.Implant
	s.db.Where("status = 'online'").Find(&agents)

	// Get credentials from vault
	var credentials []db.CredentialEntry
	s.db.Order("created_at desc").Limit(50).Find(&credentials)

	// Get statistics
	var onlineAgents int64
	s.db.Model(&db.Implant{}).Where("status = 'online'").Count(&onlineAgents)

	var totalCreds int64
	s.db.Model(&db.CredentialEntry{}).Count(&totalCreds)

	var totalTasks int64
	s.db.Model(&db.Task{}).Where("type = 'lateral'").Count(&totalTasks)

	data := gin.H{
		"Title":        "ForgeC2 - Lateral Movement",
		"ActiveNav":    "lateral",
		"Stats":        stats,
		"Agents":       agents,
		"Credentials":  credentials,
		"OnlineAgents": onlineAgents,
		"TotalCreds":   totalCreds,
		"TotalTasks":   totalTasks,
	}
	s.renderPage(c, "lateral_content", data)
}

// handleLateralHistory returns lateral movement history
func (s *Server) handleLateralHistory(c *gin.Context) {
	agentID := c.Param("id")

	var tasks []db.Task
	s.db.Where("agent_id = ? AND type = 'lateral'", agentID).
		Order("created_at desc").
		Limit(50).
		Find(&tasks)

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// handleProcessLateralResult processes lateral movement results from agent
func (s *Server) handleProcessLateralResult(c *gin.Context) {
	var req struct {
		TaskID  uint   `json:"task_id"`
		AgentID string `json:"agent_id"`
		Success bool   `json:"success"`
		Output  string `json:"output"`
		Error   string `json:"error"`
		Target  string `json:"target"`
		Method  string `json:"method"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Update task
	updates := map[string]interface{}{
		"status": "completed",
		"result": req.Output,
	}

	if !req.Success {
		updates["status"] = "failed"
		updates["error"] = req.Error
	}

	s.db.Model(&db.Task{}).Where("id = ?", req.TaskID).Updates(updates)

	// If successful, add target to network hosts
	if req.Success && req.Target != "" {
		host := db.NetworkHost{
			AgentID:  req.AgentID,
			IP:       req.Target,
			Hostname: "",
			OS:       "",
			Services: fmt.Sprintf(`[{"method":"%s","port":0}]`, req.Method),
			LastSeen: time.Now(),
		}
		s.db.Where("agent_id = ? AND ip = ?", req.AgentID, req.Target).FirstOrCreate(&host, db.NetworkHost{
			AgentID: req.AgentID,
			IP:      req.Target,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
