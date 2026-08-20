package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

// getAgentOrFail fetches agent by ID. On failure writes JSON 404 and returns false.
func (s *Server) getAgentOrFail(c *gin.Context, id string) (db.Implant, bool) {
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		slog.Error("Agent not found", "agent_id", id, "error", err)
		respondError(c, http.StatusNotFound, "agent not found")
		return agent, false
	}
	return agent, true
}

// isChromeAgentKind reports whether the target implant is a browser-extension
// (Chrome) agent, i.e. tagged "chrome". Used to keep chrome_* task types away
// from standard Go implants.
func (s *Server) isChromeAgentKind(agentID string) bool {
	var agent db.Implant
	if err := s.db.Select("tags").First(&agent, "id = ?", agentID).Error; err != nil {
		return false
	}
	return strings.Contains(agent.Tags, "chrome")
}

// TaskOption configures optional createTask behaviour.
type TaskOption func(*taskOptions)
type taskOptions struct {
	callerUserID uint
}

// WithCaller tags a createTask call with the operator user ID so the
// soft-lock check can exclude the caller from the conflict list.
func WithCaller(uid uint) TaskOption {
	return func(o *taskOptions) { o.callerUserID = uid }
}

// callerOpts extracts the user_id from gin.Context and returns a WithCaller
// option. If the context has no user_id (e.g. automation, scripting), it
// returns nil so the soft-lock check is skipped.
func callerOpts(c *gin.Context) []TaskOption {
	if c == nil {
		return nil
	}
	uid, _ := c.Get("user_id")
	u, ok := uid.(uint)
	if !ok || u == 0 {
		return nil
	}
	return []TaskOption{WithCaller(u)}
}

// createTask creates and persists a new pending task. Returns the task or error.
func (s *Server) createTask(agentID, taskType, command, shell, path, data string, offset, size int64, opts ...TaskOption) (*db.Task, error) {
	var tOpts taskOptions
	for _, opt := range opts {
		opt(&tOpts)
	}

	if !IsKnownTaskType(taskType) && !protocol.ValidTaskType(taskType) {
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}

	// Chrome-extension-only task types must never be queued onto a standard Go
	// implant: they would sit "pending" until the next beacon and then be
	// rejected as unknown. Restrict them to implants tagged "chrome".
	if chromeTaskTypes[taskType] && !s.isChromeAgentKind(agentID) {
		return nil, fmt.Errorf("task type %s requires a chrome-tagged agent (browser extension)", taskType)
	}

	// Adaptive OPSEC gate: agents at critical threat level cannot launch
	// credential-access / injection operations. Enforced at creation so every
	// path (handlers, bulk, automation, workflow, scripting) inherits the gate.
	if s.opsecAdaptive != nil && s.opsecAdaptive.ShouldBlockAction(agentID, taskType) {
		slog.Warn("Task blocked by adaptive opsec (critical threat level)",
			"agent_id", agentID, "task_type", taskType)
		s.LogAuditRecord(nil, "opsec_block", "agent", agentID,
			"blocked "+taskType+" (critical threat level)", false, nil)
		return nil, fmt.Errorf("blocked by adaptive opsec: %s is not allowed on a critical-threat host", taskType)
	}

	// Soft-lock: if another operator is actively viewing this agent, reject
	// the task to prevent conflicting concurrent commands.
	if tOpts.callerUserID != 0 && s.operatorSessions != nil {
		if others := s.operatorSessions.ActiveOperatorsForAgent(agentID, tOpts.callerUserID); len(others) > 0 {
			return nil, fmt.Errorf("agent conflict: %s is being actively operated by %s", agentID, joinUsernames(others))
		}
	}

	if len(command) > MaxCommandLength {
		return nil, fmt.Errorf("command too long (max %d characters)", MaxCommandLength)
	}

	if info, ok := getTaskTypeInfo(taskType); ok {
		for _, p := range info.Parameters {
			if p.Required {
				switch p.Name {
				case "command":
					if command == "" {
						return nil, fmt.Errorf("task type %s requires 'command' parameter", taskType)
					}
				case "shell":
					if shell == "" {
						return nil, fmt.Errorf("task type %s requires 'shell' parameter", taskType)
					}
				case "path":
					if path == "" {
						return nil, fmt.Errorf("task type %s requires 'path' parameter", taskType)
					}
				case "data":
					if data == "" {
						return nil, fmt.Errorf("task type %s requires 'data' parameter", taskType)
					}
				}
			}
		}
	}

	s.agentPendingTasksMu.Lock()
	pending := s.agentPendingTasks[agentID]
	if pending >= MaxPendingTasksPerAgent {
		s.agentPendingTasksMu.Unlock()
		return nil, fmt.Errorf("agent %s has %d pending tasks (limit %d)", agentID, pending, MaxPendingTasksPerAgent)
	}
	s.agentPendingTasks[agentID] = pending + 1
	s.agentPendingTasksMu.Unlock()

	task := db.Task{
		AgentID: agentID,
		Type:    taskType,
		Command: command,
		Shell:   shell,
		Path:    path,
		Data:    data,
		Offset:  offset,
		Size:    size,
		Status:  "pending",
	}

	task.Status = s.resolveInitialTaskStatus(taskType)
	if err := s.db.Create(&task).Error; err != nil {
		s.decPendingTasks(agentID)
		return nil, err
	}
	if s.pluginManager != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.pluginManager.ExecuteHook(s.ctx, plugin.Event{
				Type:      plugin.EventTaskCreated,
				Timestamp: time.Now(),
				AgentID:   agentID,
				Payload: map[string]interface{}{
					"task_id":   task.ID,
					"task_type": taskType,
					"command":   command,
				},
			})
		}()
	}
	s.metrics.TasksTotal.Inc()
	return &task, nil
}

// resolveInitialTaskStatus returns the status a newly created task should start
// in. When operator approval is required and the task type is flagged, the task
// waits in pending_approval; otherwise it is immediately pending. Centralizing
// this keeps the single-task and bulk/batch paths consistent so the two-man
// rule cannot be bypassed through the batch endpoint.
func (s *Server) resolveInitialTaskStatus(taskType string) string {
	if s.cfg != nil && s.cfg.Security.RequireApproval {
		if info, ok := getTaskTypeInfo(taskType); ok && info.RequiresApproval {
			return TaskStatusPendingApproval
		}
	}
	return "pending"
}

// dispatchTask logs the audit action, broadcasts the update via WS, and returns success JSON.
func (s *Server) dispatchTask(c *gin.Context, task *db.Task, auditAction, details string) {
	user, _ := c.Get("user")
	if username, ok := user.(string); ok && username != "" && task.CreatedBy == "" {
		task.CreatedBy = username
		s.db.Model(task).Update("created_by", username)
	}
	s.LogAuditRecord(c, auditAction, "agent", task.AgentID, details, true, nil)
	s.broadcastTaskUpdate(task.AgentID, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

// requeueStaleTasks retries only tasks whose delivery was never acknowledged.
func (s *Server) requeueStaleTasks() {
	cutoff := time.Now().Add(-StaleRunningTaskTimeout)
	var staleTasks []db.Task
	if err := s.db.Where("status = ? AND claimed_at < ? AND acknowledged_at IS NULL", "running", cutoff).Limit(1000).Find(&staleTasks).Error; err != nil {
		slog.Error("Failed to find stale running tasks", "error", err)
		return
	}
	if len(staleTasks) == 0 {
		return
	}
	taskIDs := make([]uint, len(staleTasks))
	for i, t := range staleTasks {
		taskIDs[i] = t.ID
	}
	if err := s.db.Model(&db.Task{}).Where("id IN ?", taskIDs).
		Updates(map[string]interface{}{"status": "pending", "claimed_by": "", "claimed_at": time.Time{}}).Error; err != nil {
		slog.Error("Failed to requeue stale running tasks", "count", len(taskIDs), "error", err)
		return
	}
	slog.Info("Requeued stale running tasks to pending", "count", len(staleTasks))
}

// failStaleAcknowledgedTasks marks tasks that were acknowledged but never produced a result.
func (s *Server) failStaleAcknowledgedTasks() {
	cutoff := time.Now().Add(-AckedTaskResultTimeout)
	var staleTasks []db.Task
	if err := s.db.Where("status = ? AND acknowledged_at IS NOT NULL AND acknowledged_at < ?", "running", cutoff).Limit(1000).Find(&staleTasks).Error; err != nil {
		slog.Error("Failed to find stale acknowledged tasks", "error", err)
		return
	}
	if len(staleTasks) == 0 {
		return
	}
	taskIDs := make([]uint, len(staleTasks))
	for i, t := range staleTasks {
		taskIDs[i] = t.ID
	}
	if err := s.db.Model(&db.Task{}).Where("id IN ?", taskIDs).
		Updates(map[string]interface{}{
			"status": "failed",
			"error":  "task acknowledged but no result received within timeout",
			"result": "",
		}).Error; err != nil {
		slog.Error("Failed to fail stale acknowledged tasks", "count", len(taskIDs), "error", err)
		return
	}
	for i := range staleTasks {
		t := staleTasks[i]
		t.Status = "failed"
		t.Error = "task acknowledged but no result received within timeout"
		t.Result = ""
		s.broadcastTaskUpdate(t.AgentID, t)
	}
	slog.Info("Failed stale acknowledged tasks with no result", "count", len(staleTasks))
}

// reconcilePendingTaskCounts recomputes the in-memory pending task counter from the DB.
func (s *Server) reconcilePendingTaskCounts() {
	var results []struct {
		AgentID string
		Count   int
	}
	if err := s.db.Model(&db.Task{}).
		Select("agent_id, COUNT(*) as count").
		Where("status IN ?", []string{"pending", "running"}).
		Group("agent_id").
		Find(&results).Error; err != nil {
		slog.Error("Failed to reconcile pending task counts", "error", err)
		return
	}
	s.agentPendingTasksMu.Lock()
	clear(s.agentPendingTasks)
	for _, r := range results {
		s.agentPendingTasks[r.AgentID] = r.Count
	}
	s.agentPendingTasksMu.Unlock()
}

// decPendingTasks decrements an agent's pending task counter and removes the
// map entry when it reaches zero, preventing unbounded map growth across the
// server lifetime (and leaked counts after an agent is purged).
func (s *Server) decPendingTasks(agentID string) {
	s.agentPendingTasksMu.Lock()
	if n := s.agentPendingTasks[agentID]; n > 0 {
		if n-1 <= 0 {
			delete(s.agentPendingTasks, agentID)
		} else {
			s.agentPendingTasks[agentID] = n - 1
		}
	}
	s.agentPendingTasksMu.Unlock()
}

// joinUsernames joins a slice of usernames with commas for error messages.
func joinUsernames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return names[0] + " and " + fmt.Sprintf("%d others", len(names)-1)
	}
}

// isConflictError returns true if the error is an agent conflict (soft-lock).
func isConflictError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "agent conflict:")
}

// respondTaskError writes the appropriate HTTP error for a createTask failure.
// Agent conflicts are surfaced as 409; everything else is 500.
func respondTaskError(c *gin.Context, err error) {
	if isConflictError(err) {
		respondError(c, http.StatusConflict, err.Error())
		return
	}
	slog.Error("Failed to create task", "error", err)
	respondError(c, http.StatusInternalServerError, "failed to create task")
}
