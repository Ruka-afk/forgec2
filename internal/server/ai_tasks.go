package server

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

// ── Task wait / result helpers ────────────────────────────────────────────

// taskPollIntervalSeconds returns poll interval in seconds: max(agent interval, 1).
func taskPollIntervalSeconds(currentInterval int) int {
	if currentInterval < 1 {
		return 1
	}
	return currentInterval
}

// taskPollSleepDuration computes the next sleep duration respecting min poll and remaining wait budget.
func taskPollSleepDuration(intervalSec int, remaining time.Duration) time.Duration {
	sleep := time.Duration(taskPollIntervalSeconds(intervalSec)) * time.Second
	if sleep < taskPollMinInterval {
		sleep = taskPollMinInterval
	}
	if remaining > 0 && sleep > remaining {
		sleep = remaining
	}
	return sleep
}

func isTaskTerminal(status string) bool {
	return status == "completed" || status == "failed"
}

func marshalTaskResult(task db.Task, extra map[string]interface{}) string {
	result := map[string]interface{}{
		"task_id": task.ID,
		"status":  task.Status,
	}
	for k, v := range extra {
		result[k] = v
	}
	if task.Result != "" {
		result["result"] = truncateStr(task.Result, AITaskResultTruncLen)
	}
	if task.Error != "" {
		result["error"] = task.Error
	}
	b, ok := marshalJSONSafe(result)
	if !ok {
		return `{"error":"failed to marshal task result"}`
	}
	return string(b)
}

func (s *Server) waitForTaskResult(taskID uint, agentID string) string {
	deadline := time.Now().Add(taskWaitMaxDuration)

	var agent db.Implant
	intervalSec := 1
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err == nil {
		intervalSec = agent.CurrentInterval
	}

	for time.Now().Before(deadline) {
		var task db.Task
		if err := s.db.First(&task, taskID).Error; err != nil {
			return `{"error":"task not found"}`
		}

		if isTaskTerminal(task.Status) {
			return marshalTaskResult(task, nil)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		time.Sleep(taskPollSleepDuration(intervalSec, remaining))
	}

	var task db.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return `{"error":"task not found"}`
	}
	if isTaskTerminal(task.Status) {
		return marshalTaskResult(task, nil)
	}
	return marshalTaskResult(task, map[string]interface{}{
		"message": "Task still pending after wait timeout. Use get_agent_tasks to check later.",
		"waited":  taskWaitMaxDuration.Seconds(),
	})
}

func parseJSONMap(s string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		slog.Error("parseJSONMap: failed to unmarshal", "error", err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

// ── Agent resolution ──────────────────────────────────────────────────────

func (s *Server) resolveAgentID(idOrHost string) string {
	var agent db.Implant
	if err := s.db.Where("id = ? OR hostname = ?", idOrHost, idOrHost).First(&agent).Error; err != nil {
		return ""
	}
	return agent.ID
}

func (s *Server) resolveAIAgentID(reqCtx *aiReqCtx, idOrHost string) string {
	if reqCtx == nil || reqCtx.Principal.UserID == 0 {
		return s.resolveAgentID(idOrHost)
	}
	var agent db.Implant
	if err := s.db.Where("tenant_id = ? AND (id = ? OR hostname = ?)", reqCtx.Principal.TenantID, idOrHost, idOrHost).First(&agent).Error; err != nil {
		return ""
	}
	return agent.ID
}
