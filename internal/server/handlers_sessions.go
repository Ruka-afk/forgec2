package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) createSession(token string, userID uint, ip, userAgent, deviceFingerprint string, maxAge int) error {
	hash := middleware.TokenHash(token)
	sess := db.UserSession{
		UserID:            userID,
		TokenHash:         hash,
		IP:                ip,
		UserAgent:         userAgent,
		DeviceFingerprint: deviceFingerprint,
		ExpiresAt:         time.Now().Add(time.Duration(maxAge) * time.Second),
	}
	if err := s.db.Create(&sess).Error; err != nil {
		slog.Error("Failed to create session", "user_id", userID, "err", err)
		return err
	}
	return nil
}

func (s *Server) revokeSession(token string) bool {
	hash := middleware.TokenHash(token)
	if err := s.db.Model(&db.UserSession{}).Where("token_hash = ?", hash).
		Update("revoked_at", time.Now()).Error; err != nil {
		slog.Error("Failed to revoke session", "err", err)
		return false
	}
	return true
}

func (s *Server) revokeAllUserSessions(userID uint) {
	if err := s.db.Model(&db.UserSession{}).Where("user_id = ? AND revoked_at = ?", userID, time.Time{}).
		Update("revoked_at", time.Now()).Error; err != nil {
		slog.Error("Failed to revoke all user sessions", "user_id", userID, "err", err)
	}
}

func (s *Server) isSessionRevoked(token string) bool {
	hash := middleware.TokenHash(token)
	var count int64
	if err := s.db.Model(&db.UserSession{}).
		Where("token_hash = ? AND revoked_at > ?", hash, time.Time{}).
		Count(&count).Error; err != nil {
		slog.Error("Failed to check if session is revoked", "err", err)
		return false
	}
	return count > 0
}

func (s *Server) handleListUserSessions(c *gin.Context) {
	userID := c.Param("id")
	requesterID, _ := c.MustGet("user_id").(uint)
	requesterRole, _ := c.MustGet("user_role").(string)
	if requesterRole != "admin" {
		targetID, err := strconv.ParseUint(userID, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "Invalid user ID")
			return
		}
		if requesterID != uint(targetID) {
			respondError(c, http.StatusForbidden, "Permission denied")
			return
		}
	}
	var sessions []db.UserSession
	if err := s.db.Where("user_id = ? AND revoked_at = ? AND expires_at > ?", userID, time.Time{}, time.Now()).
		Order("created_at DESC").Find(&sessions).Error; err != nil {
		slog.Error("Failed to list user sessions", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to list sessions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "sessions": sessions})
}

func (s *Server) handleRevokeSession(c *gin.Context) {
	userID := c.Param("id")
	sessionID := c.Param("sessionId")
	requesterID, _ := c.MustGet("user_id").(uint)
	requesterRole, _ := c.MustGet("user_role").(string)

	var session db.UserSession
	if err := s.db.First(&session, "id = ?", sessionID).Error; err != nil {
		respondError(c, http.StatusNotFound, "Session not found")
		return
	}

	if requesterRole != "admin" {
		targetID, err := strconv.ParseUint(userID, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "Invalid user ID")
			return
		}
		if requesterID != uint(targetID) || session.UserID != uint(targetID) {
			respondError(c, http.StatusForbidden, "Permission denied")
			return
		}
	}
	if err := s.db.Model(&db.UserSession{}).Where("id = ?", sessionID).
		Update("revoked_at", time.Now()).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Session revoked"})
}

func (s *Server) handleRevokeAllUserSessions(c *gin.Context) {
	userID := c.Param("id")
	requesterRole, _ := c.MustGet("user_role").(string)
	if requesterRole != "admin" {
		respondError(c, http.StatusForbidden, "Permission denied")
		return
	}
	result := s.db.Model(&db.UserSession{}).
		Where("user_id = ? AND revoked_at = ?", userID, time.Time{}).
		Update("revoked_at", time.Now())
	if err := result.Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to revoke sessions")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Revoked %d sessions", result.RowsAffected),
	})
}
