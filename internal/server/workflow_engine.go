package server

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

// WorkflowEngine handles conditional workflow execution with branching support.
type WorkflowEngine struct {
	server *Server
}

func NewWorkflowEngine(s *Server) *WorkflowEngine {
	return &WorkflowEngine{server: s}
}

// ExecuteWorkflow runs a workflow with full conditional branching support.
func (we *WorkflowEngine) ExecuteWorkflow(wf db.Workflow, agentIDs []string) (int, int, error) {
	var steps []db.WorkflowStep
	if err := we.server.db.Where("workflow_id = ?", wf.ID).Order("step_order").Find(&steps).Error; err != nil {
		slog.Error("Failed to query workflow steps", "workflow_id", wf.ID, "err", err)
	}

	if len(steps) == 0 {
		return 0, 0, nil
	}

	executionID := fmt.Sprintf("e%d", time.Now().UnixNano())
	now := time.Now()

	taskCount := 0
	var lastErr string
	for _, agentID := range agentIDs {
		stepResults := map[uint]string{}

		for i := 0; i < len(steps); i++ {
			step := steps[i]

			repeatCount := 0
			// Bounded repeat: RepeatCount<=0 previously meant "repeat forever"
			// whenever RepeatDelay was set — one API call flooded the agent
			// with tasks until process restart. Treat <=0 as a single run.
			maxRepeats := step.RepeatCount
			if maxRepeats < 1 {
				maxRepeats = 1
			}
			jumpBudget := len(steps) // bounded self/backward jumps (P2)
			for {
				created, jumpTarget, abort := we.executeStep(wf, step, agentID, executionID, stepResults)
				taskCount += created
				if abort {
					lastErr = fmt.Sprintf("aborted at step %d", step.StepOrder)
					i = len(steps) // break outer
					break
				}
				if jumpTarget != "" {
					nextIdx := we.findStepIndex(steps, jumpTarget)
					if nextIdx >= 0 && jumpBudget > 0 {
						// Self/backward jumps are allowed only within a small
						// budget; unbounded loops re-created tasks endlessly.
						jumpBudget--
						i = nextIdx - 1 // loop will increment
						slog.Info("Workflow jump", "workflow_id", wf.ID, "step", step.StepOrder, "target", jumpTarget)
					} else {
						if nextIdx >= 0 {
							slog.Warn("Workflow jump budget exhausted, continuing", "workflow_id", wf.ID, "step", step.StepOrder)
						}
					}
					break
				}
				// Step completed normally (no jump)
				repeatCount++
				if repeatCount >= maxRepeats {
					break
				}
				if step.RepeatDelay > 0 {
					time.Sleep(time.Duration(step.RepeatDelay) * time.Second)
				} else {
					break
				}
			}
		}
	}

	// If nothing was dispatched (e.g. no matching agents), record a summary
	// row so the run still shows up in execution history.
	if taskCount == 0 {
		status := "completed"
		if lastErr != "" {
			status = "aborted"
		}
		log := db.ExecutionLog{
			ExecutionID:  executionID,
			WorkflowID:   wf.ID,
			WorkflowName: wf.Name,
			Status:       status,
			ErrorMsg:     lastErr,
			StartedAt:    now,
			CompletedAt:  &now,
			CreatedAt:    now,
		}
		if err := we.server.db.Create(&log).Error; err != nil {
			slog.Error("Workflow: failed to write execution summary log", "error", err)
		}
	}

	return taskCount, len(agentIDs), nil
}

// executeStep runs a single workflow step for one agent.
// Returns: tasks created, jump target (empty = no jump), whether to abort workflow.
func (we *WorkflowEngine) executeStep(wf db.Workflow, step db.WorkflowStep, agentID, executionID string, stepResults map[uint]string) (int, string, bool) {
	startedAt := time.Now()
	agentHost := ""
	var implant db.Implant
	if err := we.server.db.First(&implant, "id = ?", agentID).Error; err == nil {
		agentHost = implant.Hostname
	}
	stepLog := db.ExecutionLog{
		ExecutionID:  executionID,
		WorkflowID:   wf.ID,
		WorkflowName: wf.Name,
		AgentID:      agentID,
		AgentHost:    agentHost,
		StepOrder:    step.StepOrder,
		TaskType:     step.TaskType,
		Command:      step.Command,
		Status:       "running",
		StartedAt:    startedAt,
		CreatedAt:    startedAt,
	}
	if err := we.server.db.Create(&stepLog).Error; err != nil {
		slog.Error("Workflow: failed to create step log", "execution_id", executionID, "step", step.StepOrder, "err", err)
	}

	finish := func(status, result, branch, target string) {
		stepLog.Status = status
		stepLog.Result = result
		stepLog.BranchAction = branch
		stepLog.BranchTarget = target
		now := time.Now()
		stepLog.CompletedAt = &now
		if err := we.server.db.Save(&stepLog).Error; err != nil {
			slog.Error("Workflow: failed to save step log", "step", step.StepOrder, "err", err)
		}
	}

	task, err := we.server.createTask(agentID, step.TaskType, step.Command, step.Shell, "", "", 0, 0)
	if err != nil {
		slog.Error("Workflow: failed to create task", "step", step.StepOrder, "agent", agentID, "error", err)
		finish("failed", err.Error(), "", "")
		// Dispatch failure must honor StopOnFailure like any other step
		// failure — offline agents / RoE blocks silently let the workflow
		// continue before (P2).
		if step.StopOnFailure {
			return 0, "", true
		}
		return 0, "", false
	}
	if err := we.server.db.Model(&task).Update("created_by", "workflow").Error; err != nil {
		slog.Error("Workflow: failed to mark task created_by", "task", task.ID, "err", err)
	}
	stepLog.TaskID = task.ID
	if err := we.server.db.Save(&stepLog).Error; err != nil {
		slog.Error("Workflow: failed to update step log with task", "step", step.StepOrder, "err", err)
	}

	if step.TimeoutSec > 0 {
		deadline := time.Now().Add(time.Duration(step.TimeoutSec) * time.Second)
		for time.Now().Before(deadline) {
			var t db.Task
			if we.server.db.First(&t, task.ID).Error == nil {
				if t.Status == "completed" || t.Status == "failed" {
					stepResults[step.ID] = t.Result
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
		var t db.Task
		if we.server.db.First(&t, task.ID).Error == nil && t.Status == "pending" {
			if err := we.server.db.Model(&t).Update("status", "failed").Update("error", "workflow timeout").Error; err != nil {
				slog.Error("Workflow: failed to time out task", "task", t.ID, "err", err)
			}
			stepResults[step.ID] = ""
		}
	} else {
		var t db.Task
		if we.server.db.First(&t, task.ID).Error == nil {
			stepResults[step.ID] = t.Result
		}
	}

	result := stepResults[step.ID]

	// StopOnFailure is evaluated independently of Condition: previously the
	// else-if meant setting both fields silently disabled stop-on-failure.
	if step.StopOnFailure {
		var t db.Task
		if we.server.db.First(&t, task.ID).Error == nil && t.Status == "failed" {
			finish("aborted", result, "abort", "stop_on_failure")
			slog.Info("Workflow stopped on failure", "workflow_id", wf.ID, "step", step.StepOrder)
			return 1, "", true
		}
	}
	if step.Condition != "" {
		matched := we.EvaluateCondition(step.Condition, result)
		if !matched && step.OnFailure != "" {
			if step.OnFailure == "abort" {
				finish("aborted", result, "abort", "")
				slog.Info("Workflow aborted", "workflow_id", wf.ID, "step", step.StepOrder)
				return 1, "abort", true
			}
			finish("completed", result, "jump", step.OnFailure)
			return 1, step.OnFailure, false
		} else if matched && step.OnSuccess != "" && step.OnSuccess != "continue" {
			finish("completed", result, "jump", step.OnSuccess)
			slog.Info("Workflow branch (success)", "workflow_id", wf.ID, "step", step.StepOrder, "jump_to", step.OnSuccess)
			return 1, step.OnSuccess, false
		}
	}

	finish("completed", result, "continue", "")
	return 1, "", false
}

// EvaluateCondition checks if a result matches the given condition expression.
func (we *WorkflowEngine) EvaluateCondition(condition, result string) bool {
	condition = strings.TrimSpace(condition)
	result = strings.TrimSpace(result)

	if strings.HasPrefix(condition, "contains(") && strings.HasSuffix(condition, ")") {
		expected := strings.TrimSuffix(strings.TrimPrefix(condition, "contains("), ")")
		expected = strings.Trim(expected, "'\"")
		return strings.Contains(result, expected)
	}
	if strings.HasPrefix(condition, "equals(") && strings.HasSuffix(condition, ")") {
		expected := strings.TrimSuffix(strings.TrimPrefix(condition, "equals("), ")")
		expected = strings.Trim(expected, "'\"")
		return result == expected
	}
	if condition == "not_empty" {
		return result != ""
	}
	if condition == "empty" {
		return result == ""
	}
	return true
}

// findStepIndex returns the index in steps with matching step order number, or -1.
func (we *WorkflowEngine) findStepIndex(steps []db.WorkflowStep, target string) int {
	if target == "" {
		return -1
	}
	for i, s := range steps {
		if fmt.Sprintf("%d", s.StepOrder) == target {
			return i
		}
	}
	for i, s := range steps {
		if s.ConditionExpr == target || s.Condition == target {
			return i
		}
	}
	return -1
}
