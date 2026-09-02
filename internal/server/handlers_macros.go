package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// MacroStep is one entry in a macro's step list.
type MacroStep struct {
	Command     string `json:"command"`
	DelayMs     int    `json:"delay_ms"`  // fixed delay before next step when not waiting for output
	Wait        bool   `json:"wait"`      // wait for this task's result before continuing
	TimeoutS    int    `json:"timeout_s"` // wait cap; 0 = default (120s)
	StopOnError bool   `json:"stop_on_error"`
}

type macroRunLogEntry struct {
	Step      int    `json:"step"`
	Command   string `json:"command"`
	Status    string `json:"status"` // ok, failed, timeout, skipped
	TaskID    uint   `json:"task_id,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

const macroDefaultWaitTimeout = 120 * time.Second
const macroMaxSteps = 50

func parseMacroSteps(raw string) ([]MacroStep, error) {
	if raw == "" {
		return nil, fmt.Errorf("steps required")
	}
	var steps []MacroStep
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return nil, fmt.Errorf("invalid steps JSON")
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("at least one step required")
	}
	if len(steps) > macroMaxSteps {
		return nil, fmt.Errorf("too many steps (max %d)", macroMaxSteps)
	}
	for i := range steps {
		steps[i].Command = strings.TrimSpace(steps[i].Command)
		if steps[i].Command == "" {
			return nil, fmt.Errorf("step %d: command required", i+1)
		}
		if steps[i].DelayMs < 0 || steps[i].DelayMs > 3600_000 {
			return nil, fmt.Errorf("step %d: invalid delay", i+1)
		}
		if steps[i].TimeoutS < 0 || steps[i].TimeoutS > 1800 {
			return nil, fmt.Errorf("step %d: invalid timeout", i+1)
		}
	}
	return steps, nil
}

func (s *Server) handleListMacros(c *gin.Context) {
	var macros []db.CommandMacro
	if err := s.db.Order("name").Find(&macros).Error; err != nil {
		slog.Error("Failed to list macros", "err", err)
		respondError(c, http.StatusInternalServerError, "query failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "macros": macros})
}

func (s *Server) handleCreateMacro(c *gin.Context) {
	var req struct {
		Name        string      `json:"name" binding:"required"`
		Description string      `json:"description"`
		Steps       []MacroStep `json:"steps" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "name and steps required")
		return
	}
	raw, err := json.Marshal(req.Steps)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid steps")
		return
	}
	if _, err := parseMacroSteps(string(raw)); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	macro := db.CommandMacro{
		Name:        req.Name,
		Description: req.Description,
		Steps:       string(raw),
		CreatedBy:   s.currentUsername(c),
	}
	if err := s.db.Create(&macro).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create macro")
		return
	}
	s.LogAuditRecord(c, "macro_create", "macro", strconv.FormatUint(uint64(macro.ID), 10), req.Name, true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "macro": macro})
}

func (s *Server) handleUpdateMacro(c *gin.Context) {
	id := c.Param("id")
	var macro db.CommandMacro
	if err := s.db.First(&macro, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "macro not found")
		return
	}
	var req struct {
		Name        string      `json:"name" binding:"required"`
		Description string      `json:"description"`
		Steps       []MacroStep `json:"steps" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "name and steps required")
		return
	}
	raw, err := json.Marshal(req.Steps)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid steps")
		return
	}
	if _, err := parseMacroSteps(string(raw)); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	updates := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"steps":       string(raw),
	}
	if err := s.db.Model(&db.CommandMacro{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update macro")
		return
	}
	s.LogAuditRecord(c, "macro_update", "macro", id, req.Name, true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleDeleteMacro(c *gin.Context) {
	id := c.Param("id")
	res := s.db.Delete(&db.CommandMacro{}, id)
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete macro")
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "macro not found")
		return
	}
	s.LogAuditRecord(c, "macro_delete", "macro", id, "", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleListMacroRuns(c *gin.Context) {
	limit := 50
	if v, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && v >= 1 && v <= 200 {
		limit = v
	}
	var runs []db.MacroRun
	if err := s.db.Order("id desc").Limit(limit).Find(&runs).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "query failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "runs": runs})
}

func (s *Server) handleGetMacroRun(c *gin.Context) {
	id := c.Param("id")
	var run db.MacroRun
	if err := s.db.First(&run, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "run not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "run": run})
}

// handleStopMacroRun marks a running macro as stopped; the runner goroutine
// polls the DB between steps and exits on sight of the stopped status.
func (s *Server) handleStopMacroRun(c *gin.Context) {
	id := c.Param("id")
	res := s.db.Model(&db.MacroRun{}).Where("id = ? AND status = ?", id, "running").
		Update("status", "stopped")
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, "failed to stop run")
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusBadRequest, "run is not running")
		return
	}
	s.broadcastOperatorEvent(map[string]interface{}{"type": "macro_update", "run_id": id, "status": "stopped"})
	s.LogAuditRecord(c, "macro_stop", "macro", id, "", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type macroRunRequest struct {
	AgentIDs    []string `json:"agent_ids" binding:"required"`
	StopOnError bool     `json:"stop_on_error"`
}

// startMacroRun creates a MacroRun for one agent and launches the background
// runner goroutine. Shared by the REST endpoint and the automation engine.
func (s *Server) startMacroRun(macro *db.CommandMacro, agentID, operator string, stopOnError bool) error {
	steps, err := parseMacroSteps(macro.Steps)
	if err != nil {
		return err
	}
	run := db.MacroRun{
		MacroID:    macro.ID,
		MacroName:  macro.Name,
		AgentID:    agentID,
		Status:     "running",
		TotalSteps: len(steps),
		Log:        "[]",
		CreatedBy:  operator,
		StartedAt:  time.Now(),
	}
	if err := s.db.Create(&run).Error; err != nil {
		return err
	}
	s.wg.Add(1)
	go func(runID uint, agentID string) {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Macro runner panicked", "run_id", runID, "panic", r)
				now := time.Now()
				if err := s.db.Model(&db.MacroRun{}).Where("id = ?", runID).Updates(map[string]interface{}{
					"status": "failed", "finished_at": &now,
				}).Error; err != nil {
					slog.Error("Failed to persist panicked macro status", "run_id", runID, "err", err)
				}
			}
		}()
		s.executeMacroRun(runID, agentID, steps, operator, stopOnError)
	}(run.ID, agentID)
	return nil
}

// handleRunMacro dispatches the macro to each requested agent.
func (s *Server) handleRunMacro(c *gin.Context) {
	id := c.Param("id")
	var macro db.CommandMacro
	if err := s.db.First(&macro, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "macro not found")
		return
	}
	var req macroRunRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AgentIDs) == 0 {
		respondError(c, http.StatusBadRequest, "agent_ids required")
		return
	}
	if len(req.AgentIDs) > 100 {
		respondError(c, http.StatusBadRequest, "too many agents (max 100)")
		return
	}
	if _, err := parseMacroSteps(macro.Steps); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Only dispatch to agents that actually exist.
	existing := map[string]bool{}
	var implants []db.Implant
	if err := s.db.Select("id").Find(&implants).Error; err != nil {
		slog.Error("Failed to resolve macro target agents", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to load agents")
		return
	}
	for _, a := range implants {
		existing[a.ID] = true
	}
	operator := s.currentUsername(c)

	created := 0
	for _, agentID := range req.AgentIDs {
		if !existing[agentID] {
			continue
		}
		macroCopy := macro
		if err := s.startMacroRun(&macroCopy, agentID, operator, req.StopOnError); err != nil {
			slog.Warn("Macro dispatch failed", "agent_id", agentID, "err", err)
			continue
		}
		created++
	}

	if created == 0 {
		respondError(c, http.StatusBadRequest, "no valid agents")
		return
	}
	s.LogAuditRecord(c, "macro_run", "macro", id,
		fmt.Sprintf("%s dispatched to %d agents", macro.Name, created), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "dispatched": created})
}

// executeMacroRun drives one agent through the step list. It re-reads the
// run row between steps so a stop request takes effect promptly.
func (s *Server) executeMacroRun(runID uint, agentID string, steps []MacroStep, operator string, globalStopOnError bool) {
	logEntries := make([]macroRunLogEntry, 0, len(steps))
	failed := false
	runCtx := s.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}

	appendLog := func(entry macroRunLogEntry) {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
		logEntries = append(logEntries, entry)
		raw, _ := json.Marshal(logEntries)
		if err := s.db.Model(&db.MacroRun{}).Where("id = ?", runID).
			Update("log", string(raw)).Error; err != nil {
			slog.Error("Failed to persist macro log", "run_id", runID, "err", err)
		}
	}

	setStatus := func(status string, step int) {
		// Conditional flip: an operator stop between steps writes "stopped";
		// this unconditional update used to overwrite it back to running/
		// failed and the deferred preservation check then kept the wrong
		// value. Only advance while still "running".
		res := s.db.Model(&db.MacroRun{}).
			Where("id = ? AND status = ?", runID, "running").
			Updates(map[string]interface{}{"status": status, "current_step": step})
		if res.Error != nil || res.RowsAffected == 0 {
			return
		}
		s.broadcastOperatorEvent(map[string]interface{}{
			"type": "macro_update", "run_id": runID, "agent_id": agentID,
			"status": status, "current_step": step, "total_steps": len(steps),
		})
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			failed = true
			slog.Error("Macro execution panicked", "run_id", runID, "agent_id", agentID, "panic", recovered)
		}
		// A stop request may have flipped the row to "stopped" between steps;
		// re-read and preserve it instead of blindly overwriting with the
		// runner's own terminal status.
		var current db.MacroRun
		final := "completed"
		if failed {
			final = "failed"
		}
		if err := s.db.Select("status").First(&current, runID).Error; err == nil && current.Status == "stopped" {
			final = "stopped"
		} else if err == nil && current.Status != "running" {
			// Some other terminal state was written concurrently; keep it.
			final = current.Status
		}
		now := time.Now()
		if err := s.db.Model(&db.MacroRun{}).Where("id = ?", runID).Updates(map[string]interface{}{
			"status": final, "finished_at": &now,
		}).Error; err != nil {
			slog.Error("Failed to persist macro terminal status", "run_id", runID, "status", final, "err", err)
			return
		}
		s.broadcastOperatorEvent(map[string]interface{}{
			"type": "macro_update", "run_id": runID, "agent_id": agentID,
			"status": final, "total_steps": len(steps),
		})
	}()

	for i, step := range steps {
		select {
		case <-runCtx.Done():
			appendLog(macroRunLogEntry{Step: i + 1, Command: step.Command, Status: "failed", Error: "server shutting down"})
			failed = true
			return
		default:
		}
		// Re-check for a stop request between steps.
		var current db.MacroRun
		if err := s.db.Select("status").First(&current, runID).Error; err != nil {
			slog.Error("Failed to load macro run status", "run_id", runID, "err", err)
			failed = true
			return
		}
		if current.Status != "running" {
			failed = false
			return
		}

		task, err := s.createTask(agentID, "shell", step.Command, "", "", "", 0, 0)
		if err != nil {
			slog.Warn("Macro step dispatch failed", "run_id", runID, "agent_id", agentID, "err", err)
			appendLog(macroRunLogEntry{Step: i + 1, Command: step.Command, Status: "failed", Error: err.Error()})
			setStatus("failed", i+1)
			failed = true
			return
		}
		s.broadcastTaskUpdate(agentID, *task)

		stepFailed := false
		if step.Wait {
			ok, output, errMsg := s.waitForMacroTask(runID, agentID, task.ID, step.TimeoutS)
			status := "ok"
			if !ok {
				status = "timeout"
				stepFailed = true
			} else if errMsg != "" {
				status = "failed"
				stepFailed = true
			}
			appendLog(macroRunLogEntry{Step: i + 1, Command: step.Command, Status: status, TaskID: task.ID, Output: truncateForLog(output), Error: errMsg})
			if runCtx.Err() != nil {
				failed = true
				return
			}
		} else {
			delay := time.Duration(step.DelayMs) * time.Millisecond
			if delay <= 0 {
				delay = 500 * time.Millisecond
			}
			select {
			case <-runCtx.Done():
				appendLog(macroRunLogEntry{Step: i + 1, Command: step.Command, Status: "failed", TaskID: task.ID, Error: "server shutting down"})
				failed = true
				return
			case <-time.After(delay):
			}
			appendLog(macroRunLogEntry{Step: i + 1, Command: step.Command, Status: "sent", TaskID: task.ID})
		}

		stop := (stepFailed && (step.StopOnError || globalStopOnError))
		if i < len(steps)-1 {
			setStatus("running", i+2)
		}
		if stop {
			setStatus("failed", i+1)
			failed = true
			return
		}
	}
}

// waitForMacroTask polls the DB until the task reaches a terminal state.
func (s *Server) waitForMacroTask(runID uint, agentID string, taskID uint, timeoutS int) (done bool, output, errMsg string) {
	timeout := macroDefaultWaitTimeout
	if timeoutS > 0 {
		timeout = time.Duration(timeoutS) * time.Second
	}
	deadline := time.Now().Add(timeout)
	runCtx := s.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	for time.Now().Before(deadline) {
		select {
		case <-runCtx.Done():
			return false, "", "server shutting down"
		case <-time.After(1 * time.Second):
		}
		// Honor a stop request mid-wait: without this the runner sat through
		// the full TimeoutS (up to 1800s) ignoring the operator's stop.
		var run db.MacroRun
		if err := s.db.Select("status").First(&run, "id = ?", runID).Error; err == nil && run.Status == "stopped" {
			return true, "", "stopped by operator"
		}
		var t db.Task
		if err := s.db.Select("status, result, error").First(&t, taskID).Error; err != nil {
			continue
		}
		switch t.Status {
		case "completed":
			return true, t.Result, ""
		case "failed":
			return true, t.Result, t.Error
		}
	}
	return false, "", "timeout waiting for output"
}

func truncateForLog(s string) string {
	const maxLen = 4096
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}
	return s
}
