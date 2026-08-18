package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleLateralPage renders the lateral movement page
func (s *Server) handleLateralPage(c *gin.Context) {
	stats := s.getNavStats(c)

	// Get available agents
	var agents []db.Implant
	if err := s.db.Where("status = 'online'").Order("last_seen desc").Limit(5000).Find(&agents).Error; err != nil {
		slog.Error("Failed to query lateral agents", "err", err)
	}

	// Get credentials from vault
	var credentials []db.CredentialEntry
	if err := s.db.Order("created_at desc").Limit(50).Find(&credentials).Error; err != nil {
		slog.Error("Failed to query lateral credentials", "err", err)
	}

	// Get statistics
	var onlineAgents int64
	if err := s.db.Model(&db.Implant{}).Where("status = 'online'").Count(&onlineAgents).Error; err != nil {
		slog.Error("Failed to count online agents", "err", err)
	}

	var totalCreds int64
	if err := s.db.Model(&db.CredentialEntry{}).Count(&totalCreds).Error; err != nil {
		slog.Error("Failed to count credentials", "err", err)
	}

	var totalTasks int64
	if err := s.db.Model(&db.Task{}).Where("type = 'lateral'").Count(&totalTasks).Error; err != nil {
		slog.Error("Failed to count lateral tasks", "err", err)
	}

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
	q := s.db.Where("type = 'lateral'")
	if agentID != "all" {
		q = q.Where("agent_id = ?", agentID)
	}
	q.Order("created_at desc").Limit(50).Find(&tasks)

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

type lateralSpec struct {
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

// lateralAuditSummary renders a redacted summary of a lateral-movement spec
// for logs and the tamper-evident audit chain. Credential material
// (password/hash/credential/key_path/pivot) is never written to either sink;
// unparsable specs degrade to a byte count.
func lateralAuditSummary(spec string) string {
	if strings.TrimSpace(spec) == "" {
		return "lateral movement"
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(spec), &m); err != nil {
		return fmt.Sprintf("lateral movement (spec %d bytes)", len(spec))
	}
	method, _ := m["method"].(string)
	target, _ := m["target"].(string)
	user, _ := m["username"].(string)
	if method == "" {
		method = "unknown"
	}
	return fmt.Sprintf("lateral movement: method=%s target=%s username=%s", method, target, user)
}

// handleAPILateralExecute dispatches a lateral movement task via JSON API
func (s *Server) handleAPILateralExecute(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req lateralSpec
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
	slog.Info("Lateral movement via JSON API", "agent_id", req.Source, "target", req.Target, "method", req.Method)
	s.LogAuditRecord(c, "lateral", "agent", req.Source, lateralAuditSummary(string(spec)), true, nil)
	s.broadcastTaskUpdate(req.Source, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}
