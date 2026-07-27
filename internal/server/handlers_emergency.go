package server

import (
	"log/slog"
	"net/http"
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

	result := s.db.Model(&db.Implant{}).
		Where("status != ?", "offline").
		Update("kill_date", killDate)
	affected := result.RowsAffected

	if err := result.Error; err != nil {
		slog.Error("Emergency stop: failed to set kill dates", "err", err)
		respondError(c, http.StatusInternalServerError, "Emergency stop failed")
		return
	}

	s.LogAuditRecord(c, "emergency_stop", "security", user.Username,
		"Emergency stop activated", true, nil)

	s.broadcastSystemAlert("EMERGENCY STOP",
		"Emergency stop activated by "+user.Username+". All online agents will self-terminate.",
		"emergency_stop")

	slog.Warn("EMERGENCY STOP ACTIVATED",
		"operator", user.Username,
		"agents_affected", affected)

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "Emergency stop activated",
		"agents_killed": affected,
	})
}

func (s *Server) handleEmergencyStatus(c *gin.Context) {
	var onlineCount int64
	s.db.Model(&db.Implant{}).Where("status != ?", "offline").Count(&onlineCount)

	var pendingKill int64
	s.db.Model(&db.Task{}).Where("type = ? AND status IN ?", "kill", []string{"pending", "running"}).Count(&pendingKill)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"online_agents":   onlineCount,
		"pending_kills":   pendingKill,
	})
}
