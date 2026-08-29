package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func processTreeEnvelope(result, source string) gin.H {
	kind := "process_tree"
	live := false
	if source == "ps" {
		kind = "last_ps_snapshot"
	}
	return gin.H{
		"processes": result,
		"source":    source,
		"live":      live,
		"kind":      kind,
	}
}

// handleGetProcesses queues a process_tree task (falls back to ps if needed).
func (s *Server) handleGetProcesses(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	agentID := c.Param("id")

	task, err := s.createTask(agentID, "process_tree", "", "", "", "", 0, 0, callerOpts(c)...)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create process_tree task")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"source":  "process_tree",
		"live":    false,
		"message": "Queued process_tree. Wait for the next beacon; this is not a live stream.",
	})
}

// handleGetProcessTree returns the last completed process_tree snapshot,
// falling back to a completed ps listing when no tree has been collected yet.
func (s *Server) handleGetProcessTree(c *gin.Context) {
	agentID := c.Param("id")

	var task struct {
		Type   string
		Result string
	}
	err := s.db.Table("tasks").
		Select("type, result").
		Where("agent_id = ? AND type IN ? AND status = 'completed' AND result <> ''", agentID, []string{"process_tree", "ps"}).
		Order("CASE WHEN type = 'process_tree' THEN 0 ELSE 1 END, created_at desc").
		Limit(1).
		Scan(&task).Error

	if err != nil || task.Result == "" {
		respondError(c, http.StatusNotFound, "No completed process tree. Queue process_tree first.")
		return
	}

	c.JSON(http.StatusOK, processTreeEnvelope(task.Result, task.Type))
}
