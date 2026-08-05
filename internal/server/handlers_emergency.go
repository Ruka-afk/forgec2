package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleEmergencyStop(c *gin.Context) {
	password := c.PostForm("password")
	if password == "" {
		respondError(c, http.StatusBadRequest, "Password confirmation required")
		return
	}

	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "User not found")
		return
	}

	if !middleware.CheckPassword(user.PasswordHash, password) {
		respondError(c, http.StatusUnauthorized, "Invalid password")
		return
	}

	killDate := time.Now()
	groupID := c.PostForm("group_id")

	query := s.db.Model(&db.Implant{}).Where("status != ?", "offline")

	if groupID != "" {
		// Group-scoped kill: only kill agents in the specified group
		var group db.AgentGroup
		if err := s.db.Where("id = ?", groupID).First(&group).Error; err != nil {
			respondError(c, http.StatusNotFound, "Agent group not found")
			return
		}
		query = query.Joins("JOIN agent_group_assignments ON agent_group_assignments.implant_id = implants.id").
			Where("agent_group_assignments.agent_group_id = ?", groupID)
	}

	result := query.Update("kill_date", killDate)

	if err := result.Error; err != nil {
		slog.Error("Emergency stop: failed to set kill dates", "err", err)
		respondError(c, http.StatusInternalServerError, "Emergency stop failed")
		return
	}

	scope := "all online agents"
	if groupID != "" {
		scope = "group " + groupID
	}

	s.LogAuditRecord(c, "emergency_stop", "security", user.Username,
		"Emergency stop activated ("+scope+")", true, nil)

	s.broadcastSystemAlert("EMERGENCY STOP",
		"Emergency stop activated by "+user.Username+". "+scope+" will self-terminate.",
		"emergency_stop")

	slog.Warn("EMERGENCY STOP ACTIVATED",
		"user", user.Username,
		"scope", scope,
		"agents_affected", result.RowsAffected)

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Emergency stop activated",
		"scope":         scope,
		"agents_killed": result.RowsAffected,
	})
}

// --- Fleet kill-switch broadcast ---

// killSwitchState returns the cached armed flag and token (beacon hot path).
func (s *Server) killSwitchState() (bool, string) {
	s.killSwitchMu.RLock()
	defer s.killSwitchMu.RUnlock()
	return s.killSwitchArmed, s.killSwitchToken
}

// reloadKillSwitchState refreshes the in-memory cache from the singleton DB
// row. Called at server startup and after every arm/disarm.
func (s *Server) reloadKillSwitchState() {
	var ks db.KillSwitch
	if err := s.db.First(&ks, 1).Error; err != nil {
		s.killSwitchMu.Lock()
		s.killSwitchArmed = false
		s.killSwitchToken = ""
		s.killSwitchMu.Unlock()
		return
	}
	s.killSwitchMu.Lock()
	s.killSwitchArmed = ks.Armed
	s.killSwitchToken = ks.Token
	s.killSwitchMu.Unlock()
}

// setKillSwitch persists the kill-switch state and refreshes the cache.
func (s *Server) setKillSwitch(armed bool, token, operator string) {
	now := time.Now()
	var ks db.KillSwitch
	err := s.db.First(&ks, 1).Error
	if err != nil {
		ks = db.KillSwitch{ID: 1}
	}
	ks.Armed = armed
	ks.Token = token
	ks.UpdatedAt = now
	if armed {
		ks.TriggeredAt = &now
		ks.TriggeredBy = operator
		ks.DisarmedAt = nil
		ks.DisarmedBy = ""
	} else {
		ks.DisarmedAt = &now
		ks.DisarmedBy = operator
	}
	if err := s.db.Save(&ks).Error; err != nil {
		slog.Error("Kill switch: failed to persist state", "armed", armed, "err", err)
	}
	s.reloadKillSwitchState()
}

// handleKillSwitch arms or disarms the fleet kill-switch. Arming requires an
// operator password confirmation (like emergency-stop), broadcasts uninstall
// tasks to every registered implant immediately, and attaches the armed token
// to every subsequent beacon response so offline, sleeping, and newly
// registered implants self-destruct when they next check in.
func (s *Server) handleKillSwitch(c *gin.Context) {
	var req struct {
		Action   string `json:"action" binding:"required"` // "arm" | "disarm"
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid request")
		return
	}

	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "User not found")
		return
	}
	if !middleware.CheckPassword(user.PasswordHash, req.Password) {
		respondError(c, http.StatusUnauthorized, "Invalid password")
		return
	}

	switch req.Action {
	case "arm":
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			respondError(c, http.StatusInternalServerError, "Failed to generate kill-switch token")
			return
		}
		token := hex.EncodeToString(tokenBytes)
		s.setKillSwitch(true, token, user.Username)

		// Dispatch self-destruct (uninstall) tasks to the whole fleet so
		// online implants react immediately; the beacon-level token covers
		// everyone else (offline, sleeping, future registrations).
		var agents []db.Implant
		dispatched := 0
		if err := s.db.Find(&agents).Error; err == nil {
			for _, a := range agents {
				if _, err := s.createTask(a.ID, "uninstall", "", "", "", "", 0, 0); err == nil {
					dispatched++
				}
			}
		}
		s.LogAuditRecord(c, "kill_switch_arm", "security", user.Username,
			"Fleet kill-switch ARMED ("+strconv.Itoa(dispatched)+" uninstall tasks dispatched)", true, nil)
		s.LogEmergencyAction(c, "KILL SWITCH ARMED", dispatched)
		s.broadcastSystemAlert("KILL SWITCH ARMED",
			"Fleet kill-switch armed by "+user.Username+". All implants will self-destruct on next beacon.", "kill_switch")
		slog.Warn("KILL SWITCH ARMED",
			"user", user.Username,
			"uninstall_tasks_dispatched", dispatched)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Kill switch armed. All implants will self-destruct on next beacon.",
			"armed":   true,
			"tasks_dispatched": dispatched,
		})
	case "disarm":
		s.setKillSwitch(false, "", user.Username)
		s.LogAuditRecord(c, "kill_switch_disarm", "security", user.Username,
			"Fleet kill-switch disarmed", true, nil)
		s.broadcastSystemAlert("KILL SWITCH DISARMED",
			"Fleet kill-switch disarmed by "+user.Username+".", "kill_switch")
		slog.Warn("KILL SWITCH DISARMED", "user", user.Username)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Kill switch disarmed.",
			"armed":   false,
		})
	default:
		respondError(c, http.StatusBadRequest, "action must be 'arm' or 'disarm'")
	}
}

// handleKillSwitchStatus reports the current kill-switch state. The token
// itself is never exposed: it only authenticates the per-implant broadcast.
func (s *Server) handleKillSwitchStatus(c *gin.Context) {
	var ks db.KillSwitch
	status := gin.H{"success": true, "armed": false}
	if err := s.db.First(&ks, 1).Error; err == nil {
		status = gin.H{
			"success":      true,
			"armed":        ks.Armed,
			"triggered_at": ks.TriggeredAt,
			"triggered_by": ks.TriggeredBy,
			"disarmed_at":  ks.DisarmedAt,
			"disarmed_by":  ks.DisarmedBy,
		}
	}
	c.JSON(http.StatusOK, status)
}

func (s *Server) handleEmergencyStatus(c *gin.Context) {
	var onlineCount int64
	if err := s.db.Model(&db.Implant{}).Where("status != ?", "offline").Count(&onlineCount).Error; err != nil {
		slog.Error("Failed to count online agents", "err", err)
	}

	var pendingKill int64
	if err := s.db.Model(&db.Task{}).Where("type = ? AND status IN ?", "kill", []string{"pending", "running"}).Count(&pendingKill).Error; err != nil {
		slog.Error("Failed to count pending kills", "err", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"online_agents": onlineCount,
		"pending_kills": pendingKill,
	})
}
