package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// One-shot scheduled tasks: "at 02:00 run X on agent Y". Distinct from
// automation rules — single execution, operator-facing, no rule machinery.

type oneShotRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
	Command string `json:"command" binding:"required"`
	RunAt   string `json:"run_at"` // RFC3339; empty = ASAP
	Type    string `json:"type"`   // default "shell"
}

func (s *Server) handleListOneShotTasks(c *gin.Context) {
	var tasks []db.OneShotTask
	if err := s.db.Order("run_at asc").Limit(200).Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "query failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "tasks": tasks})
}

func (s *Server) handleCreateOneShotTask(c *gin.Context) {
	var req oneShotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "agent_id and command required")
		return
	}
	if _, ok := s.getAgentOrFail(c, req.AgentID); !ok {
		return
	}
	taskType := req.Type
	if taskType == "" {
		taskType = "shell"
	}
	var runAt time.Time
	if req.RunAt != "" {
		var err error
		runAt, err = time.Parse(time.RFC3339, req.RunAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "run_at must be RFC3339 (e.g. 2026-01-01T02:00:00Z)")
			return
		}
	}
	username := s.currentUsername(c)
	row := db.OneShotTask{
		AgentID:   req.AgentID,
		Type:      taskType,
		Command:   req.Command,
		Status:    "pending",
		CreatedBy: username,
	}
	if !runAt.IsZero() {
		row.RunAt = runAt
	} else {
		row.RunAt = time.Now()
	}
	if len(req.Command) > MaxCommandLength {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("command too long (max %d characters)", MaxCommandLength))
		return
	}
	if err := s.db.Create(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to schedule task")
		return
	}
	s.LogAuditRecord(c, "oneshot_schedule", "agent", req.AgentID,
		fmt.Sprintf("one-shot %s scheduled for %s: %s", taskType, row.RunAt.Format(time.RFC3339), truncateStr(req.Command, 120)), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "task": row})
}

func (s *Server) handleCancelOneShotTask(c *gin.Context) {
	id := c.Param("id")
	res := s.db.Model(&db.OneShotTask{}).Where("id = ? AND status = ?", id, "pending").Update("status", "cancelled")
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, "failed to cancel")
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "pending task not found")
		return
	}
	s.LogAuditRecord(c, "oneshot_cancel", "agent", id, "", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// dispatchDueOneShotTasks runs every scheduler tick: pending rows whose time
// has arrived are dispatched as regular shell tasks and marked done.
func (s *Server) dispatchDueOneShotTasks() {
	var due []db.OneShotTask
	if err := s.db.Where("status = ? AND run_at <= ?", "pending", time.Now()).Limit(50).Find(&due).Error; err != nil {
		slog.Error("OneShot scheduler: query failed", "err", err)
		return
	}
	for _, st := range due {
		task, err := s.createTask(st.AgentID, st.Type, st.Command, "", "", "", 0, 0)
		now := time.Now()
		if err != nil {
			slog.Warn("OneShot dispatch failed", "id", st.ID, "agent_id", st.AgentID, "err", err)
			s.db.Model(&db.OneShotTask{}).Where("id = ?", st.ID).Updates(map[string]interface{}{
				"status": "error", "finished_at": &now,
			})
			continue
		}
		s.metrics.TasksTotal.Inc()
		s.broadcastTaskUpdate(st.AgentID, *task)
		s.db.Model(&db.OneShotTask{}).Where("id = ?", st.ID).Updates(map[string]interface{}{
			"status": "done", "task_id": task.ID, "finished_at": &now,
		})
		s.LogAuditRecord(nil, "oneshot_dispatched", "agent", st.AgentID,
			fmt.Sprintf("one-shot #%d dispatched as task #%d", st.ID, task.ID), true, nil)
	}
}
