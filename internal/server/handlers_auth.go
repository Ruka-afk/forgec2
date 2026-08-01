package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/server/totp"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	dummyPasswordHash string
	dummyHashOnce     sync.Once
)

// dummyHash returns a valid bcrypt hash used to equalize login timing for
// unknown users / disabled accounts. Without it, a user-not-found branch
// returns far faster than a wrong-password branch, leaking username existence.
func dummyHash() string {
	dummyHashOnce.Do(func() {
		h, err := middleware.HashPassword("forgec2-timing-equalizer-dummy")
		if err == nil {
			dummyPasswordHash = h
		}
	})
	return dummyPasswordHash
}

func (s *Server) handleLoginPage(c *gin.Context) {
	if tokenStr, err := c.Cookie("forgec2_session"); err == nil && tokenStr != "" {
		if _, err := middleware.ParseToken(tokenStr); err == nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		middleware.SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)
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
	respondError(c, http.StatusUnauthorized, errMsg)
}

func (s *Server) handleLogin(c *gin.Context) {

	username := c.PostForm("username")
	password := c.PostForm("password")
	totpCode := c.PostForm("totp_code")
	backupCode := c.PostForm("backup_code")
	rememberMe := c.PostForm("remember_me") == "on"
	clientIP := c.ClientIP()

	slog.Info("Login attempt", "ip", clientIP)

	// Note: no audit record is written for requests rejected while in lockout —
	// the failures that triggered the lockout were already audited, and writing one
	// per subsequent attempt lets an attacker flood the audit log during a lockout.
	if locked, retryAfter := s.checkLoginLockout(clientIP); locked {
		s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
		return
	}

	if locked, retryAfter := s.checkAccountLockout(username); locked {
		s.renderLoginError(c, fmt.Sprintf("Account temporarily locked. Try again in %d seconds.", retryAfter), username, rememberMe)
		return
	}

	if username == "" || password == "" {
		s.renderLoginError(c, "Username and password required", username, rememberMe)
		return
	}

	if middleware.RequireTLSForAuth && !middleware.IsSecureConnection(c) {
		slog.Warn("Login blocked: RequireTLSForAuth=true but connection is not TLS",
			"username", username, "ip", c.ClientIP())
		s.renderLoginError(c, "Login requires a secure (HTTPS) connection", username, rememberMe)
		return
	}

	var user db.User
	result := s.db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		// Equalize timing with the wrong-password branch (bcrypt compare).
		if h := dummyHash(); h != "" {
			middleware.CheckPassword(h, password)
		}
		slog.Warn("Login failed: invalid credentials", "ip", c.ClientIP())
		s.LogAuditRecord(c, "login_failed", "auth", username, "User not found", false, nil)
		if locked, retryAfter := s.recordLoginFailure(clientIP, username); locked {
			s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
			return
		}
		s.renderLoginError(c, "Invalid username or password", username, false)
		return
	}

	if !user.IsActive {
		// Equalize timing with the wrong-password branch.
		if h := dummyHash(); h != "" {
			middleware.CheckPassword(h, password)
		}
		slog.Warn("Login failed: invalid credentials", "ip", c.ClientIP())
		s.LogAuditRecord(c, "login_failed", "auth", username, "Account disabled", false, nil)
		s.renderLoginError(c, "Invalid username or password", username, false)
		return
	}

	if user.PasswordHash == "" {
		if err := s.validatePasswordComplexity(password); err != nil {
			s.renderLoginError(c, err.Error(), username, rememberMe)
			return
		}
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
		slog.Warn("Login failed: invalid credentials", "ip", c.ClientIP())
		s.LogAuditRecord(c, "login_failed", "auth", username, "Wrong password", false, nil)
		if err := s.db.Model(&user).UpdateColumn("login_attempts", gorm.Expr("login_attempts + 1")).Error; err != nil {
			slog.Error("Failed to increment login attempts", "user_id", user.ID, "err", err)
		}
		if locked, retryAfter := s.recordLoginFailure(clientIP, username); locked {
			s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
			return
		}
		if locked, retryAfter := s.recordAccountLoginFailure(username, clientIP); locked {
			s.renderLoginError(c, fmt.Sprintf("Account temporarily locked. Try again in %d seconds.", retryAfter), username, rememberMe)
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

	s.configMu.RLock()
	jwtSecret := s.cfg.Server.JWTSecret
	sessionMaxAgeHours := s.cfg.Server.SessionMaxAgeHours
	s.configMu.RUnlock()

	if user.TOTPSecret != "" {
		if totpCode != "" {
			decryptedSecret, err := decryptSecret(user.TOTPSecret, jwtSecret)
			if err != nil {
				slog.Error("Failed to decrypt TOTP secret", "username", username, "err", err)
				s.renderLoginError(c, "2FA configuration error", username, rememberMe)
				return
			}
			if !totp.VerifyCode(decryptedSecret, totpCode) {
				slog.Warn("Login failed: invalid 2FA code", "username", username, "ip", c.ClientIP())
				s.LogAuditRecord(c, "login_failed", "auth", username, "Invalid 2FA code", false, nil)
				if locked, retryAfter := s.recordLoginFailure(clientIP, username); locked {
					s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
					return
				}
				if locked, retryAfter := s.recordAccountLoginFailure(username, clientIP); locked {
					s.renderLoginError(c, fmt.Sprintf("Account temporarily locked. Try again in %d seconds.", retryAfter), username, rememberMe)
					return
				}
				s.renderLoginError(c, "Invalid username or password", username, rememberMe)
				return
			}
		} else if backupCode != "" {
			var codes []db.BackupCode
			if err := s.db.Where("user_id = ? AND used = false", user.ID).Find(&codes).Error; err != nil || len(codes) == 0 {
				slog.Warn("Login failed: no unused backup codes", "username", username, "ip", c.ClientIP())
				s.LogAuditRecord(c, "login_failed", "auth", username, "No unused backup codes", false, nil)
				if locked, retryAfter := s.recordLoginFailure(clientIP, username); locked {
					s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
					return
				}
				s.renderLoginError(c, "Invalid username or password", username, rememberMe)
				return
			}
			matched := false
			for _, bc := range codes {
				if totp.VerifyBackupCode(bc.CodeHash, backupCode) {
					result := s.db.Model(&db.BackupCode{}).
						Where("id = ? AND used = ?", bc.ID, false).
						Updates(map[string]interface{}{"used": true, "used_at": time.Now()})
					if result.Error != nil {
						slog.Error("Failed to mark backup code as used", "code_id", bc.ID, "err", result.Error)
						s.renderLoginError(c, "Authentication error", username, rememberMe)
						return
					}
					if result.RowsAffected == 0 {
						slog.Warn("Backup code already used (race condition)", "code_id", bc.ID, "username", username)
						s.recordLoginFailure(clientIP, username)
						s.renderLoginError(c, "Invalid username or password", username, rememberMe)
						return
					}
					matched = true
					break
				}
			}
			if !matched {
				slog.Warn("Login failed: invalid backup code", "username", username, "ip", c.ClientIP())
				s.LogAuditRecord(c, "login_failed", "auth", username, "Invalid backup code", false, nil)
				if locked, retryAfter := s.recordLoginFailure(clientIP, username); locked {
					s.renderLoginError(c, fmt.Sprintf("Too many login attempts. Try again in %d seconds.", retryAfter), username, rememberMe)
					return
				}
				if locked, retryAfter := s.recordAccountLoginFailure(username, clientIP); locked {
					s.renderLoginError(c, fmt.Sprintf("Account temporarily locked. Try again in %d seconds.", retryAfter), username, rememberMe)
					return
				}
				s.renderLoginError(c, "Invalid username or password", username, rememberMe)
				return
			}
			slog.Info("Login via backup code", "username", username, "ip", c.ClientIP())
			s.LogAuditRecord(c, "login_backup_code", "auth", username, "Login via backup code", true, nil)
		} else {
			s.renderLoginError(c, "Two-factor authentication required", username, rememberMe)
			return
		}
	}

	token, err := middleware.GenerateToken(user, rememberMe, sessionMaxAgeHours)
	if err != nil {
		s.renderLoginError(c, "Token generation error", username, rememberMe)
		return
	}

	sessionHours := sessionMaxAgeHours
	if sessionHours < 1 {
		sessionHours = DefaultSessionHours
	}
	maxAge := sessionHours * SecondsPerHour
	if rememberMe {
		maxAge = RememberMeMaxAgeSec
	}
	middleware.SetCookieWithSameSite(c, "forgec2_session", token, maxAge, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)

	if err := s.createSession(token, user.ID, c.ClientIP(), c.Request.UserAgent(), "", maxAge); err != nil {
		slog.Error("Failed to create session during login", "user_id", user.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create session")
		return
	}

	var csrfBuf [32]byte
	if _, err := rand.Read(csrfBuf[:]); err != nil {
		slog.Error("Failed to generate CSRF token", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to generate security token")
		return
	}
	csrfToken := hex.EncodeToString(csrfBuf[:])
	middleware.SetCookieWithSameSite(c, "forgec2_csrf", csrfToken, 0, "/", middleware.CookieSecure, false, http.SameSiteLaxMode)

	s.clearLoginLockout(clientIP)
	s.loginLockout.resetAccount(username)
	if err := s.db.Model(&db.User{}).Where("id = ?", user.ID).Update("force_logout_at", nil).Error; err != nil {
		slog.Error("Failed to clear force_logout_at", "user_id", user.ID, "err", err)
	}

	if s.pluginManager != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.pluginManager.ExecuteHook(s.ctx, plugin.Event{
				Type:      plugin.EventUserLogin,
				Timestamp: time.Now(),
				UserID:    user.ID,
				Payload: map[string]interface{}{
					"username": user.Username,
					"role":     user.Role,
					"ip":       c.ClientIP(),
				},
			})
		}()
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
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.pluginManager.ExecuteHook(s.ctx, plugin.Event{
				Type:      plugin.EventUserLogout,
				Timestamp: time.Now(),
				UserID:    userID,
				Payload: map[string]interface{}{
					"username": username,
					"ip":       c.ClientIP(),
				},
			})
		}()
	}

	s.LogAuditRecord(c, "logout", "auth", username, "User logged out", true, nil)
	slog.Info("User logged out", "username", username, "ip", c.ClientIP())
	if tokenStr, err := c.Cookie("forgec2_session"); err == nil && tokenStr != "" {
		s.revokeSession(tokenStr)
	}
	middleware.SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", middleware.CookieSecure, true, http.SameSiteLaxMode)
	c.Redirect(http.StatusFound, "/login")
}

func (s *Server) handleGetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		respondError(c, http.StatusUnauthorized, "not authenticated")
		return
	}

	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusUnauthorized, "user not found")
		return
	}

	if !user.IsActive {
		respondError(c, http.StatusForbidden, "account is disabled")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}
