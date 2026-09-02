package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// blockedReasonSuffix appends a parenthesized reason to audit details without
// producing dangling parentheses when the operator supplied none.
func blockedReasonSuffix(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return ""
	}
	return " (" + reason + ")"
}

// maxBlockedReasonLen mirrors the column size so oversized reasons are
// truncated instead of failing the whole update.
const maxBlockedReasonLen = 255

// handleBlockAgent puts an implant out of service SERVER-SIDE. Unlike the
// kill task (which trusts the agent to exit itself), blocking is enforced by
// the teamserver: check-ins from a blocked implant are refused indefinitely —
// no tasks delivered, no results accepted — until it is explicitly unblocked.
//
// POST /api/agents/:id/block  {"reason": "..."}  (body optional)
func (s *Server) handleBlockAgent(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(strings.ToLower(err.Error()), "eof") {
		// An empty body is fine (block without comment); anything else
		// malformed is rejected.
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if len(reason) > maxBlockedReasonLen {
		reason = reason[:maxBlockedReasonLen]
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	if err := s.db.Model(&db.Implant{}).Where("id = ?", id).Updates(map[string]interface{}{
		"blocked":        true,
		"blocked_reason": reason,
		"status":         "offline",
	}).Error; err != nil {
		slog.Error("Failed to block agent", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to block agent")
		return
	}

	s.LogAuditRecord(c, "block_agent", "agent", id,
		"implant blocked server-side"+blockedReasonSuffix(reason), true, nil)
	s.eventManager.Emit(Event{
		Type:      EventImplantDisconnect,
		AgentID:   id,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"blocked": true, "reason": reason},
	})
	slog.Info("Agent blocked server-side", "agent_id", id, "reason", reason)
	respond(c, gin.H{"success": true, "agent_id": id, "blocked": true, "reason": reason})
}

// handleUnblockAgent restores service for a previously blocked implant. The
// next check-in re-registers it as online through the normal path.
//
// DELETE /api/agents/:id/block
func (s *Server) handleUnblockAgent(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	result := s.db.Model(&db.Implant{}).Where("id = ? AND blocked = ?", id, true).
		Updates(map[string]interface{}{"blocked": false, "blocked_reason": ""})
	if result.Error != nil {
		slog.Error("Failed to unblock agent", "agent_id", id, "error", result.Error)
		respondError(c, http.StatusInternalServerError, "failed to unblock agent")
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusConflict, "agent is not blocked")
		return
	}

	s.LogAuditRecord(c, "unblock_agent", "agent", id, "implant unblocked", true, nil)
	slog.Info("Agent unblocked", "agent_id", id)
	respond(c, gin.H{"success": true, "agent_id": id, "blocked": false})
}
