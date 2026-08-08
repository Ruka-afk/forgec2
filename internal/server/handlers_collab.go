package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleCollabAgents returns the list of agents along with their current
// collaboration lock owner (if any). Used by the agents page to show locks.
func (s *Server) handleCollabAgents(c *gin.Context) {
	var agents []db.Implant
	if err := s.db.Select("id").Order("last_seen desc").Limit(5000).Find(&agents).Error; err != nil {
		slog.Error("Failed to query collab agents", "err", err)
	}

	var locks []db.AgentLock
	if err := s.db.Limit(1000).Find(&locks).Error; err != nil {
		slog.Error("Failed to query collab locks", "err", err)
	}
	lockMap := make(map[string]string, len(locks))
	for _, l := range locks {
		lockMap[l.AgentID] = l.LockedBy
	}

	out := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		out = append(out, gin.H{
			"id":        a.ID,
			"locked_by": lockMap[a.ID],
		})
	}
	respond(c, gin.H{"agents": out})
}

// handleCollabLock places (or refreshes) a collaboration lock on an agent.
func (s *Server) handleCollabLock(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	username := s.currentUsername(c)
	if username == "" {
		respondError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var existing db.AgentLock
	if err := s.db.Where("agent_id = ?", id).First(&existing).Error; err == nil {
		existing.LockedBy = username
		existing.LockedAt = time.Now()
		if err := s.db.Save(&existing).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "failed to update collaboration lock")
			return
		}
	} else {
		if err := s.db.Create(&db.AgentLock{
			ID:       uuid.New().String(),
			AgentID:  id,
			LockedBy: username,
			LockedAt: time.Now(),
		}).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "failed to create collaboration lock")
			return
		}
	}

	s.broadcastCollabEvent("agent_locked", id, username)
	respond(c, gin.H{"success": true})
}

// handleCollabUnlock releases a collaboration lock on an agent.
func (s *Server) handleCollabUnlock(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if err := s.db.Where("agent_id = ?", id).Delete(&db.AgentLock{}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to release collaboration lock")
		return
	}
	s.broadcastCollabEvent("agent_unlocked", id, s.currentUsername(c))
	respond(c, gin.H{"success": true})
}

// handleCollabClaimTask claims a task for the current operator.
func (s *Server) handleCollabClaimTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	taskID := c.Param("taskId")
	username := s.currentUsername(c)

	var task db.Task
	if !s.findOrFail(c, &task, taskID, "task") {
		return
	}
	task.ClaimedBy = username
	task.ClaimedAt = time.Now()
	if err := s.db.Save(&task).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to claim task")
		return
	}
	respond(c, gin.H{"success": true})
}

// handleCollabReleaseTask releases a previously claimed task.
func (s *Server) handleCollabReleaseTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	taskID := c.Param("taskId")
	var task db.Task
	if !s.findOrFail(c, &task, taskID, "task") {
		return
	}
	task.ClaimedBy = ""
	if err := s.db.Save(&task).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to release task")
		return
	}
	respond(c, gin.H{"success": true})
}

// currentUsername extracts the authenticated operator username from context.
func (s *Server) currentUsername(c *gin.Context) string {
	if u, ok := c.Get("user"); ok {
		if name, ok := u.(string); ok {
			return name
		}
	}
	return ""
}

// broadcastCollabEvent pushes a collaboration event over the operator websocket bus.
func (s *Server) broadcastCollabEvent(eventType, agentID, username string) {
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":     eventType,
		"agent_id": agentID,
		"username": username,
	})
}
