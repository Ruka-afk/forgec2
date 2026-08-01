package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/server/totp"
	"github.com/gin-gonic/gin"
)

type pendingTOTPState struct {
	secret         string
	backupRawCodes []string
	createdAt      time.Time
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
	s.pendingTOTPMu.Lock()
	if s.pendingTOTP == nil {
		s.pendingTOTP = make(map[uint]*pendingTOTPState)
	}
	s.pendingTOTP[user.ID] = &pendingTOTPState{
		secret:         encryptedSecret,
		backupRawCodes: rawCodes,
		createdAt:      time.Now(),
	}
	s.pendingTOTPMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"secret":       secret,
		"qr_url":       qrURL,
		"backup_codes": rawCodes,
	})
}

func (s *Server) handleTOTPEnable(c *gin.Context) {
	userID, _ := c.Get("user_id")
	password := c.PostForm("password")
	secret := c.PostForm("secret")
	code := c.PostForm("code")

	if password == "" {
		respondError(c, http.StatusBadRequest, "password is required")
		return
	}

	var curUser db.User
	if err := s.db.First(&curUser, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "user not found")
		return
	}

	if !middleware.CheckPassword(curUser.PasswordHash, password) {
		respondError(c, http.StatusForbidden, "incorrect password")
		return
	}

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
	pending := s.pendingTOTP[user.ID]
	delete(s.pendingTOTP, user.ID)
	s.pendingTOTPMu.Unlock()

	if pending != nil {
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
	totpCode := c.PostForm("code")

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

	if user.TOTPSecret != "" {
		decryptedSecret, err := decryptSecret(user.TOTPSecret, s.cfg.Server.JWTSecret)
		if err != nil {
			slog.Error("Failed to decrypt TOTP secret for disable", "user_id", user.ID, "err", err)
			respondError(c, http.StatusInternalServerError, "failed to verify 2FA code")
			return
		}
		if totpCode == "" || !totp.VerifyCode(decryptedSecret, totpCode) {
			respondError(c, http.StatusBadRequest, "current TOTP code is required to disable 2FA")
			return
		}
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
	if err := s.db.Model(&db.BackupCode{}).Where("user_id = ? AND used = false", userID).Count(&count).Error; err != nil {
		slog.Error("Failed to count backup codes", "err", err)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "remaining": count})
}
