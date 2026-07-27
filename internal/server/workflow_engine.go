package server

import (
	"encoding/json"
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

func marshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ExecuteWorkflow runs a workflow with full conditional branching support.
func (we *WorkflowEngine) ExecuteWorkflow(wf db.Workflow, agentIDs []string) (int, int, error) {
	var steps []db.WorkflowStep
	we.server.db.Where("workflow_id = ?", wf.ID).Order("step_order").Find(&steps)

	if len(steps) == 0 {
		return 0, 0, nil
	}

	exec := db.WorkflowExecution{
		WorkflowID:   wf.ID,
		WorkflowName: wf.Name,
		AgentIDs:     marshalJSON(agentIDs),
		Status:       "running",
		StartedAt:    time.Now(),
	}
	if err := we.server.db.Create(&exec).Error; err != nil {
		slog.Error("workflow: failed to create execution record", "error", err)
	}

	taskCount := 0
	var lastErr string
	for _, agentID := range agentIDs {
		stepResults := map[uint]string{}

		for i := 0; i < len(steps); i++ {
			step := steps[i]

			repeatCount := 0
			for {
				created, jumpTarget, abort := we.executeStep(wf, step, agentID, stepResults, &exec)
				taskCount += created
				if abort {
					lastErr = fmt.Sprintf("aborted at step %d", step.StepOrder)
					i = len(steps) // break outer
					break
				}
				if jumpTarget != "" {
					nextIdx := we.findStepIndex(steps, jumpTarget)
					if nextIdx >= 0 {
						i = nextIdx - 1 // loop will increment
						slog.Info("Workflow jump", "workflow_id", wf.ID, "step", step.StepOrder, "target", jumpTarget)
					} else if jumpTarget == "abort" {
						lastErr = fmt.Sprintf("aborted at step %d", step.StepOrder)
						i = len(steps)
					}
					break
				}
				// Step completed normally (no jump)
				repeatCount++
				if step.RepeatCount > 0 && repeatCount >= step.RepeatCount {
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

	now := time.Now()
	exec.CompletedAt = &now
	exec.TasksCreated = taskCount
	exec.AgentsCount = len(agentIDs)
	if exec.Status == "running" {
		if lastErr != "" {
			exec.Status = "aborted"
			exec.ErrorMsg = lastErr
		} else {
			exec.Status = "completed"
		}
	}
	we.server.db.Save(&exec)

	return taskCount, len(agentIDs), nil
}

// executeStep runs a single workflow step for one agent.
// Returns: tasks created, jump target (empty = no jump), whether to abort workflow.
func (we *WorkflowEngine) executeStep(wf db.Workflow, step db.WorkflowStep, agentID string, stepResults map[uint]string, exec *db.WorkflowExecution) (int, string, bool) {
	startedAt := time.Now()
	stepLog := db.WorkflowStepLog{
		ExecutionID: exec.ID,
		StepOrder:   step.StepOrder,
		TaskType:    step.TaskType,
		Command:     step.Command,
		AgentID:     agentID,
		Status:      "pending",
		StartedAt:   startedAt,
	}
	we.server.db.Create(&stepLog)

	task, err := we.server.createTask(agentID, step.TaskType, step.Command, step.Shell, "", "", 0, 0)
	if err != nil {
		slog.Error("workflow: failed to create task", "step", step.StepOrder, "agent", agentID, "error", err)
		stepLog.Status = "failed"
		stepLog.Result = err.Error()
		now := time.Now()
		stepLog.CompletedAt = &now
		we.server.db.Save(&stepLog)
		return 0, "", false
	}
	we.server.db.Model(&task).Update("created_by", "workflow")
	stepLog.TaskID = task.ID
	we.server.db.Save(&stepLog)

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
			we.server.db.Model(&t).Update("status", "failed").Update("error", "workflow timeout")
			stepResults[step.ID] = ""
		}
	} else {
		var t db.Task
		if we.server.db.First(&t, task.ID).Error == nil {
			stepResults[step.ID] = t.Result
		}
	}

	result := stepResults[step.ID]
	stepLog.Result = result
	{
		now := time.Now()
		stepLog.CompletedAt = &now
	}

	if step.Condition != "" {
		matched := we.EvaluateCondition(step.Condition, result)
		if !matched && step.OnFailure != "" {
			if step.OnFailure == "abort" {
				stepLog.BranchAction = "abort"
				we.server.db.Save(&stepLog)
				slog.Info("Workflow aborted", "workflow_id", wf.ID, "step", step.StepOrder)
				return 1, "abort", true
			}
			stepLog.BranchAction = "jump"
			stepLog.BranchTarget = step.OnFailure
			we.server.db.Save(&stepLog)
			return 1, step.OnFailure, false
		} else if matched && step.OnSuccess != "" && step.OnSuccess != "continue" {
			stepLog.BranchAction = "jump"
			stepLog.BranchTarget = step.OnSuccess
			we.server.db.Save(&stepLog)
			slog.Info("Workflow branch (success)", "workflow_id", wf.ID, "step", step.StepOrder, "jump_to", step.OnSuccess)
			return 1, step.OnSuccess, false
		}
	} else if step.StopOnFailure {
		var t db.Task
		if we.server.db.First(&t, task.ID).Error == nil && t.Status == "failed" {
			stepLog.BranchAction = "abort"
			stepLog.BranchTarget = "stop_on_failure"
			we.server.db.Save(&stepLog)
			slog.Info("Workflow stopped on failure", "workflow_id", wf.ID, "step", step.StepOrder)
			return 1, "", true
		}
	}

	stepLog.BranchAction = "continue"
	we.server.db.Save(&stepLog)
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
