package server

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

func (s *Server) createSession(token string, userID uint, ip, userAgent, deviceFingerprint string, maxAge int) {
	hash := tokenHash(token)
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
	}
}

func (s *Server) revokeSession(token string) {
	hash := tokenHash(token)
	s.db.Model(&db.UserSession{}).Where("token_hash = ?", hash).
		Update("revoked_at", time.Now())
}

func (s *Server) revokeAllUserSessions(userID uint) {
	s.db.Model(&db.UserSession{}).Where("user_id = ? AND revoked_at = zero", userID).
		Update("revoked_at", time.Now())
}

func (s *Server) isSessionRevoked(token string) bool {
	hash := tokenHash(token)
	var count int64
	s.db.Model(&db.UserSession{}).
		Where("token_hash = ? AND revoked_at > zero", hash).
		Count(&count)
	return count > 0
}

func (s *Server) handleListUserSessions(c *gin.Context) {
	userID := c.Param("id")
	var sessions []db.UserSession
	s.db.Where("user_id = ? AND revoked_at = zero AND expires_at > ?", userID, time.Now()).
		Order("created_at DESC").Find(&sessions)
	c.JSON(http.StatusOK, gin.H{"success": true, "sessions": sessions})
}

func (s *Server) handleRevokeSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if err := s.db.Model(&db.UserSession{}).Where("id = ?", sessionID).
		Update("revoked_at", time.Now()).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to revoke session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Session revoked"})
}

func (s *Server) handleRevokeAllUserSessions(c *gin.Context) {
	userID := c.Param("id")
	result := s.db.Model(&db.UserSession{}).
		Where("user_id = ? AND revoked_at = zero", userID).
		Update("revoked_at", time.Now())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Revoked %d sessions", result.RowsAffected),
	})
}
