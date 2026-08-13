package server

import (
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleActiveMissions lists all tasks currently in flight (pending or
// running) across every agent, enriched with agent hostname/IP/OS so the
// dashboard can render a live "mission board". Results are ordered oldest
// first so operators see the longest-running work on top.
func (s *Server) handleActiveMissions(c *gin.Context) {
	// Live board: never cache, the frontend refreshes via WS events.
	c.Header("Cache-Control", "no-store")
	var tasks []db.Task
	if err := s.db.WithContext(c.Request.Context()).
		Where("status IN ?", []string{"pending", "running"}).
		Preload("Agent").
		Order("created_at asc").
		Limit(50).
		Find(&tasks).Error; err != nil {
		handleQueryError(c, err, "Failed to query active missions")
		return
	}

	type mission struct {
		ID          uint      `json:"id"`
		AgentID     string    `json:"agent_id"`
		Hostname    string    `json:"hostname"`
		IP          string    `json:"ip"`
		OS          string    `json:"os"`
		Type        string    `json:"type"`
		Command     string    `json:"command"`
		Status      string    `json:"status"`
		Priority    int       `json:"priority"`
		CreatedBy   string    `json:"created_by"`
		CreatedAt   time.Time `json:"created_at"`
		ClaimedBy   string    `json:"claimed_by"`
		Progress    int       `json:"progress,omitempty"`
		TotalBytes  int64     `json:"total_bytes,omitempty"`
		Transferred int64     `json:"transferred,omitempty"`
	}

	out := make([]mission, 0, len(tasks))
	for _, t := range tasks {
		m := mission{
			ID:          t.ID,
			AgentID:     t.AgentID,
			Type:        t.Type,
			Command:     t.Command,
			Status:      t.Status,
			Priority:    t.Priority,
			CreatedBy:   t.CreatedBy,
			CreatedAt:   t.CreatedAt,
			ClaimedBy:   t.ClaimedBy,
			Progress:    t.Progress,
			TotalBytes:  t.TotalBytes,
			Transferred: t.Transferred,
		}
		if t.Agent.ID != "" {
			m.Hostname = t.Agent.Hostname
			m.IP = t.Agent.IP
			m.OS = t.Agent.OS
		}
		out = append(out, m)
	}
	respond(c, gin.H{"missions": out})
}