package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// handleUsersPage shows user management
func (s *Server) handleUsersPage(c *gin.Context) {
	var users []db.User
	s.db.Order("created_at desc").Find(&users)

	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - User Management",
		"ActiveNav": "settings",
		"Users":     users,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "users_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

// handleAddUser creates a new user
func (s *Server) handleAddUser(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	role := c.PostForm("role")

	if username == "" || password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password required"})
		return
	}
	if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters"})
		return
	}
	if role == "" {
		role = "operator"
	}
	if role != "admin" && role != "operator" {
		role = "operator"
	}

	hash, err := middleware.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := db.User{
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
	}

	if result := s.db.Create(&user); result.Error != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	currentUser, _ := c.Get("user")
	s.LogAuditRecord(c, "user_create", "auth", currentUser.(string),
		fmt.Sprintf("Created user %s with role %s", username, role), true, nil)
	slog.Info("User created", "username", username, "role", role, "by", currentUser)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("User %s created", username)})
}

// handleToggleUser enables/disables a user
func (s *Server) handleToggleUser(c *gin.Context) {
	idStr := c.Param("id")
	var user db.User
	if err := s.db.First(&user, idStr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	currentUser, _ := c.Get("user")
	currentRole, _ := c.Get("user_role")

	// Prevent disabling yourself
	if currentUser == user.Username {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot disable your own account"})
		return
	}
	// Only admins can toggle users
	if currentRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	s.db.Model(&user).Update("is_active", !user.IsActive)
	status := "enabled"
	if !user.IsActive {
		status = "disabled"
	}
	s.LogAuditRecord(c, "user_toggle", "auth", currentUser.(string),
		fmt.Sprintf("%s account %s", status, user.Username), true, nil)
	slog.Info("User toggled", "username", user.Username, "active", !user.IsActive)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("User %s", status)})
}

// handleDeleteUser removes a user (admin only)
func (s *Server) handleDeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	currentUser, _ := c.Get("user")
	currentRole, _ := c.Get("user_role")

	if currentRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var user db.User
	if err := s.db.First(&user, idStr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Prevent deleting yourself
	if currentUser == user.Username {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete your own account"})
		return
	}

	s.db.Delete(&user)
	s.LogAuditRecord(c, "user_delete", "auth", currentUser.(string),
		fmt.Sprintf("Deleted user %s", user.Username), true, nil)
	slog.Info("User deleted", "username", user.Username, "by", currentUser)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("User %s deleted", user.Username)})
}

// handleSetUserPassword (admin sets password for user)
func (s *Server) handleSetUserPassword(c *gin.Context) {
	idStr := c.Param("id")
	password := c.PostForm("password")
	currentRole, _ := c.Get("user_role")

	if currentRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}
	if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters"})
		return
	}

	hash, err := middleware.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Hash failed"})
		return
	}

	result := s.db.Model(&db.User{}).Where("id = ?", idStr).Update("password_hash", hash)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password updated"})
}
