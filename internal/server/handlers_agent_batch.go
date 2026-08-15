package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/pkg/protocol"
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
	if len(req.AgentIDs) > MaxBatchAgentLimit {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("too many agents (max %d)", MaxBatchAgentLimit))
		return
	}

	// Whole batch is deleted in a single transaction: any failure rolls back
	// every agent so the operation never leaves a partially-deleted state.
	deleted := 0
	var deleteAudit []auditEntry
	tx := s.db.Begin()
	for _, id := range req.AgentIDs {
		if err := s.deleteAgentRecordTx(tx, id); err != nil {
			tx.Rollback()
			slog.Error("Bulk delete: failed to delete agent, rolling back batch", "agent_id", id, "err", err)
			respondError(c, http.StatusInternalServerError, "failed to delete agents")
			return
		}
		deleted++
		deleteAudit = append(deleteAudit, auditEntry{action: "delete_agent", resource: "agent", agentID: id, details: "bulk delete", success: true})
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		slog.Error("Bulk delete: commit failed, rolling back entire batch", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to delete agents")
		return
	}
	s.LogAuditRecords(c, deleteAudit)

	user, _ := c.Get("user")
	operator := fmt.Sprintf("%v", user)
	s.broadcastBulkAgentDeleteAlert(operator, deleted)
	s.LogAuditRecord(c, "batch_delete_agents", "agent", "", fmt.Sprintf("deleted %d agents", deleted), true, nil)
	slog.Warn("Bulk agent delete", "deleted", deleted, "user", operator)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"deleted": deleted,
		"failed":  0,
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
		TaskType string   `json:"task_type"`
		File     string   `json:"file"`
		Args     string   `json:"args"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid json")
		return
	}

	if len(req.AgentIDs) == 0 {
		respondError(c, http.StatusBadRequest, "no agents selected")
		return
	}
	if len(req.AgentIDs) > MaxBatchAgentLimit {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("too many agents (max %d)", MaxBatchAgentLimit))
		return
	}

	if req.TaskType == "" {
		req.TaskType = "shell"
	}

	// Operator identity for audit attribution on every batched task.
	user, _ := c.Get("user")
	operator, _ := user.(string)

	uniqueIDs := make([]string, 0, len(req.AgentIDs))
	seen := make(map[string]struct{}, len(req.AgentIDs))
	for _, id := range req.AgentIDs {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	var existingAgents []db.Implant
	if err := s.db.Select("id").Where("id IN ?", uniqueIDs).Find(&existingAgents).Error; err != nil {
		handleQueryError(c, err, "Failed to query existing agents for batch")
		return
	}
	existingSet := make(map[string]bool, len(existingAgents))
	for _, a := range existingAgents {
		existingSet[a.ID] = true
	}

	// Build all tasks first, then batch-insert in a single DB call.
	// Apply the same gates as createTask (command length + per-agent pending
	// depth). The single-task path rejects new tasks once an agent has
	// MaxPendingTasksPerAgent outstanding; the bulk path must not silently
	// bypass that ceiling by inserting straight into the DB.
	if len(req.Command) > MaxCommandLength {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("command too long (max %d characters)", MaxCommandLength))
		return
	}

	tasks := make([]db.Task, 0, len(uniqueIDs))
	validAgentIDs := make([]string, 0, len(uniqueIDs))

	s.agentPendingTasksMu.Lock()
	for _, agentID := range uniqueIDs {
		if !existingSet[agentID] {
			continue
		}

		// Validate task type once per agent
		if !IsKnownTaskType(req.TaskType) && !protocol.ValidTaskType(req.TaskType) {
			slog.Error("Batch command: unknown task type", "type", req.TaskType)
			continue
		}

		if s.agentPendingTasks[agentID] >= MaxPendingTasksPerAgent {
			slog.Warn("Batch command: agent pending queue full, skipping",
				"agent_id", agentID, "pending", s.agentPendingTasks[agentID], "limit", MaxPendingTasksPerAgent)
			continue
		}
		s.agentPendingTasks[agentID]++

		tasks = append(tasks, db.Task{
			AgentID:  agentID,
			Type:     req.TaskType,
			Command:  req.Command,
			Shell:    req.Shell,
			Path:     req.File,
			Data:     req.Args,
			Status:   s.resolveInitialTaskStatus(req.TaskType),
			CreatedBy: operator,
		})
		validAgentIDs = append(validAgentIDs, agentID)
	}
	s.agentPendingTasksMu.Unlock()

	// Batch-insert all tasks in one DB round-trip
	if err := s.db.CreateInBatches(tasks, 100).Error; err != nil {
		s.agentPendingTasksMu.Lock()
		for i := range tasks {
			s.agentPendingTasks[tasks[i].AgentID]--
		}
		s.agentPendingTasksMu.Unlock()
		slog.Error("Batch command: failed to batch-create tasks", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create tasks")
		return
	}

	// Post-insert: fire hooks, broadcast. Pending counters were already
	// incremented under the gate check above.
	for i := range tasks {
		if s.pluginManager != nil {
			taskCopy := tasks[i]
			go s.pluginManager.ExecuteHook(s.ctx, plugin.Event{
				Type:      plugin.EventTaskCreated,
				Timestamp: time.Now(),
				AgentID:   taskCopy.AgentID,
				Payload: map[string]interface{}{
					"task_id":   taskCopy.ID,
					"task_type": taskCopy.Type,
					"command":   taskCopy.Command,
				},
			})
		}
		s.metrics.TasksTotal.Inc()
		s.broadcastTaskUpdate(tasks[i].AgentID, tasks[i])
	}

	taskCount := len(tasks)
	failedCount := len(uniqueIDs) - taskCount

	slog.Info("Batch command sent", "count", taskCount, "failed", failedCount, "type", req.TaskType, "command", req.Command)
	s.LogAuditRecord(c, "batch_command", "agent", "", fmt.Sprintf("%s to %d agents (%d failed)", req.TaskType, taskCount, failedCount), true, nil)

	s.pushBulkResult(BulkResult{
		Timestamp: time.Now(),
		Command:   req.Command,
		TaskType:  req.TaskType,
		Created:   taskCount,
		Skipped:   0,
		Failed:    failedCount,
		Operator:  operator,
	})

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"tasks_created": taskCount,
		"failed":        failedCount,
	})
}

func (s *Server) handleBulkResults(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}

	page := 1
	pageSize := 20
	if p := c.Query("page"); p != "" {
		if v, err := fmt.Sscanf(p, "%d", &page); err == nil && v > 0 {
			// use page
		}
	}
	if ps := c.Query("pageSize"); ps != "" {
		if v, err := fmt.Sscanf(ps, "%d", &pageSize); err == nil && v > 0 {
			// use pageSize
		}
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	s.bulkHistoryMu.Lock()
	results := make([]BulkResult, len(s.bulkHistory))
	copy(results, s.bulkHistory)
	s.bulkHistoryMu.Unlock()

	total := len(results)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results[start:end],
		"total":   total,
		"page":    page,
		"page_size": pageSize,
	})
}
