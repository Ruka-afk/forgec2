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
	"gorm.io/gorm"
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
// (Chrome) agent, i.e. tagged "chrome". Exact token match to avoid
// misclassifying "chromed" or "my-chromebook" tags.
func (s *Server) isChromeAgentKind(agentID string) bool {
	var agent db.Implant
	if err := s.db.Select("tags").First(&agent, "id = ?", agentID).Error; err != nil {
		return false
	}
	for _, t := range strings.Split(agent.Tags, ",") {
		if strings.TrimSpace(strings.ToLower(t)) == "chrome" {
			return true
		}
	}
	return false
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

// validateTaskCreation applies every shared gate for new tasks: task-type
// validity, chrome-agent affinity, Rules-of-Engagement, the lportfwd kill
// switch, adaptive OPSEC blocking, and the operator soft-lock. Centralizing
// these keeps the single-task and batch paths consistent so neither can
// bypass a gate.
func (s *Server) validateTaskCreation(agentID, taskType, command string, callerUserID uint) error {
	if !IsKnownTaskType(taskType) && !protocol.ValidTaskType(taskType) {
		return fmt.Errorf("unknown task type: %s", taskType)
	}

	// Chrome-extension-only task types must never be queued onto a standard Go
	// implant: they would sit "pending" until the next beacon and then be
	// rejected as unknown. Restrict them to implants tagged "chrome".
	chromeAgent := s.isChromeAgentKind(agentID)
	if chromeTaskTypes[taskType] && !chromeAgent {
		return fmt.Errorf("task type %s requires a chrome-tagged agent (browser extension)", taskType)
	}
	// The reverse: a chrome extension cannot execute implant task types
	// (shell, inject, ...). Queueing them would claim on check-in and stall.
	if chromeAgent && !chromeTaskTypes[taskType] {
		return fmt.Errorf("task type %s is not supported on chrome extension agents", taskType)
	}

	// lportfwd opens a tunneled egress path through the teamserver; honor the
	// server.lportfwd_enabled kill switch centrally so every creation path
	// (handlers, bulk, automation, scripting) inherits it.
	if err := s.checkRoE(agentID, taskType, command); err != nil {
		return err
	}

	if (taskType == protocol.TaskTypeLPortFwdStart) && !s.lportFwdAllowed() {
		slog.Warn("lportfwd task refused: disabled by configuration", "agent_id", agentID)
		return fmt.Errorf("lportfwd is disabled by server configuration (server.lportfwd_enabled)")
	}

	// Adaptive OPSEC gate: agents at critical threat level cannot launch
	// credential-access / injection operations. Enforced at creation so every
	// path (handlers, bulk, automation, workflow, scripting) inherits the gate.
	if s.opsecAdaptive != nil && s.opsecAdaptive.ShouldBlockAction(agentID, taskType) {
		slog.Warn("Task blocked by adaptive opsec (critical threat level)",
			"agent_id", agentID, "task_type", taskType)
		s.LogAuditRecord(nil, "opsec_block", "agent", agentID,
			"blocked "+taskType+" (critical threat level)", false, nil)
		return fmt.Errorf("blocked by adaptive opsec: %s is not allowed on a critical-threat host", taskType)
	}

	// Soft-lock: if another operator is actively viewing this agent, reject
	// the task to prevent conflicting concurrent commands.
	if callerUserID != 0 && s.operatorSessions != nil {
		if others := s.operatorSessions.ActiveOperatorsForAgent(agentID, callerUserID); len(others) > 0 {
			return fmt.Errorf("agent conflict: %s is being actively operated by %s", agentID, joinUsernames(others))
		}
	}

	return nil
}

// createTask creates and persists a new pending task. Returns the task or error.
func (s *Server) createTask(agentID, taskType, command, shell, path, data string, offset, size int64, opts ...TaskOption) (*db.Task, error) {
	var tOpts taskOptions
	for _, opt := range opts {
		opt(&tOpts)
	}

	if err := s.validateTaskCreation(agentID, taskType, command, tOpts.callerUserID); err != nil {
		return nil, err
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
	// lportfwd bookkeeping: register the operator-declared target so
	// connect frames can be validated against it, and clear declarations on
	// stop (P1 — closes the arbitrary-dial SSRF via agent-controlled frames).
	switch taskType {
	case protocol.TaskTypeLPortFwdStart:
		if _, target, ok := parseLPortFwdCommand(command); ok {
			s.registerLPortFwdDecl(agentID, target)
		}
	case protocol.TaskTypeLPortFwdStop:
		s.clearLPortFwdDecl(agentID)
	}
	if s.pluginManager != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// A panicking third-party hook must never take the
					// teamserver down from a task-creation path.
					slog.Error("Panic in task_created hook", "agent_id", agentID, "task_id", task.ID, "recover", r)
				}
			}()
			if err := s.pluginManager.ExecuteHook(s.ctx, plugin.Event{
				Type:      plugin.EventTaskCreated,
				Timestamp: time.Now(),
				AgentID:   agentID,
				Payload: map[string]interface{}{
					"task_id":   task.ID,
					"task_type": taskType,
					"command":   command,
				},
			}); err != nil {
				slog.Warn("Hook errors on task_created event", "agent_id", agentID, "task_id", task.ID, "err", err)
			}
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

// requeueStaleTasks retries only tasks whose delivery was never acknowledged,
// capped at 3 delivery attempts; exhausted tasks are failed instead of
// being requeued forever.
func (s *Server) requeueStaleTasks() {
	cutoff := time.Now().Add(-StaleRunningTaskTimeout)

	// Tasks already at the delivery-attempt cap: delivered repeatedly but
	// never acknowledged. Fail them so they stop cycling through the queue.
	var exhaustedTasks []db.Task
	if err := s.db.Where("status = ? AND claimed_at < ? AND acknowledged_at IS NULL AND delivery_attempts >= 3", "running", cutoff).Limit(1000).Find(&exhaustedTasks).Error; err != nil {
		slog.Error("Failed to find stale running tasks past attempt cap", "error", err)
		return
	}
	if len(exhaustedTasks) > 0 {
		exhaustedIDs := make([]uint, len(exhaustedTasks))
		for i, t := range exhaustedTasks {
			exhaustedIDs[i] = t.ID
		}
		// Status guard in the UPDATE (not just the SELECT): a result landing
		// between the two statements must not be stomped — previously a
		// just-completed task could be flipped to "failed" here.
		if err := s.db.Model(&db.Task{}).Where("id IN ? AND status = ? AND acknowledged_at IS NULL", exhaustedIDs, "running").
			Updates(map[string]interface{}{
				"status": "failed",
				"error":  "delivered but unacknowledged after 3 attempts",
			}).Error; err != nil {
			slog.Error("Failed to fail stale running tasks past attempt cap", "count", len(exhaustedIDs), "error", err)
			return
		}
		for i := range exhaustedTasks {
			t := exhaustedTasks[i]
			t.Status = "failed"
			t.Error = "delivered but unacknowledged after 3 attempts"
			s.broadcastTaskUpdate(t.AgentID, t)
		}
		slog.Info("Failed stale running tasks past delivery-attempt cap", "count", len(exhaustedIDs))
	}

	var staleTasks []db.Task
	if err := s.db.Where("status = ? AND claimed_at < ? AND acknowledged_at IS NULL AND delivery_attempts < 3", "running", cutoff).Limit(1000).Find(&staleTasks).Error; err != nil {
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
	// Status guard: without it a result arriving between SELECT and UPDATE
	// flipped a completed task back to "pending", re-delivering and
	// double-executing it on the agent.
	if err := s.db.Model(&db.Task{}).Where("id IN ? AND status = ? AND acknowledged_at IS NULL", taskIDs, "running").
		Updates(map[string]interface{}{"status": "pending", "claimed_by": "", "claimed_at": time.Time{}, "delivery_attempts": gorm.Expr("delivery_attempts + 1")}).Error; err != nil {
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
	// Status guard plus keep any partial result: the previous unconditional
	// update blanked "result" on tasks whose output arrived between SELECT
	// and UPDATE, destroying real data.
	if err := s.db.Model(&db.Task{}).Where("id IN ? AND status = ?", taskIDs, "running").
		Updates(map[string]interface{}{
			"status": "failed",
			"error":  "task acknowledged but no result received within timeout",
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
	// Include pending_approval: createTask counts approval-queued slots and
	// cancel/reject release them, so the recount must use the same semantics
	// or every pass silently shrinks counts and defeats MaxPendingTasksPerAgent.
	if err := s.db.Model(&db.Task{}).
		Select("agent_id, COUNT(*) as count").
		Where("status IN ?", []string{"pending", "running", TaskStatusPendingApproval}).
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

// trackPendingTask enforces MaxPendingTasksPerAgent and bumps the in-memory
// counter for tasks created OUTSIDE the standard createTask/dispatch path
// (the AI assistant inserts Task rows directly). Callers must decPendingTasks
// if the subsequent insert fails so the counter never drifts high.
func (s *Server) trackPendingTask(agentID string) error {
	s.agentPendingTasksMu.Lock()
	defer s.agentPendingTasksMu.Unlock()
	if n := s.agentPendingTasks[agentID]; n >= MaxPendingTasksPerAgent {
		return fmt.Errorf("agent %s has %d pending tasks (limit %d)", agentID, n, MaxPendingTasksPerAgent)
	}
	s.agentPendingTasks[agentID]++
	return nil
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
