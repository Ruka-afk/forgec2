package server

import (
	"log/slog"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/server/totp"
	"github.com/gin-gonic/gin"
)

type pendingTOTPState struct {
	userID         uint
	secret         string
	backupRawCodes []string
}

func (s *Server) handleTOTPStatus(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "user not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"totp_enabled": user.TOTPSecret != "",
	})
}

func (s *Server) handleTOTPGenerate(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "user not found")
		return
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to generate secret")
		return
	}

	qrURL := totp.GenerateQRCodeURL(user.Username, secret)
	rawCodes := totp.GenerateBackupCodes()

	encryptedSecret, err := encryptSecret(secret, s.cfg.Server.JWTSecret)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to process secret")
		return
	}
	s.pendingTOTP = &pendingTOTPState{
		userID:         user.ID,
		secret:         encryptedSecret,
		backupRawCodes: rawCodes,
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"secret":       secret,
		"qr_url":       qrURL,
		"backup_codes": rawCodes,
	})
}

func (s *Server) handleTOTPEnable(c *gin.Context) {
	userID, _ := c.Get("user_id")
	secret := c.PostForm("secret")
	code := c.PostForm("code")

	if secret == "" || code == "" {
		respondError(c, http.StatusBadRequest, "secret and code are required")
		return
	}

	if !totp.VerifyCode(secret, code) {
		respondError(c, http.StatusBadRequest, "invalid verification code")
		return
	}

	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "user not found")
		return
	}

	encryptedSecret, err := encryptSecret(secret, s.cfg.Server.JWTSecret)
	if err != nil {
		slog.Error("Failed to encrypt TOTP secret", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to enable 2FA")
		return
	}

	if err := s.db.Model(&user).Update("totp_secret", encryptedSecret).Error; err != nil {
		slog.Error("Failed to enable TOTP", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to enable 2FA")
		return
	}

	s.pendingTOTPMu.Lock()
	pending := s.pendingTOTP
	s.pendingTOTP = nil
	s.pendingTOTPMu.Unlock()

	if pending != nil && pending.userID == user.ID {
		for _, raw := range pending.backupRawCodes {
			hash, err := totp.HashBackupCode(raw)
			if err != nil {
				slog.Error("Failed to hash backup code", "user_id", user.ID, "err", err)
				continue
			}
			bc := db.BackupCode{
				UserID:   user.ID,
				CodeHash: hash,
			}
			if err := s.db.Create(&bc).Error; err != nil {
				slog.Error("Failed to persist backup code", "user_id", user.ID, "err", err)
			}
		}
		slog.Info("Backup codes persisted for user", "user_id", user.ID, "count", len(pending.backupRawCodes))
	}

	s.LogAuditRecord(c, "2fa_enable", "auth", user.Username, "2FA enabled", true, nil)
	slog.Info("2FA enabled for user", "username", user.Username)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Two-factor authentication enabled"})
}

func (s *Server) handleTOTPDisable(c *gin.Context) {
	userID, _ := c.Get("user_id")
	password := c.PostForm("password")

	if password == "" {
		respondError(c, http.StatusBadRequest, "password is required")
		return
	}

	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "user not found")
		return
	}

	if !middleware.CheckPassword(user.PasswordHash, password) {
		respondError(c, http.StatusUnauthorized, "incorrect password")
		return
	}

	if err := s.db.Model(&user).Update("totp_secret", "").Error; err != nil {
		slog.Error("Failed to disable TOTP", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to disable 2FA")
		return
	}
	s.db.Where("user_id = ?", user.ID).Delete(&db.BackupCode{})
	s.LogAuditRecord(c, "2fa_disable", "auth", user.Username, "2FA disabled", true, nil)
	slog.Info("2FA disabled for user", "username", user.Username)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Two-factor authentication disabled"})
}

func (s *Server) handleBackupCodeCount(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var count int64
	s.db.Model(&db.BackupCode{}).Where("user_id = ? AND used = false", userID).Count(&count)
	c.JSON(http.StatusOK, gin.H{"success": true, "remaining": count})
}
