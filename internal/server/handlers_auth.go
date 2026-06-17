package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleLoginPage(c *gin.Context) {
	csrfToken := middleware.GenerateCSRFToken()
	c.SetCookie("csrf_token", csrfToken, middleware.CookieMaxAge, "/", "", false, false)
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{
		"Title":     "ForgeC2 - Login",
		"CSRFToken": csrfToken,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	password := c.PostForm("password")
	if password == "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{"Error": "Password required"})
		return
	}

	if s.cfg.Auth.PasswordHash == "" {
		hash, err := middleware.HashPassword(password)
		if err != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{"Error": "Failed to set password"})
			return
		}
		s.cfg.Auth.PasswordHash = hash
		_ = s.cfg.Save("config.yaml")
		slog.Info("Initial admin password set")

		token, err := middleware.GenerateToken("admin", c.ClientIP())
		if err != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{"Error": "Token error"})
			return
		}
		c.SetCookie("forgec2_session", token, middleware.CookieMaxAge, "/", "", false, true)
		c.Redirect(http.StatusFound, "/")
		return
	}

	if !middleware.CheckPassword(s.cfg.Auth.PasswordHash, password) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{"Error": "Invalid password"})
		return
	}

	token, err := middleware.GenerateToken("admin", c.ClientIP())
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		s.tmpl.ExecuteTemplate(c.Writer, "login.html", gin.H{"Error": "Token error"})
		return
	}

	c.SetCookie("forgec2_session", token, middleware.CookieMaxAge, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func (s *Server) handleLogout(c *gin.Context) {
	c.SetCookie("forgec2_session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func (s *Server) handleSettingsPage(c *gin.Context) {
	var totalAgents, onlineAgents int64
	s.db.Model(&db.Agent{}).Count(&totalAgents)
	s.db.Model(&db.Agent{}).Where("last_seen > ?", time.Now().Add(-OfflineThreshold)).Count(&onlineAgents)

	data := gin.H{
		"Title":           "ForgeC2 - Settings",
		"ActiveNav":       "settings",
		"DefaultInterval": s.cfg.Agent.DefaultInterval,
		"DefaultJitter":   s.cfg.Agent.DefaultJitter,
		"DefaultUA":       s.cfg.Agent.DefaultUA,
		"ServerAddress":   fmt.Sprintf("%s:%d (HTTPS)", "0.0.0.0", s.cfg.Server.Port),
		"DatabasePath":    s.cfg.Database.Path,
		"TotalAgents":     totalAgents,
		"OnlineAgents":    onlineAgents,
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

	if err := s.cfg.Save("config.yaml"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
		return
	}

	slog.Info("Agent config updated", "interval", s.cfg.Agent.DefaultInterval, "jitter", s.cfg.Agent.DefaultJitter)
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
	if !middleware.CheckPassword(s.cfg.Auth.PasswordHash, current) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password incorrect"})
		return
	}

	hash, err := middleware.HashPassword(newPass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Hash failed"})
		return
	}
	s.cfg.Auth.PasswordHash = hash
	_ = s.cfg.Save("config.yaml")
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAuditLogPage(c *gin.Context) {
	data := gin.H{
		"Title":        "ForgeC2 - 安全审计",
		"ActiveNav":    "audit",
		"Online":       s.cfg.Auth.PasswordHash != "",
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
