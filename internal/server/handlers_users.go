package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// handleUsersPage shows user management
func (s *Server) handleUsersPage(c *gin.Context) {
	var users []db.User
	s.db.Order("created_at desc").Find(&users)

	stats := s.getNavStats()
	data := gin.H{
		"Title":           "ForgeC2 - User Management",
		"ActiveNav":       "settings",
		"Users":           users,
		"AllRoles":        db.GetAllRoles(),
		"AllPermissions":  db.GetAllPermissions(),
		"RolePermissions": db.RolePermissionsMap,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// handleAddUser creates a new user (admin only)
func (s *Server) handleAddUser(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	username := c.PostForm("username")
	password := c.PostForm("password")
	role := c.PostForm("role")

	if username == "" || password == "" {
		respondError(c, http.StatusBadRequest, "Username and password required")
		return
	}
	if err := s.validatePasswordComplexity(password); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	validRoles := map[string]bool{
		db.RoleAdmin: true,
		db.RoleUser:  true,
	}
	if !validRoles[role] {
		role = db.RoleUser
	}

	hash, err := middleware.HashPassword(password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	var user db.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		user = db.User{
			Username:     username,
			PasswordHash: hash,
			Role:         role,
			IsActive:     true,
		}
		return tx.Create(&user).Error
	})
	if err != nil {
		respondError(c, http.StatusConflict, "Username already exists")
		return
	}

	currentUser, _ := c.Get("user")
	s.LogAuditRecord(c, "user_create", "auth", currentUser.(string),
		fmt.Sprintf("Created user %s with role %s", username, role), true, nil)
	slog.Info("User created", "username", username, "role", role, "user", currentUser)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("User %s created", username)})
}

// handleToggleUser enables/disables a user
func (s *Server) handleToggleUser(c *gin.Context) {
	idStr := c.Param("id")
	var user db.User
	if !s.findOrFail(c, &user, idStr, "User") {
		return
	}

	currentUser, _ := c.Get("user")

	// Prevent disabling yourself
	if currentUser == user.Username {
		respondError(c, http.StatusBadRequest, "Cannot disable your own account")
		return
	}
	// Only admins can toggle users
	if !s.requireAdmin(c) {
		return
	}

	if err := s.db.Model(&user).Update("is_active", !user.IsActive).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to toggle user")
		return
	}
	status := "enabled"
	if !user.IsActive {
		status = "disabled"
	}
	s.LogAuditRecord(c, "user_toggle", "auth", currentUser.(string),
		fmt.Sprintf("%s account %s", status, user.Username), true, nil)
	slog.Info("User toggled", "username", user.Username, "active", !user.IsActive)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("User %s", status)})
}
func (s *Server) handleKickUser(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Kick functionality removed"})
}

// handleEditUser updates username/role (admin only)
func (s *Server) handleEditUser(c *gin.Context) {
	idStr := c.Param("id")
	if !s.requireAdmin(c) {
		return
	}

	var user db.User
	if !s.findOrFail(c, &user, idStr, "User") {
		return
	}

	username := c.PostForm("username")
	role := c.PostForm("role")

	updates := make(map[string]interface{})
	if username != "" && username != user.Username {
		// Check uniqueness
		var dup db.User
		if s.db.Where("username = ?", username).First(&dup).Error == nil {
			respondError(c, http.StatusConflict, "Username already taken")
			return
		}
		updates["username"] = username
	}
	if role != "" && role != user.Role {
		validRoles := map[string]bool{
			db.RoleAdmin: true,
			db.RoleUser:  true,
		}
		if !validRoles[role] {
			respondError(c, http.StatusBadRequest, "Invalid role")
			return
		}
		updates["role"] = role
	}

	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "No changes"})
		return
	}

	if err := s.db.Model(&user).Updates(updates).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update user")
		return
	}
	currentUser, _ := c.Get("user")
	s.LogAuditRecord(c, "user_edit", "auth", currentUser.(string),
		fmt.Sprintf("Edited user %s: %v", user.Username, updates), true, nil)
	slog.Info("User edited", "user_id", idStr, "updates", updates, "user", currentUser)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "User updated"})
}

// handleForceLogoutUser invalidates all sessions for a user (admin only)
func (s *Server) handleForceLogoutUser(c *gin.Context) {
	idStr := c.Param("id")
	if !s.requireAdmin(c) {
		return
	}

	currentUser, _ := c.Get("user")
	var target db.User
	if !s.findOrFail(c, &target, idStr, "User") {
		return
	}

	if currentUser == target.Username {
		respondError(c, http.StatusBadRequest, "Cannot force-logout yourself")
		return
	}

	// Set ForceLogoutAt to now
	now := time.Now()
	if err := s.db.Model(&target).Update("force_logout_at", now).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to force logout user")
		return
	}

	s.LogAuditRecord(c, "user_force_logout", "auth", currentUser.(string),
		fmt.Sprintf("Force logged out user %s", target.Username), true, nil)
	slog.Info("User force logged out", "username", target.Username, "user", currentUser)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Force logged out %s", target.Username)})
}

// handleDeleteUser removes a user (admin only)
func (s *Server) handleDeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	currentUser, _ := c.Get("user")

	if !s.requireAdmin(c) {
		return
	}

	var user db.User
	if !s.findOrFail(c, &user, idStr, "User") {
		return
	}

	// Prevent deleting yourself
	if currentUser == user.Username {
		respondError(c, http.StatusBadRequest, "Cannot delete your own account")
		return
	}

	// Use transaction to clean up associated data
	tx := s.db.Begin()
	if err := tx.Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}

	// Delete the user
	if err := tx.Delete(&user).Error; err != nil {
		tx.Rollback()
		respondError(c, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	if err := tx.Commit().Error; err != nil {
		respondError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	s.LogAuditRecord(c, "user_delete", "auth", currentUser.(string),
		fmt.Sprintf("Deleted user %s", user.Username), true, nil)
	slog.Info("User deleted", "username", user.Username, "user", currentUser)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("User %s deleted", user.Username)})
}

// handleSetUserPassword (admin sets password for user)
func (s *Server) handleSetUserPassword(c *gin.Context) {
	idStr := c.Param("id")
	password := c.PostForm("password")

	if !s.requireAdmin(c) {
		return
	}
	if err := s.validatePasswordComplexity(password); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := middleware.HashPassword(password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "Hash failed")
		return
	}

	result := s.db.Model(&db.User{}).Where("id = ?", idStr).Update("password_hash", hash)
	if result.Error != nil {
		respondError(c, http.StatusInternalServerError, "Database error")
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password updated"})
}
