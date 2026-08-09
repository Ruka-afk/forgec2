package server

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleSettingsPage(c *gin.Context) {
	type settingsCounts struct {
		TotalAgents    int64
		OnlineAgents   int64
		PendingTasks   int64
		CompletedTasks int64
		FailedTasks    int64
		TotalAudits    int64
		TotalCreds     int64
		TotalTokens    int64
		TotalSocks     int64
		TotalListeners int64
	}
	var counts settingsCounts
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	s.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM implants) AS total_agents,
			(SELECT COUNT(*) FROM implants WHERE last_seen > ?) AS online_agents,
			(SELECT COUNT(*) FROM tasks WHERE status = 'pending') AS pending_tasks,
			(SELECT COUNT(*) FROM tasks WHERE status = 'completed') AS completed_tasks,
			(SELECT COUNT(*) FROM tasks WHERE status = 'failed') AS failed_tasks,
			(SELECT COUNT(*) FROM audit_logs) AS total_audits,
			(SELECT COUNT(*) FROM credential_entries) AS total_creds,
			(SELECT COUNT(*) FROM token_entries) AS total_tokens,
			(SELECT COUNT(*) FROM socks_sessions) AS total_socks,
			(SELECT COUNT(*) FROM listeners) AS total_listeners
	`, offlineCutoff).Scan(&counts)

	var dbSize int64
	if fi, err := os.Stat(s.cfg.Database.Path); err == nil {
		dbSize = fi.Size()
	}

	uptime := time.Since(s.startTime)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	jwtSecret := s.cfg.Server.JWTSecret
	jwtMasked := ""
	if len(jwtSecret) > 0 {
		jwtMasked = "****"
	}

	currentUser, _ := c.Get("user")
	currentRole, _ := c.Get("user_role")
	currentUserID, _ := c.Get("user_id")

	var totalUsers int64
	if err := s.db.Model(&db.User{}).Count(&totalUsers).Error; err != nil {
		slog.Error("Failed to count users", "err", err)
	}

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
		"DatabasePath":     "***",
		"DatabaseSize":     dbSize,
		"DataDir":          "***",
		"JWTMasked":        jwtMasked,
		"TotalAgents":      counts.TotalAgents,
		"OnlineAgents":     counts.OnlineAgents,
		"PendingTasks":     counts.PendingTasks,
		"CompletedTasks":   counts.CompletedTasks,
		"FailedTasks":      counts.FailedTasks,
		"TotalAudits":      counts.TotalAudits,
		"TotalCredentials": counts.TotalCreds,
		"TotalTokens":      counts.TotalTokens,
		"TotalSocks":       counts.TotalSocks,
		"TotalListeners":   counts.TotalListeners,
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

	certPath := s.cfg.Server.CertFile
	if certPath == "" {
		certPath = "data/certs/server.crt"
	}
	if certPEM, err := os.ReadFile(certPath); err == nil {
		if block, _ := pem.Decode(certPEM); block != nil {
			if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				data["CertSubject"] = cert.Subject.CommonName
				data["CertIssuer"] = cert.Issuer.CommonName
				data["CertExpiresAt"] = cert.NotAfter
				data["CertExpiresIn"] = int(time.Until(cert.NotAfter).Hours() / 24)
				data["CertDNSNames"] = cert.DNSNames
				data["CertSelfSigned"] = cert.Subject.CommonName == cert.Issuer.CommonName
			}
		}
	}

	s.renderPageOrJSON(c, data)
}

// validateAndSaveConfig validates the running config before persisting it so an
// invalid value entered via the UI cannot be written to disk and break startup.
func (s *Server) validateAndSaveConfig(c *gin.Context, action string) bool {
	if err := s.cfg.Validate(); err != nil {
		slog.Warn("Config validation failed before save", "action", action, "err", err)
		respondError(c, http.StatusBadRequest, "Config validation failed")
		return false
	}
	if err := s.cfg.Save(s.configPath); err != nil {
		slog.Error("Failed to save config", "action", action, "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to save config")
		return false
	}
	return true
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
		if len(userAgent) > 256 || !isPrintableASCII(userAgent) {
			respondError(c, http.StatusBadRequest, "Invalid user agent")
			return
		}
		s.cfg.Implant.DefaultUA = userAgent
	}

	s.cfg.Implant.DefaultSkipTLS = skipTLS == "true" || skipTLS == "1"

	s.cfg.Implant.DefaultWorkingStart = workingStart
	s.cfg.Implant.DefaultWorkingEnd = workingEnd
	s.cfg.Implant.DefaultWorkingTZ = workingTZ

	if !s.validateAndSaveConfig(c, "agent_config") {
		return
	}

	slog.Info("Agent config updated", "interval", s.cfg.Implant.DefaultInterval, "jitter", s.cfg.Implant.DefaultJitter, "skip_tls", s.cfg.Implant.DefaultSkipTLS)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Agent configuration saved successfully"})
}

func (s *Server) handleChangePassword(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	s.pwdChangeTimesMu.Lock()
	if lastChange, exists := s.pwdChangeTimes[uid]; exists {
		if time.Since(lastChange) < 5*time.Minute {
			s.pwdChangeTimesMu.Unlock()
			remaining := 5*time.Minute - time.Since(lastChange)
			respondError(c, http.StatusTooManyRequests,
				fmt.Sprintf("Password change rate limited. Try again in %d seconds.", int(remaining.Seconds())))
			return
		}
	}
	s.pwdChangeTimesMu.Unlock()

	current := c.PostForm("current_password")
	newPass := c.PostForm("new_password")
	confirm := c.PostForm("confirm_password")

	if newPass != confirm {
		respondError(c, http.StatusBadRequest, "Passwords do not match")
		return
	}
	if err := s.validatePasswordComplexity(newPass); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	userID, _ = c.Get("user_id")
	var user db.User
	if err := s.db.First(&user, userID).Error; err != nil {
		respondError(c, http.StatusUnauthorized, "User not found")
		return
	}

	if !middleware.CheckPassword(user.PasswordHash, current) {
		respondError(c, http.StatusUnauthorized, "Current password incorrect")
		return
	}

	var recentHistory []db.PasswordHistory
	if err := s.db.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(5).Find(&recentHistory).Error; err != nil {
		slog.Error("Failed to query password history", "err", err)
	}
	for _, entry := range recentHistory {
		if middleware.CheckPassword(entry.PasswordHash, newPass) {
			respondError(c, http.StatusBadRequest, "Cannot reuse a recent password")
			return
		}
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

	if err := s.db.Create(&db.PasswordHistory{
		UserID:       user.ID,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}).Error; err != nil {
		slog.Error("Failed to record password history", "user_id", user.ID, "err", err)
	}

	var count int64
	if err := s.db.Model(&db.PasswordHistory{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
		slog.Error("Failed to count password history", "user_id", user.ID, "err", err)
	}
	if count > PasswordHistoryMax {
		var oldest []db.PasswordHistory
		if err := s.db.Where("user_id = ?", user.ID).Order("created_at ASC").Limit(int(count - PasswordHistoryMax)).Find(&oldest).Error; err != nil {
			slog.Error("Failed to query oldest password history", "err", err)
		}
		ids := make([]uint, len(oldest))
		for i, h := range oldest {
			ids[i] = h.ID
		}
		s.db.Delete(&db.PasswordHistory{}, ids)
	}

	s.LogAuditRecord(c, "password_change", "auth", user.Username, "Password changed", true, nil)

	s.pwdChangeTimesMu.Lock()
	s.pwdChangeTimes[user.ID] = time.Now()
	s.pwdChangeTimesMu.Unlock()

	s.revokeAllUserSessions(user.ID)

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

	if !s.validateAndSaveConfig(c, "server_config") {
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

	if !s.validateAndSaveConfig(c, "malleable_profile") {
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
	if days > 365 {
		days = 365
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
		days = 90
	}
	if days > 730 {
		days = 730
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
	// All storage keys (loot/ExtC2/TOTP/backup/CSRF) are independent of the
	// JWT secret, so rotating the JWT cannot invalidate FC2ENC:/FC2EXT:/TOTP
	// ciphertext — no data re-encryption is required.

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Error("Failed to generate JWT secret", "err", err)
		respondError(c, http.StatusInternalServerError, "Failed to generate JWT secret")
		return
	}
	s.cfg.Server.JWTSecret = hex.EncodeToString(b)
	if !s.validateAndSaveConfig(c, "regenerate_jwt") {
		return
	}
	if err := middleware.InitJWTSecret(s.cfg, s.configPath); err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to reinitialize JWT secret")
		return
	}

	slog.Info("JWT secret regenerated (independent encryption keys left untouched)")
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
	if err := s.db.Raw("SELECT page_count * page_size as size FROM pragma_page_count, pragma_page_size").Scan(&dbSize).Error; err != nil {
		slog.Error("Failed to query database size after vacuum", "err", err)
	}
	slog.Info("Database vacuum completed")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Database vacuum completed", "size": dbSize})
}

func (s *Server) handleDownloadConfig(c *gin.Context) {
	role, _ := c.Get("user_role")
	if role != "admin" {
		respondError(c, http.StatusForbidden, "insufficient permissions")
		return
	}
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to read config")
		return
	}
	redacted := string(data)
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
	if err := s.db.Model(&db.User{}).Select("username, role").Where("is_active = ?", true).Limit(100).Find(&users).Error; err != nil {
		slog.Error("Settings: failed to query users for audit page", "err", err)
	}

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
	p := parsePagination(c, 50, 200)
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
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to count audit logs")
		return
	}

	if err := query.Limit(p.PageSize).Offset(p.Offset).Find(&logs).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to query audit logs")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"logs":  logs,
			"total": total,
		},
	})
}

func (s *Server) handleGetSettingsWebhooks(c *gin.Context) {
	var cfg db.ServerConfig
	s.db.Where("key = ?", "notification_targets").First(&cfg)
	if cfg.Value == "" {
		respond(c, gin.H{"data": gin.H{"notifications": []interface{}{}}})
		return
	}
	var targets []interface{}
	if err := json.Unmarshal([]byte(cfg.Value), &targets); err != nil {
		slog.Error("Failed to parse stored notification targets", "err", err)
		respond(c, gin.H{"data": gin.H{"notifications": []interface{}{}}})
		return
	}
	respond(c, gin.H{"data": gin.H{"notifications": targets}})
}

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
		cfg := EmailConfig{
			SMTPHost: req.SMTPHost,
			SMTPPort: req.SMTPPort,
			SMTPUser: req.SMTPUser,
			SMTPPass: req.SMTPPass,
			From:     req.From,
			To:       req.To,
		}
		msg := buildMIMEMessage(cfg.From, cfg.To, "ForgeC2 Test Notification", "<h1>ForgeC2 Test Notification</h1><p>This is a test email from ForgeC2.</p>")
		if err := sendEmailWithRetry(cfg, msg); err != nil {
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "Email test"))
			return
		}
		respond(c, gin.H{"success": true, "message": "Test email sent successfully"})
		return
	}

	if req.URL == "" {
		respondError(c, http.StatusBadRequest, "webhook URL is required")
		return
	}

	parsedURL, err := url.Parse(req.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		respondError(c, http.StatusBadRequest, "invalid webhook URL")
		return
	}
	host := parsedURL.Hostname()
	if isPrivateIP(host) {
		respondError(c, http.StatusBadRequest, "webhook URL cannot target private/internal IPs")
		return
	}

	payload := gin.H{"text": "ForgeC2 test notification", "content": "This is a test notification from ForgeC2."}
	body, ok := marshalJSONSafe(payload)
	if !ok {
		respondError(c, http.StatusInternalServerError, "failed to marshal payload")
		return
	}
	reqHTTP, err := http.NewRequest("POST", req.URL, strings.NewReader(string(body)))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to create request")
		return
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(reqHTTP)
	if err != nil {
		respondError(c, http.StatusBadGateway, sanitizeError(err, "Settings save"))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		respond(c, gin.H{"success": true})
	} else {
		respondError(c, http.StatusBadGateway, fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode))
	}
}

func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 32 || s[i] > 126 {
			return false
		}
	}
	return true
}

func (s *Server) validatePasswordComplexity(password string) error {
	policy := &s.cfg.PasswordPolicy
	minLen := 8
	requireUpper := true
	requireLower := true
	requireDigit := true
	requireSymbol := false
	if policy != nil {
		if policy.MinLength > 0 {
			minLen = policy.MinLength
		}
		requireUpper = policy.RequireUpper
		requireLower = policy.RequireLower
		requireDigit = policy.RequireDigit
		requireSymbol = policy.RequireSymbol
	}
	if len(password) < minLen {
		return fmt.Errorf("password must be at least %d characters", minLen)
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSymbol = true
		}
	}
	if requireUpper && !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if requireLower && !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if requireDigit && !hasDigit {
		return fmt.Errorf("password must contain at least one digit")
	}
	if requireSymbol && !hasSymbol {
		return fmt.Errorf("password must contain at least one special character")
	}
	return nil
}
