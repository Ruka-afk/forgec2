package server

import (
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type schedulerTaskRequest struct {
	Name     string `json:"name"`
	AgentID  string `json:"agent_id"`
	TaskType string `json:"task_type"`
	Command  string `json:"command"`
	Params   string `json:"params"`
	Schedule string `json:"schedule"`
	Enabled  *bool  `json:"enabled"`
}

// handleSchedulerListTasks returns all scheduled tasks.
func (s *Server) handleSchedulerListTasks(c *gin.Context) {
	var tasks []db.ScheduledTask
	if err := s.db.Order("created_at desc").Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}
	respond(c, gin.H{"success": true, "tasks": tasks})
}

// handleSchedulerCreateTask creates a new scheduled task.
func (s *Server) handleSchedulerCreateTask(c *gin.Context) {
	var req schedulerTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" || req.AgentID == "" || req.Schedule == "" {
		respondError(c, http.StatusBadRequest, "name, agent_id, and schedule required")
		return
	}

	createdBy, _ := c.Get("username")

	task := db.ScheduledTask{
		ID:        uuid.NewString(),
		Name:      req.Name,
		Enabled:   true,
		AgentID:   req.AgentID,
		TaskType:  req.TaskType,
		Command:   req.Command,
		Params:    req.Params,
		Schedule:  req.Schedule,
		CreatedBy: toString(createdBy),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Create(&task).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create scheduled task")
		return
	}

	s.LogAuditRecord(c, "scheduler_create", "scheduled_task", task.ID, "Scheduled task created", true, nil)
	respond(c, gin.H{"success": true, "task": task})
}

// handleSchedulerUpdateTask updates an existing scheduled task.
func (s *Server) handleSchedulerUpdateTask(c *gin.Context) {
	id := c.Param("id")

	var req schedulerTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	var task db.ScheduledTask
	if !s.findOrFail(c, &task, id, "task") {
		return
	}

	updates := map[string]interface{}{
		"name":      req.Name,
		"agent_id":  req.AgentID,
		"task_type": req.TaskType,
		"command":   req.Command,
		"params":    req.Params,
		"schedule":  req.Schedule,
		"updated_at": time.Now(),
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if err := s.db.Model(&task).Updates(updates).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "update failed")
		return
	}

	s.LogAuditRecord(c, "scheduler_update", "scheduled_task", id, "Scheduled task updated", true, nil)
	respond(c, gin.H{"success": true, "message": "updated"})
}

// handleSchedulerToggleTask flips the enabled state of a scheduled task.
func (s *Server) handleSchedulerToggleTask(c *gin.Context) {
	id := c.Param("id")

	var task db.ScheduledTask
	if !s.findOrFail(c, &task, id, "task") {
		return
	}

	task.Enabled = !task.Enabled
	if err := s.db.Model(&task).Update("enabled", task.Enabled).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "toggle failed")
		return
	}

	s.LogAuditRecord(c, "scheduler_toggle", "scheduled_task", id, "Scheduled task toggled", true, nil)
	respond(c, gin.H{"success": true, "enabled": task.Enabled})
}

// handleSchedulerDeleteTask deletes a scheduled task.
func (s *Server) handleSchedulerDeleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := s.db.Delete(&db.ScheduledTask{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "delete failed")
		return
	}

	s.LogAuditRecord(c, "scheduler_delete", "scheduled_task", id, "Scheduled task deleted", true, nil)
	respond(c, gin.H{"success": true, "message": "deleted"})
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
