package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetProcesses returns process list for tree visualization
func (s *Server) handleGetProcesses(c *gin.Context) {
	agentID := c.Param("id")

	// Request process list from agent
	task, err := s.createTask(agentID, "ps", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ps task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"message": "Process list requested. Refresh in a few seconds.",
	})
}

// handleGetProcessTree returns hierarchical process tree
func (s *Server) handleGetProcessTree(c *gin.Context) {
	agentID := c.Param("id")

	// Get the latest ps task result
	var task struct {
		Result string
	}
	err := s.db.Table("tasks").
		Select("result").
		Where("agent_id = ? AND type = 'ps' AND status = 'completed'", agentID).
		Order("created_at desc").
		Limit(1).
		Scan(&task).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No process list available. Run 'ps' command first."})
		return
	}

	// Return raw result for frontend to parse
	c.JSON(http.StatusOK, gin.H{
		"processes": task.Result,
	})
}
