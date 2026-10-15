package server

import (
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
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

	task := s.issueAgentTask(c, agentID, TaskSpec{Type: "process_tree"})
	if task == nil {
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

	// Scan into the model, not a bare struct: Task.AfterFind is what decrypts
	// Result/Error, and it only runs when the destination is a db.Task. A
	// struct scan handed this handler raw FC2ENC: ciphertext, which then failed
	// the envelope parse and answered "no completed process tree" for every
	// agent whose output was encrypted at rest.
	var tasks []db.Task
	err := s.db.Table("tasks").
		Select("type, result").
		Where("agent_id = ? AND type IN ? AND status = 'completed' AND result <> ''", agentID, []string{"process_tree", "ps"}).
		Order("CASE WHEN type = 'process_tree' THEN 0 ELSE 1 END, created_at desc").
		Limit(1).
		Find(&tasks).Error

	if err != nil || len(tasks) == 0 || tasks[0].Result == "" {
		respondError(c, http.StatusNotFound, "No completed process tree. Queue process_tree first.")
		return
	}

	c.JSON(http.StatusOK, processTreeEnvelope(tasks[0].Result, tasks[0].Type))
}
