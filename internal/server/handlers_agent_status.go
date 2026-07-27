package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAgentStatusHistory(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		respondError(c, http.StatusBadRequest, "agent id required")
		return
	}

	rangeParam := c.DefaultQuery("range", "24h")
	var startTime time.Time
	switch rangeParam {
	case "7d":
		startTime = time.Now().AddDate(0, 0, -7)
	case "30d":
		startTime = time.Now().AddDate(0, 0, -30)
	default:
		startTime = time.Now().Add(-24 * time.Hour)
	}

	var events []db.AgentStatusEvent
	s.db.Where("agent_id = ? AND timestamp >= ?", agentID, startTime).
		Order("timestamp ASC").
		Find(&events)

	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"range":    rangeParam,
		"events":   events,
	})
}

func (s *Server) recordAgentStatusEvent(agentID, status string) {
	event := db.AgentStatusEvent{
		AgentID:   agentID,
		Status:    status,
		Timestamp: time.Now(),
	}
	if err := s.db.Create(&event).Error; err != nil {
		slog.Warn("Failed to record agent status event", "agent_id", agentID, "status", status, "err", err)
	}
}
