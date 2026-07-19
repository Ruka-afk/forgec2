package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/server/totp"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleLoginPage(c *gin.Context) {
	// Check if already logged in
	if _, err := c.Cookie("forgec2_session"); err == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	s.renderLoginPage(c, gin.H{
		"Title": "ForgeC2 - Login",
		"Error": "",
	})
}

func (s *Server) renderLoginPage(c *gin.Context, data gin.H) {
	s.renderPageOrJSON(c, data)
}

func (s *Server) renderLoginError(c *gin.Context, errMsg, lastUsername string, rememberMe bool) {
	s.renderLoginPage(c, gin.H{
		"Title":        "ForgeC2 - Login",
		"Error":        errMsg,
		"LastUsername": lastUsername,
		"RememberMe":   rememberMe,
	})
}

func (s *Server) handleLogin(c *gin.Context) {

	username := c.PostForm("username")
	password := c.PostForm("password")
	totpCode := c.PostForm("totp_code")
	rememberMe := c.PostForm("remember_me") == "on"
	clientIP := c.ClientIP()

	slog.Info("Login attempt", "username", username, "ip", clientIP, "password_provided", password != "")

	if locked, retryAfter := s.checkLoginLockout(clientIP); locked {
		s.LogAuditRecord(c, "login_failed", "auth", username, fmt.Sprintf("Rate limit lockout (%ds)", retryAfter), false, nil)
		s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
		return
	}

	if username == "" || password == "" {
		s.renderLoginError(c, "Username and password required", username, rememberMe)
		return
	}

	var user db.User
	result := s.db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		slog.Warn("Login failed: user not found", "username", username, "ip", c.ClientIP())
		s.LogAuditRecord(c, "login_failed", "auth", username, "User not found", false, nil)
		s.renderLoginError(c, "Invalid username or password", username, false)
		return
	}

	if !user.IsActive {
		slog.Warn("Login failed: account inactive", "username", username, "ip", c.ClientIP())
		s.LogAuditRecord(c, "login_failed", "auth", username, "Account disabled", false, nil)
		s.renderLoginError(c, "Account is disabled", username, false)
		return
	}

	if user.PasswordHash == "" {
		hash, err := middleware.HashPassword(password)
		if err != nil {
			s.renderLoginError(c, "Failed to set password", username, rememberMe)
			return
		}
		if err := s.db.Model(&user).Updates(map[string]interface{}{
			"password_hash": hash,
			"last_login":    time.Now(),
			"last_ip":       c.ClientIP(),
		}).Error; err != nil {
			slog.Error("Failed to update password hash", "username", username, "err", err)
		}
		user.PasswordHash = hash
		slog.Info("Password set for user", "username", username)
	} else if !middleware.CheckPassword(user.PasswordHash, password) {
		slog.Warn("Login failed: wrong password",
			"username", username,
			"ip", c.ClientIP(),
		)
		s.LogAuditRecord(c, "login_failed", "auth", username, "Wrong password", false, nil)
		delay := user.LoginAttempts
		if delay > MaxLoginDelayIter {
			delay = MaxLoginDelayIter
		}
		time.Sleep(time.Duration(delay) * LoginBruteForceDelay)
		if err := s.db.Model(&user).UpdateColumn("login_attempts", user.LoginAttempts+1).Error; err != nil {
			slog.Error("Failed to update login attempts", "username", username, "err", err)
		}
		if locked, retryAfter := s.recordLoginFailure(clientIP, username); locked {
			s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
			return
		}
		s.renderLoginError(c, "Invalid username or password", username, rememberMe)
		return
	} else {
		if err := s.db.Model(&user).Updates(map[string]interface{}{
			"last_login":     time.Now(),
			"last_ip":        c.ClientIP(),
			"login_attempts": 0,
		}).Error; err != nil {
			slog.Error("Failed to update login success", "username", username, "err", err)
		}
	}

	if user.TOTPSecret != "" {
		if totpCode == "" {
			s.renderLoginError(c, "Two-factor authentication required", username, rememberMe)
			return
		}
		decryptedSecret, err := decryptSecret(user.TOTPSecret, s.cfg.Server.JWTSecret)
		if err != nil {
			slog.Error("Failed to decrypt TOTP secret", "username", username, "err", err)
			s.renderLoginError(c, "2FA configuration error", username, rememberMe)
			return
		}
		if !totp.VerifyCode(decryptedSecret, totpCode) {
			slog.Warn("Login failed: invalid 2FA code", "username", username, "ip", c.ClientIP())
			s.LogAuditRecord(c, "login_failed", "auth", username, "Invalid 2FA code", false, nil)
			s.renderLoginError(c, "Invalid two-factor authentication code", username, rememberMe)
			return
		}
	}

	token, err := middleware.GenerateToken(user, rememberMe, s.cfg.Server.SessionMaxAgeHours)
	if err != nil {
		s.renderLoginError(c, "Token generation error", username, rememberMe)
		return
	}

	sessionHours := s.cfg.Server.SessionMaxAgeHours
	if sessionHours < 1 {
		sessionHours = DefaultSessionHours
	}
	maxAge := sessionHours * SecondsPerHour
	if rememberMe {
		maxAge = RememberMeMaxAgeSec
	}
	middleware.SetCookieWithSameSite(c, "forgec2_session", token, maxAge, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)

	s.clearLoginLockout(clientIP)
	if err := s.db.Model(&db.User{}).Where("id = ?", user.ID).Update("force_logout_at", nil).Error; err != nil {
		slog.Error("Failed to clear force_logout_at", "user_id", user.ID, "err", err)
	}

	if s.pluginManager != nil {
		go s.pluginManager.ExecuteHook(context.Background(), plugin.Event{
			Type:      plugin.EventUserLogin,
			Timestamp: time.Now(),
			UserID:    user.ID,
			Payload: map[string]interface{}{
				"username": user.Username,
				"role":     user.Role,
				"ip":       c.ClientIP(),
			},
		})
	}

	s.LogAuditRecord(c, "login", "auth", username, "Login successful", true, nil)
	slog.Info("Login successful, session cookie set",
		"username", username,
		"role", user.Role,
		"ip", c.ClientIP(),
		"max_age", maxAge,
		"secure", middleware.CookieSecure)

	c.Redirect(http.StatusFound, "/")
}

func (s *Server) handleLogout(c *gin.Context) {
	user, exists := c.Get("user")
	username := "unknown"
	if exists {
		if u, ok := user.(string); ok {
			username = u
		}
	}
	if s.pluginManager != nil {
		uid, _ := c.Get("user_id")
		userID, _ := uid.(uint)
		go s.pluginManager.ExecuteHook(context.Background(), plugin.Event{
			Type:      plugin.EventUserLogout,
			Timestamp: time.Now(),
			UserID:    userID,
			Payload: map[string]interface{}{
				"username": username,
				"ip":       c.ClientIP(),
			},
		})
	}

	s.LogAuditRecord(c, "logout", "auth", username, "User logged out", true, nil)
	slog.Info("User logged out", "username", username, "ip", c.ClientIP())
	middleware.SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)
	c.Redirect(http.StatusFound, "/login")
}

func (s *Server) handleSettingsPage(c *gin.Context) {
	var totalAgents, onlineAgents int64
	s.db.Model(&db.Implant{}).Count(&totalAgents)
	s.db.Model(&db.Implant{}).Where("last_seen > ?", time.Now().Add(-s.offlineThreshold())).Count(&onlineAgents)

	// Database statistics
	var (
		pendingTasks   int64
		completedTasks int64
		failedTasks    int64
		totalAudits    int64
		totalCreds     int64
		totalTokens    int64
		totalSocks     int64
		totalListeners int64
	)
	s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks)
	s.db.Model(&db.Task{}).Where("status = ?", "completed").Count(&completedTasks)
	s.db.Model(&db.Task{}).Where("status = ?", "failed").Count(&failedTasks)
	s.db.Model(&db.AuditLog{}).Count(&totalAudits)
	s.db.Model(&db.CredentialEntry{}).Count(&totalCreds)
	s.db.Model(&db.TokenEntry{}).Count(&totalTokens)
	s.db.Model(&db.SocksSession{}).Count(&totalSocks)
	s.db.Model(&db.Listener{}).Count(&totalListeners)

	// Database file size
	var dbSize int64
	if fi, err := os.Stat(s.cfg.Database.Path); err == nil {
		dbSize = fi.Size()
	}

	// Runtime stats
	uptime := time.Since(s.startTime)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Mask JWT secret for display
	jwtSecret := s.cfg.Server.JWTSecret
		jwtMasked := ""
	if len(jwtSecret) > 0 {
		jwtMasked = "****"
	}

	currentUser, _ := c.Get("user")
	currentRole, _ := c.Get("user_role")
	currentUserID, _ := c.Get("user_id")

	var totalUsers int64
	s.db.Model(&db.User{}).Count(&totalUsers)

	var profileUser db.User
	profileInfo := gin.H{}
	if currentUserID != nil {
		if uid, ok := currentUserID.(uint); ok {
			if err := s.db.First(&profileUser, uid).Error; err == nil {
				profileInfo = gin.H{
					"username":      profileUser.Username,
					"role":          profileUser.Role,
					"last_activity": profileUser.LastActivity,
					"last_login":    profileUser.LastLogin,
					"created_at":    profileUser.CreatedAt,
				}
			}
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":            "ForgeC2 - Settings",
		"ActiveNav":        "settings",
		"CurrentUsername":  currentUser,
		"CurrentUserRole":  currentRole,
		"CurrentUserId":    currentUserID,
		"ProfileInfo":      profileInfo,
		"TotalUsers":       totalUsers,
		"DefaultInterval":  s.cfg.Implant.DefaultInterval,
		"DefaultJitter":    s.cfg.Implant.DefaultJitter,
		"DefaultSkipTLS":   s.cfg.Implant.DefaultSkipTLS,
		"DefaultUA":        s.cfg.Implant.DefaultUA,
		"ServerPort":       s.cfg.Server.Port,
		"ServerHost":       s.cfg.Server.Host,
		"TLSEnabled":       s.cfg.Server.TLSEnabled,
		"TCPEnabled":       s.cfg.Server.TCPEnabled,
		"TCPAddr":          s.cfg.Server.TCPAddr,
		"LogLevel":         s.cfg.Logging.Level,
		"ServerAddress":    fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port),
		"DatabasePath":     s.cfg.Database.Path,
		"DatabaseSize":     dbSize,
		"DataDir":          s.cfg.Server.DataDir,
		"JWTMasked":        jwtMasked,
		"TotalAgents":      totalAgents,
		"OnlineAgents":     onlineAgents,
		"PendingTasks":     pendingTasks,
		"CompletedTasks":   completedTasks,
		"FailedTasks":      failedTasks,
		"TotalAudits":      totalAudits,
		"TotalCredentials": totalCreds,
		"TotalTokens":      totalTokens,
		"TotalSocks":       totalSocks,
		"TotalListeners":   totalListeners,
		"Uptime":           uptime,
		"GoVersion":        runtime.Version(),
		"Goroutines":       runtime.NumGoroutine(),
		"AllocMem":         int64(m.Alloc),
		"TotalAllocMem":    int64(m.TotalAlloc),
		"NumCPU":           runtime.NumCPU(),
		"GOOS":             runtime.GOOS,
		"GOARCH":           runtime.GOARCH,
		"OfflineThreshold": s.cfg.Server.OfflineThreshold,
		"SessionMaxAge":    s.cfg.Server.SessionMaxAgeHours,
		"CleanupRetention": s.cfg.Server.CleanupRetentionDays,
		"MalleableEnabled": s.cfg.Malleable.Enabled,
		"MalleableStatus":  s.cfg.Malleable.StatusCode,
		"MalleableCT":      s.cfg.Malleable.ContentType,
		"MalleableHeaders": s.cfg.Malleable.Headers,
		"MalleablePrepend": s.cfg.Malleable.Prepend,
		"MalleableAppend":  s.cfg.Malleable.Append,
		"WorkingStart":     s.cfg.Implant.DefaultWorkingStart,
		"WorkingEnd":       s.cfg.Implant.DefaultWorkingEnd,
		"WorkingTZ":        s.cfg.Implant.DefaultWorkingTZ,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

func (s *Server) handleSaveAgentConfig(c *gin.Context) {
	interval := c.PostForm("interval")
	jitter := c.PostForm("jitter")
	userAgent := c.PostForm("user_agent")
	skipTLS := c.PostForm("skip_tls")
	workingStart := c.PostForm("working_start")
	workingEnd := c.PostForm("working_end")
	workingTZ := c.PostForm("working_tz")

	if interval != "" {
		var intInterval int
		if _, err := fmt.Sscanf(interval, "%d", &intInterval); err == nil && intInterval >= 0 {
			s.cfg.Implant.DefaultInterval = intInterval
		}
	}

	if jitter != "" {
		var intJitter int
		if _, err := fmt.Sscanf(jitter, "%d", &intJitter); err == nil && intJitter >= 0 && intJitter <= 100 {
			s.cfg.Implant.DefaultJitter = intJitter
		}
	}

	if userAgent != "" {
		s.cfg.Implant.DefaultUA = userAgent
	}

	s.cfg.Implant.DefaultSkipTLS = skipTLS == "true" || skipTLS == "1"

	s.cfg.Implant.DefaultWorkingStart = workingStart
	s.cfg.Implant.DefaultWorkingEnd = workingEnd
	s.cfg.Implant.DefaultWorkingTZ = workingTZ

	if err := s.cfg.Save(s.configPath); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to save config")
		return
	}

	slog.Info("Agent config updated", "interval", s.cfg.Implant.DefaultInterval, "jitter", s.cfg.Implant.DefaultJitter, "skip_tls", s.cfg.Implant.DefaultSkipTLS)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Agent configuration saved successfully"})
}

func (s *Server) handleChangePassword(c *gin.Context) {
	current := c.PostForm("current_password")
	newPass := c.PostForm("new_password")
	confirm := c.PostForm("confirm_password")

	if newPass != confirm || len(newPass) < 8 {
		respondError(c, http.StatusBadRequest, "Passwords do not match or too short")
		return
	}

	// Get current user from DB
	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusUnauthorized, "User not found")
		return
	}

	if !middleware.CheckPassword(user.PasswordHash, current) {
		respondError(c, http.StatusUnauthorized, "Current password incorrect")
		return
	}

	hash, err := middleware.HashPassword(newPass)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Hash failed")
		return
	}

	if err := s.db.Model(&user).Update("password_hash", hash).Error; err != nil {
		slog.Error("Failed to update password hash", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to update password")
		return
	}
	s.LogAuditRecord(c, "password_change", "auth", user.Username, "Password changed", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleTOTPStatus(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "User not found")
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
		respondError(c, http.StatusInternalServerError, "User not found")
		return
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to generate secret")
		return
	}

	qrURL := totp.GenerateQRCodeURL(user.Username, secret)
	backupCodes := totp.GenerateBackupCodes()

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"secret":       secret,
		"qr_url":       qrURL,
		"backup_codes": backupCodes,
	})
}

func (s *Server) handleTOTPEnable(c *gin.Context) {
	userID, _ := c.Get("user_id")
	secret := c.PostForm("secret")
	code := c.PostForm("code")

	if secret == "" || code == "" {
		respondError(c, http.StatusBadRequest, "Secret and code are required")
		return
	}

	if !totp.VerifyCode(secret, code) {
		respondError(c, http.StatusBadRequest, "Invalid verification code")
		return
	}

	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "User not found")
		return
	}

	encryptedSecret, err := encryptSecret(secret, s.cfg.Server.JWTSecret)
	if err != nil {
		slog.Error("Failed to encrypt TOTP secret", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to enable 2FA")
		return
	}

	if err := s.db.Model(&user).Update("totp_secret", encryptedSecret).Error; err != nil {
		slog.Error("Failed to enable TOTP", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to enable 2FA")
		return
	}
	s.LogAuditRecord(c, "2fa_enable", "auth", user.Username, "2FA enabled", true, nil)
	slog.Info("2FA enabled for user", "username", user.Username)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Two-factor authentication enabled"})
}

func (s *Server) handleTOTPDisable(c *gin.Context) {
	userID, _ := c.Get("user_id")
	password := c.PostForm("password")

	if password == "" {
		respondError(c, http.StatusBadRequest, "Password is required")
		return
	}

	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "User not found")
		return
	}

	if !middleware.CheckPassword(user.PasswordHash, password) {
		respondError(c, http.StatusUnauthorized, "Incorrect password")
		return
	}

	if err := s.db.Model(&user).Update("totp_secret", "").Error; err != nil {
		slog.Error("Failed to disable TOTP", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to disable 2FA")
		return
	}
	s.LogAuditRecord(c, "2fa_disable", "auth", user.Username, "2FA disabled", true, nil)
	slog.Info("2FA disabled for user", "username", user.Username)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Two-factor authentication disabled"})
}

func (s *Server) handleSaveServerConfig(c *gin.Context) {
	logLevel := c.PostForm("log_level")
	tcpEnabled := c.PostForm("tcp_enabled")
	tcpAddr := c.PostForm("tcp_addr")
	offlineThreshold := c.PostForm("offline_threshold")
	sessionMaxAge := c.PostForm("session_max_age")
	cleanupRetention := c.PostForm("cleanup_retention")

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if logLevel != "" && validLevels[logLevel] {
		s.cfg.Logging.Level = logLevel
	}

	if tcpEnabled != "" {
		s.cfg.Server.TCPEnabled = tcpEnabled == "true" || tcpEnabled == "1"
	}

	if tcpAddr != "" {
		s.cfg.Server.TCPAddr = tcpAddr
	}

	if offlineThreshold != "" {
		if v, err := strconv.Atoi(offlineThreshold); err == nil && v >= MinOfflineThresholdSec && v <= MaxOfflineThresholdSec {
			s.cfg.Server.OfflineThreshold = v
		}
	}

	if sessionMaxAge != "" {
		if v, err := strconv.Atoi(sessionMaxAge); err == nil && v >= MinSessionMaxAgeHours && v <= MaxSessionMaxAgeHours {
			s.cfg.Server.SessionMaxAgeHours = v
		}
	}

	if cleanupRetention != "" {
		if v, err := strconv.Atoi(cleanupRetention); err == nil && v >= MinCleanupRetentionDays && v <= MaxCleanupRetentionDays {
			s.cfg.Server.CleanupRetentionDays = v
		}
	}

	if err := s.cfg.Save(s.configPath); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to save config")
		return
	}

	slog.Info("Server config updated", "log_level", s.cfg.Logging.Level, "tcp_enabled", s.cfg.Server.TCPEnabled)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Server configuration saved"})
}

func (s *Server) handleSaveMalleableProfile(c *gin.Context) {
	s.cfg.Malleable.Enabled = c.PostForm("enabled") == "true"
	if sc := c.PostForm("status_code"); sc != "" {
		if v, err := strconv.Atoi(sc); err == nil && v >= MinHTTPStatusCode && v <= MaxHTTPStatusCode {
			s.cfg.Malleable.StatusCode = v
		}
	}
	if ct := c.PostForm("content_type"); ct != "" {
		s.cfg.Malleable.ContentType = ct
	}
	s.cfg.Malleable.Prepend = c.PostForm("prepend")
	s.cfg.Malleable.Append = c.PostForm("append")

	// Parse headers from textarea (one "Header: Value" per line)
	if headersText := c.PostForm("headers_text"); headersText != "" {
		headers := make(map[string]string)
		for _, line := range strings.Split(headersText, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if idx := strings.Index(line, ":"); idx > 0 {
				k := strings.TrimSpace(line[:idx])
				v := strings.TrimSpace(line[idx+1:])
				if k != "" {
					headers[k] = v
				}
			}
		}
		if len(headers) > 0 {
			s.cfg.Malleable.Headers = headers
		}
	}

	if err := s.cfg.Save(s.configPath); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to save config")
		return
	}

	slog.Info("Malleable profile saved", "enabled", s.cfg.Malleable.Enabled, "status_code", s.cfg.Malleable.StatusCode)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Malleable C2 profile saved"})
}

func (s *Server) handlePurgeTasks(c *gin.Context) {
	daysStr := c.PostForm("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	result := s.db.Where("created_at < ?", cutoff).
		Where("status IN ?", []string{"completed", "failed"}).
		Delete(&db.Task{})
	if result.Error != nil {
		respondError(c, http.StatusInternalServerError, "Failed to purge tasks")
		return
	}
	slog.Info("Purged old tasks", "count", result.RowsAffected, "older_than_days", days)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Purged %d old tasks", result.RowsAffected)})
}

func (s *Server) handlePurgeAuditLogs(c *gin.Context) {
	daysStr := c.PostForm("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 90
	}
	if days < 90 {
		days = 90 // enforce 90-day minimum retention for audit logs
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	result := s.db.Where("created_at < ?", cutoff).Delete(&db.AuditLog{})
	if result.Error != nil {
		respondError(c, http.StatusInternalServerError, "Failed to purge audit logs")
		return
	}
	slog.Info("Purged old audit logs", "count", result.RowsAffected, "older_than_days", days)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Purged %d old audit logs", result.RowsAffected)})
}

func (s *Server) handleRegenerateJWT(c *gin.Context) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Error("Failed to generate JWT secret", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to generate JWT secret")
		return
	}
	s.cfg.Server.JWTSecret = hex.EncodeToString(b)
	if err := s.cfg.Save(s.configPath); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to save config")
		return
	}
	middleware.InitJWTSecret(s.cfg)
	slog.Info("JWT secret regenerated")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "JWT secret regenerated successfully"})
}

func (s *Server) handleDBVacuum(c *gin.Context) {
	var dbSize int64
	rawDB, err := s.db.DB()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to get database connection")
		return
	}
	if _, err := rawDB.Exec("VACUUM"); err != nil {
		respondError(c, http.StatusInternalServerError, "VACUUM failed")
		return
	}
	// Get new size
	s.db.Raw("SELECT page_count * page_size as size FROM pragma_page_count, pragma_page_size").Scan(&dbSize)
	slog.Info("Database vacuum completed")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Database vacuum completed", "size": dbSize})
}

func (s *Server) handleBackupDatabase(c *gin.Context) {
	src := s.cfg.Database.Path
	backupDir := filepath.Join(s.cfg.Server.DataDir, "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to create backup directory")
		return
	}
	ts := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("forgec2_%s.db", ts))
	srcFile, err := os.Open(src)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to open database file")
		return
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to create backup file")
		return
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to copy database")
		return
	}
	slog.Info("Database backup created", "path", backupPath)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Backup saved: %s", backupPath)})
}

func (s *Server) handleDownloadConfig(c *gin.Context) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read config")
		return
	}
	redacted := string(data)
	// Redact all secret fields
	secrets := []string{
		s.cfg.Server.JWTSecret,
		s.cfg.Auth.PasswordHash,
		s.cfg.AI.APIKey,
		s.cfg.SSO.ClientSecret,
		s.cfg.Integrations.Slack.BotToken,
		s.cfg.Integrations.Slack.AppToken,
		s.cfg.Integrations.Slack.SigningSecret,
		s.cfg.Crypto.Key,
	}
	for _, secret := range secrets {
		if secret != "" {
			redacted = strings.ReplaceAll(redacted, secret, "****")
		}
	}
	c.Header("Content-Disposition", "attachment; filename=config.yaml")
	c.Data(http.StatusOK, "application/x-yaml", []byte(redacted))
}

func (s *Server) handleAuditLogPage(c *gin.Context) {
	stats := s.getNavStats()
	var users []db.User
	s.db.Model(&db.User{}).Select("username, role").Where("is_active = ?", true).Find(&users)

	data := gin.H{
		"Title":     "ForgeC2 - Security Audit",
		"ActiveNav": "audit",
		"Online":    s.cfg.Auth.PasswordHash != "",
		"UserList":  users,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

func (s *Server) handleGetAuditLogs(c *gin.Context) {
	var logs []db.AuditLog
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("pageSize", "50")
	search := c.DefaultQuery("search", "")
	action := c.DefaultQuery("action", "")
	user := c.DefaultQuery("user", "")

	query := s.db.Model(&db.AuditLog{}).Order("created_at DESC")

	if search != "" {
		query = query.Where("(user LIKE ? ESCAPE '\\' OR resource LIKE ? ESCAPE '\\' OR details LIKE ? ESCAPE '\\')",
			"%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%")
	}

	if action != "" {
		query = query.Where("action = ?", action)
	}

	if user != "" {
		query = query.Where("user = ?", user)
	}

	var total int64
	query.Count(&total)

	pageNum := 1
	fmt.Sscanf(page, "%d", &pageNum)
	pageSizeNum := 50
	fmt.Sscanf(pageSize, "%d", &pageSizeNum)

	offset := (pageNum - 1) * pageSizeNum
	query.Limit(pageSizeNum).Offset(offset).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   total,
	})
}



func (s *Server) handleSetLanguage(c *gin.Context) {
	lang := c.PostForm("lang")
	if lang == "" {
		lang = c.Query("lang")
	}
	if lang == "" {
		respondError(c, http.StatusBadRequest, "Language code is required")
		return
	}

	if !IsLanguageSupported(lang) {
		respondError(c, http.StatusBadRequest, "Unsupported language")
		return
	}

	middleware.SetCookieWithSameSite(c, "forgec2_lang", lang, LangCookieMaxAgeSec, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)

	referer := c.GetHeader("Referer")
	if referer != "" && strings.HasPrefix(referer, "/") && !strings.HasPrefix(referer, "//") {
		c.Redirect(http.StatusFound, referer)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"language": lang,
		"message":  "Language updated",
	})
}

func (s *Server) handleTranslationsPage(c *gin.Context) {
	s.renderPageOrJSON(c, gin.H{
		"Title": "Translation Management",
		"ActiveNav": "translations",
	})
}

func (s *Server) handleDocsPage(c *gin.Context) {
	c.Redirect(http.StatusFound, "/api/docs/")
}

func (s *Server) handleGetTranslations(c *gin.Context) {
	lang := c.Query("lang")
	if lang == "" {
		lang = detectLanguage(c)
	}

	translations, err := ExportTranslations(lang)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Settings save"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"language":     lang,
		"translations": translations,
		"count":        len(translations),
	})
}

func (s *Server) handleTranslationStats(c *gin.Context) {
	stats := GetTranslationStats()

	missing := make(map[string][]string)
	for lang := range SupportedLanguages {
		missing[lang] = GetMissingTranslations(lang)
	}

	allKeys := GetAllTranslationKeys()

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"languages":  SupportedLanguages,
		"stats":      stats,
		"total_keys": len(allKeys),
		"missing":    missing,
	})
}

func (s *Server) handleTranslationCheck(c *gin.Context) {
	lang := c.Query("lang")
	if lang == "" {
		lang = detectLanguage(c)
	}

	missing := GetMissingTranslations(lang)
	placeholderIssues := CheckPlaceholderConsistency(DefaultLanguage, lang)
	htmlIssues := CheckHTMLTags(lang)

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"language":             lang,
		"missing_translations": missing,
		"missing_count":        len(missing),
		"placeholder_issues":   placeholderIssues,
		"html_tag_issues":      htmlIssues,
	})
}

// handleGetSettingsWebhooks returns saved notification webhook targets
func (s *Server) handleGetSettingsWebhooks(c *gin.Context) {
	var cfg db.ServerConfig
	s.db.Where("key = ?", "notification_targets").First(&cfg)
	if cfg.Value == "" {
		respond(c, gin.H{"data": gin.H{"notifications": []interface{}{}}})
		return
	}
	respond(c, gin.H{"data": gin.H{"notifications": cfg.Value}})
}

// handleSaveSettingsWebhooks saves notification webhook targets
func (s *Server) handleSaveSettingsWebhooks(c *gin.Context) {
	var req struct {
		Notifications interface{} `json:"notifications"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	data, err := json.Marshal(req.Notifications)
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to marshal notifications")
		return
	}
	cfg := db.ServerConfig{Key: "notification_targets", Value: string(data), UpdatedAt: time.Now()}
	if err := s.db.Save(&cfg).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Notification config"))
		return
	}
	respond(c, gin.H{"success": true})
}

// handleTestSettingsWebhook sends a test notification to a webhook target
func (s *Server) handleTestSettingsWebhook(c *gin.Context) {
	var req struct {
		Type     string `json:"type"`
		URL      string `json:"url"`
		Secret   string `json:"secret"`
		To       string `json:"to"`
		SMTPHost string `json:"smtp_host"`
		SMTPPort int    `json:"smtp_port"`
		SMTPUser string `json:"smtp_user"`
		SMTPPass string `json:"smtp_pass"`
		From     string `json:"from"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Type == "email" {
		respond(c, gin.H{"success": true, "message": "email test not implemented in dev mode"})
		return
	}

	if req.URL == "" {
		respondError(c, http.StatusBadRequest, "webhook URL is required")
		return
	}

	payload := gin.H{"text": "ForgeC2 test notification", "content": "This is a test notification from ForgeC2."}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(req.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		respond(c, gin.H{"success": false, "error": sanitizeError(err, "Settings save")})
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		respond(c, gin.H{"success": true})
	} else {
		respond(c, gin.H{"success": false, "error": fmt.Sprintf("HTTP %d", resp.StatusCode)})
	}
}

