package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleLoginPage(c *gin.Context) {
	// Use token from CSRFProtection middleware if available, otherwise generate
	csrfToken, exists := c.Get("csrf_token_value")
	if !exists {
		csrfToken = middleware.GenerateCSRFToken()
	}
	csrfTokenStr := csrfToken.(string)
	c.SetCookie("csrf_token", csrfTokenStr, middleware.CookieMaxAge, "/", "", middleware.CookieSecure, false)

	// Check if already logged in
	if _, err := c.Cookie("forgec2_session"); err == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Check for error from rate limiter redirect
	var errMsg string
	if c.Query("error") == "rate_limited" {
		errMsg = "Too many login attempts. Please try again later."
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{
		"Title":     "ForgeC2 - Login",
		"CSRFToken": csrfTokenStr,
		"Error":     errMsg,
	})
}

func (s *Server) renderLoginError(c *gin.Context, errMsg, lastUsername string, rememberMe bool) {
	csrfToken := middleware.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, middleware.CookieMaxAge, "/", "", middleware.CookieSecure, false)
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{
		"Error":       errMsg,
		"LastUsername": lastUsername,
		"RememberMe":  rememberMe,
		"CSRFToken":   csrfToken,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	rememberMe := c.PostForm("remember_me") == "on"

	if username == "" || password == "" {
		s.renderLoginError(c, "Username and password required", username, rememberMe)
		return
	}

	// Find user in DB
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

	// First login: set password
	if user.PasswordHash == "" {
		hash, err := middleware.HashPassword(password)
		if err != nil {
			s.renderLoginError(c, "Failed to set password", username, rememberMe)
			return
		}
		s.db.Model(&user).Updates(map[string]interface{}{
			"password_hash": hash,
			"last_login":    time.Now(),
			"last_ip":       c.ClientIP(),
		})
		// Update local user object with the new hash
		user.PasswordHash = hash
		slog.Info("Password set for user", "username", username)
	} else if !middleware.CheckPassword(user.PasswordHash, password) {
		slog.Warn("Login failed: wrong password", "username", username, "ip", c.ClientIP())
		s.LogAuditRecord(c, "login_failed", "auth", username, "Wrong password", false, nil)
		// Progressive delay to slow brute-force attacks (cap at 10 attempts = 5 seconds)
		delay := user.LoginAttempts
		if delay > 10 {
			delay = 10
		}
		time.Sleep(time.Duration(delay) * 500 * time.Millisecond)
		s.db.Model(&user).UpdateColumn("login_attempts", user.LoginAttempts+1)
		s.renderLoginError(c, "Invalid username or password", username, rememberMe)
		return
	} else {
		// Update last login and reset attempts
		s.db.Model(&user).Updates(map[string]interface{}{
			"last_login":     time.Now(),
			"last_ip":        c.ClientIP(),
			"login_attempts": 0,
		})
	}

	token, err := middleware.GenerateToken(user, rememberMe, s.cfg.Server.SessionMaxAgeHours)
	if err != nil {
		s.renderLoginError(c, "Token generation error", username, rememberMe)
		return
	}

	sessionHours := s.cfg.Server.SessionMaxAgeHours
	if sessionHours < 1 {
		sessionHours = 24
	}
	maxAge := sessionHours * 3600
	if rememberMe {
		maxAge = 7 * 86400
	}
	c.SetCookie("forgec2_session", token, maxAge, "/", "", middleware.CookieSecure, true)

	// Clear force logout flag after successful login
	s.db.Model(&db.User{}).Where("id = ?", user.ID).Update("force_logout_at", nil)

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
	s.LogAuditRecord(c, "logout", "auth", username, "User logged out", true, nil)
	slog.Info("User logged out", "username", username, "ip", c.ClientIP())
	c.SetCookie("forgec2_session", "", -1, "/", "", middleware.CookieSecure, true)
	c.Redirect(http.StatusFound, "/login")
}

func (s *Server) handleSettingsPage(c *gin.Context) {
	var totalAgents, onlineAgents int64
	s.db.Model(&db.Agent{}).Count(&totalAgents)
	s.db.Model(&db.Agent{}).Where("last_seen > ?", time.Now().Add(-s.offlineThreshold())).Count(&onlineAgents)

	// Database statistics
	var (
		pendingTasks  int64
		completedTasks int64
		failedTasks   int64
		totalAudits   int64
		totalCreds    int64
		totalTokens   int64
		totalSocks    int64
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
	if len(jwtSecret) > 8 {
		jwtMasked = jwtSecret[:4] + strings.Repeat("*", len(jwtSecret)-8) + jwtSecret[len(jwtSecret)-4:]
	} else if len(jwtSecret) > 0 {
		jwtMasked = strings.Repeat("*", len(jwtSecret))
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
		"DefaultInterval":  s.cfg.Agent.DefaultInterval,
		"DefaultJitter":    s.cfg.Agent.DefaultJitter,
		"DefaultSkipTLS":   s.cfg.Agent.DefaultSkipTLS,
		"DefaultUA":        s.cfg.Agent.DefaultUA,
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
		"OfflineThreshold":  s.cfg.Server.OfflineThreshold,
		"SessionMaxAge":     s.cfg.Server.SessionMaxAgeHours,
		"CleanupRetention":  s.cfg.Server.CleanupRetentionDays,
		"MalleableEnabled":  s.cfg.Malleable.Enabled,
		"MalleableStatus":   s.cfg.Malleable.StatusCode,
		"MalleableCT":       s.cfg.Malleable.ContentType,
		"MalleableHeaders":  s.cfg.Malleable.Headers,
		"MalleablePrepend":  s.cfg.Malleable.Prepend,
		"MalleableAppend":   s.cfg.Malleable.Append,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "settings_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleSaveAgentConfig(c *gin.Context) {
	interval := c.PostForm("interval")
	jitter := c.PostForm("jitter")
	userAgent := c.PostForm("user_agent")
	skipTLS := c.PostForm("skip_tls")

	if interval != "" {
		var intInterval int
		if _, err := fmt.Sscanf(interval, "%d", &intInterval); err == nil && intInterval > 0 {
			s.cfg.Agent.DefaultInterval = intInterval
		}
	}

	if jitter != "" {
		var intJitter int
		if _, err := fmt.Sscanf(jitter, "%d", &intJitter); err == nil && intJitter >= 0 && intJitter <= 100 {
			s.cfg.Agent.DefaultJitter = intJitter
		}
	}

	if userAgent != "" {
		s.cfg.Agent.DefaultUA = userAgent
	}

	s.cfg.Agent.DefaultSkipTLS = skipTLS == "true" || skipTLS == "1"

	if err := s.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	slog.Info("Agent config updated", "interval", s.cfg.Agent.DefaultInterval, "jitter", s.cfg.Agent.DefaultJitter, "skip_tls", s.cfg.Agent.DefaultSkipTLS)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Agent configuration saved successfully"})
}

func (s *Server) handleChangePassword(c *gin.Context) {
	current := c.PostForm("current_password")
	newPass := c.PostForm("new_password")
	confirm := c.PostForm("confirm_password")

	if newPass != confirm || len(newPass) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match or too short"})
		return
	}

	// Get current user from DB
	userID, _ := c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	if !middleware.CheckPassword(user.PasswordHash, current) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password incorrect"})
		return
	}

	hash, err := middleware.HashPassword(newPass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Hash failed"})
		return
	}

	s.db.Model(&user).Update("password_hash", hash)
	s.LogAuditRecord(c, "password_change", "auth", user.Username, "Password changed", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
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
		if v, err := strconv.Atoi(offlineThreshold); err == nil && v >= 5 && v <= 3600 {
			s.cfg.Server.OfflineThreshold = v
		}
	}

	if sessionMaxAge != "" {
		if v, err := strconv.Atoi(sessionMaxAge); err == nil && v >= 1 && v <= 720 {
			s.cfg.Server.SessionMaxAgeHours = v
		}
	}

	if cleanupRetention != "" {
		if v, err := strconv.Atoi(cleanupRetention); err == nil && v >= 1 && v <= 365 {
			s.cfg.Server.CleanupRetentionDays = v
		}
	}

	if err := s.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	slog.Info("Server config updated", "log_level", s.cfg.Logging.Level, "tcp_enabled", s.cfg.Server.TCPEnabled)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Server configuration saved"})
}

func (s *Server) handleSaveMalleableProfile(c *gin.Context) {
	s.cfg.Malleable.Enabled = c.PostForm("enabled") == "true"
	if sc := c.PostForm("status_code"); sc != "" {
		if v, err := strconv.Atoi(sc); err == nil && v >= 100 && v <= 599 {
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

	if err := s.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to purge tasks"})
		return
	}
	slog.Info("Purged old tasks", "count", result.RowsAffected, "older_than_days", days)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Purged %d old tasks", result.RowsAffected)})
}

func (s *Server) handlePurgeAuditLogs(c *gin.Context) {
	daysStr := c.PostForm("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	result := s.db.Where("created_at < ?", cutoff).Delete(&db.AuditLog{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to purge audit logs"})
		return
	}
	slog.Info("Purged old audit logs", "count", result.RowsAffected, "older_than_days", days)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Purged %d old audit logs", result.RowsAffected)})
}

func (s *Server) handleRegenerateJWT(c *gin.Context) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Error("Failed to generate JWT secret", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate JWT secret"})
		return
	}
	s.cfg.Server.JWTSecret = hex.EncodeToString(b)
	if err := s.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database connection"})
		return
	}
	if _, err := rawDB.Exec("VACUUM"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "VACUUM failed"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup directory"})
		return
	}
	ts := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("forgec2_%s.db", ts))
	srcFile, err := os.Open(src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open database file"})
		return
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create backup file"})
		return
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy database"})
		return
	}
	slog.Info("Database backup created", "path", backupPath)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Backup saved: %s", backupPath)})
}

func (s *Server) handleDownloadConfig(c *gin.Context) {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read config")
		return
	}
	// Redact JWT secret before serving
	redacted := strings.Replace(string(data), s.cfg.Server.JWTSecret, strings.Repeat("*", len(s.cfg.Server.JWTSecret)), 1)
	// Redact password hash
	if s.cfg.Auth.PasswordHash != "" {
		redacted = strings.Replace(redacted, s.cfg.Auth.PasswordHash, strings.Repeat("*", len(s.cfg.Auth.PasswordHash)), 1)
	}
	c.Header("Content-Disposition", "attachment; filename=config.yaml")
	c.Data(http.StatusOK, "application/x-yaml", []byte(redacted))
}

func (s *Server) handleAuditLogPage(c *gin.Context) {
	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Security Audit",
		"ActiveNav": "audit",
		"Online":    s.cfg.Auth.PasswordHash != "",
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "audit_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())

	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleGetAuditLogs(c *gin.Context) {
	var logs []db.AuditLog
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("pageSize", "50")
	search := c.DefaultQuery("search", "")
	action := c.DefaultQuery("action", "")

	query := s.db.Model(&db.AuditLog{}).Order("created_at DESC")

	if search != "" {
		query = query.Where("user LIKE ? OR resource LIKE ? OR details LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	if action != "" {
		query = query.Where("action = ?", action)
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
