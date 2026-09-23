package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/util"
	"github.com/gin-gonic/gin"
)

// handleCollabAgents returns the list of agents along with their current
// collaboration lock owner (if any). Used by the agents page to show locks.
func (s *Server) handleCollabAgents(c *gin.Context) {
	// Tenant gate: lock presence reveals which agents exist and who views
	// them — restrict both sides to caller-visible agents.
	var agents []db.Implant
	if err := s.tenantScope(s.db, c).Select("id").Order("last_seen desc").Limit(5000).Find(&agents).Error; err != nil {
		slog.Error("Failed to query collab agents", "err", err)
	}
	visible := make([]string, 0, len(agents))
	for _, a := range agents {
		visible = append(visible, a.ID)
	}

	var locks []db.AgentLock
	lockQ := s.db.Limit(1000)
	if len(visible) > 0 {
		lockQ = lockQ.Where("agent_id IN ?", visible)
	} else {
		lockQ = lockQ.Where("1 = 0")
	}
	if err := lockQ.Find(&locks).Error; err != nil {
		slog.Error("Failed to query collab locks", "err", err)
	}
	lockMap := make(map[string]string, len(locks))
	now := time.Now()
	for _, l := range locks {
		// Expired locks read as absent everywhere (see lockHolderOf).
		if holder, live := lockHolderOf(&l, now); live {
			lockMap[l.AgentID] = holder
		}
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

// agentLockTTL bounds a collaboration lock: the holder's UI heartbeats via
// repeated lock calls; a crashed browser stops refreshing and the lock
// lapses instead of pinning the agent forever.
const agentLockTTL = 15 * time.Minute

// isAdmin reports the caller's admin role without writing a response (for
// force-flag checks that produce their own status codes).
func (s *Server) isAdmin(c *gin.Context) bool {
	role, _ := c.Get("user_role")
	return role == "admin"
}

// lockHolderOf returns the live holder of an agent lock ("", false when
// absent or expired). Expired rows read as absent; unlock paths delete them.
func lockHolderOf(l *db.AgentLock, now time.Time) (string, bool) {
	if l == nil || l.AgentID == "" {
		return "", false
	}
	if !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt) {
		return "", false
	}
	if l.LockedBy == "" {
		return "", false
	}
	return l.LockedBy, true
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
	// Tenant gate: locks are per-agent operator presence — no foreign agents.
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	now := time.Now()
	expiry := now.Add(agentLockTTL)
	force := c.Query("force") == "true"

	var existing db.AgentLock
	haveRow := s.db.Where("agent_id = ?", id).First(&existing).Error == nil
	if haveRow {
		if holder, live := lockHolderOf(&existing, now); live && holder != username {
			// Held by someone else: only an explicit admin force takes over.
			if !(force && s.isAdmin(c)) {
				respondError(c, http.StatusConflict, "agent is locked by "+holder)
				return
			}
			s.LogAuditRecord(c, "collab_lock_force", "agent", id, "lock taken from "+holder+" by admin "+username, true, nil)
		}
		// Atomic conditional write: only the holder (or an expired row, or a
		// forced admin) may take it. A concurrent takeover loses this race
		// instead of silently overwriting.
		res := s.db.Model(&db.AgentLock{}).
			Where("agent_id = ? AND (locked_by = ? OR expires_at IS NULL OR expires_at <= ?)", id, username, now).
			Updates(map[string]interface{}{"locked_by": username, "locked_at": now, "expires_at": expiry})
		if res.Error != nil {
			respondError(c, http.StatusInternalServerError, "failed to update collaboration lock")
			return
		}
		if res.RowsAffected == 0 {
			// Lost the race (or force path changed the holder underneath):
			// re-read to report who holds it now.
			var cur db.AgentLock
			holder := "another operator"
			if s.db.Where("agent_id = ?", id).First(&cur).Error == nil {
				if h, live := lockHolderOf(&cur, time.Now()); live {
					holder = h
				}
			}
			respondError(c, http.StatusConflict, "agent is locked by "+holder)
			return
		}
	} else {
		if err := s.db.Create(&db.AgentLock{
			ID:        util.NewString(),
			AgentID:   id,
			LockedBy:  username,
			LockedAt:  now,
			ExpiresAt: expiry,
		}).Error; err != nil {
			// Lost a create race with another operator: report, don't overwrite.
			respondError(c, http.StatusConflict, "agent was just locked by another operator")
			return
		}
	}

	s.broadcastCollabEvent("agent_locked", id, username)
	respond(c, gin.H{"success": true, "expires_at": expiry.UTC().Format(time.RFC3339)})
}

// handleCollabUnlock releases a collaboration lock on an agent.
func (s *Server) handleCollabUnlock(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	// Tenant gate: see handleCollabLock.
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	username := s.currentUsername(c)
	force := c.Query("force") == "true"

	var existing db.AgentLock
	if err := s.db.Where("agent_id = ?", id).First(&existing).Error; err != nil {
		respond(c, gin.H{"success": true, "already_unlocked": true})
		return
	}
	if holder, live := lockHolderOf(&existing, time.Now()); live && holder != username {
		// Someone else's live lock: only an explicit admin force releases it.
		if !(force && s.isAdmin(c)) {
			respondError(c, http.StatusForbidden, "lock is held by "+holder)
			return
		}
		s.LogAuditRecord(c, "collab_unlock_force", "agent", id, "lock released from "+holder+" by admin "+username, true, nil)
	}
	if err := s.db.Where("agent_id = ?", id).Delete(&db.AgentLock{}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to release collaboration lock")
		return
	}
	s.broadcastCollabEvent("agent_unlocked", id, username)
	respond(c, gin.H{"success": true})
}

// handleCollabClaimTask claims a task for the current operator. Operator
// claims live in operator_claimed_by/at — never in the beacon pipeline's
// claimed_by (agent dispatch claim), which sharing them used to corrupt.
func (s *Server) handleCollabClaimTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	username := s.currentUsername(c)

	taskID, err := parseTaskIDParam(c.Param("taskId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid task id")
		return
	}
	task, ok := s.findVisibleTask(c, taskID)
	if !ok {
		return
	}
	if task.OperatorClaimedBy != "" && task.OperatorClaimedBy != username {
		respondError(c, http.StatusConflict, "task is claimed by "+task.OperatorClaimedBy)
		return
	}
	now := time.Now()
	if err := s.db.Model(&db.Task{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{"operator_claimed_by": username, "operator_claimed_at": now}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to claim task")
		return
	}
	respond(c, gin.H{"success": true})
}

// handleCollabReleaseTask releases a previously claimed task. Only the
// claiming operator (or an admin with ?force=true, audited) may release.
func (s *Server) handleCollabReleaseTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	taskID, err := parseTaskIDParam(c.Param("taskId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid task id")
		return
	}
	task, ok := s.findVisibleTask(c, taskID)
	if !ok {
		return
	}
	username := s.currentUsername(c)
	if task.OperatorClaimedBy == "" {
		respond(c, gin.H{"success": true, "already_released": true})
		return
	}
	if task.OperatorClaimedBy != username {
		if !(c.Query("force") == "true" && s.isAdmin(c)) {
			respondError(c, http.StatusForbidden, "task is claimed by "+task.OperatorClaimedBy)
			return
		}
		s.LogAuditRecord(c, "collab_claim_force_release", "task", task.AgentID,
			"operator claim on task released", true, nil)
	}
	if err := s.db.Model(&db.Task{}).Where("id = ?", taskID).
		Updates(map[string]interface{}{"operator_claimed_by": "", "operator_claimed_at": nil}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to release task")
		return
	}
	respond(c, gin.H{"success": true})
}

// batchLockedByOther returns agentID -> holder for live locks held by
// operators other than caller, for the given agents in one query. Used by
// bulk paths to skip coordinated work without N+1 lookups.
func (s *Server) batchLockedByOther(agentIDs []string, caller string) map[string]string {
	out := make(map[string]string)
	if len(agentIDs) == 0 {
		return out
	}
	var locks []db.AgentLock
	if err := s.db.Where("agent_id IN ?", agentIDs).Find(&locks).Error; err != nil {
		return out
	}
	now := time.Now()
	for i := range locks {
		if holder, live := lockHolderOf(&locks[i], now); live && holder != caller {
			out[locks[i].AgentID] = holder
		}
	}
	return out
}

// parseTaskIDParam parses a task id route parameter.
func parseTaskIDParam(raw string) (uint, error) {
	var taskID uint
	if _, err := fmt.Sscanf(raw, "%d", &taskID); err != nil {
		return 0, err
	}
	return taskID, nil
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
