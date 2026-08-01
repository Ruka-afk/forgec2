package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleAISessionsList returns all AI chat sessions for the current user.
func (s *Server) handleAISessionsList(c *gin.Context) {
	username, _ := c.Get("user")

	var sessions []db.AIChatSession
	if err := s.db.Where("owner = ?", username).Order("updated_at desc").Limit(100).Find(&sessions).Error; err != nil {
		slog.Error("Failed to list AI sessions", "err", err)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": sessions})
}

// handleAISessionsCreate creates a new AI chat session.
func (s *Server) handleAISessionsCreate(c *gin.Context) {
	username, _ := c.Get("user")

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	title := req.Title
	if title == "" {
		title = "New Chat"
	}

	session := db.AIChatSession{
		Title: title,
		Owner: username.(string),
	}
	if err := s.db.Create(&session).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create session")
		return
	}
	s.LogAuditRecord(c, "ai_session_create", "ai", "", "Created AI chat session: "+title, true, nil)
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": session})
}

// handleAISessionsGet returns messages for a specific AI chat session.
func (s *Server) handleAISessionsGet(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid session id")
		return
	}

	username, _ := c.Get("user")
	var session db.AIChatSession
	if err := s.db.Where("id = ? AND owner = ?", sessionID, username).First(&session).Error; err != nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}

	var messages []db.AIChatMessage
	if err := s.db.Where("session_id = ?", sessionID).Order("created_at asc").Limit(1000).Find(&messages).Error; err != nil {
		slog.Error("Failed to list AI messages", "err", err)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": messages})
}

// handleAISessionsMessages creates a new message in an AI chat session.
func (s *Server) handleAISessionsMessages(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid session id")
		return
	}

	username, _ := c.Get("user")
	var session db.AIChatSession
	if err := s.db.Where("id = ? AND owner = ?", sessionID, username).First(&session).Error; err != nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}

	var req struct {
		Role     string `json:"role"`
		Content  string `json:"content"`
		ToolName string `json:"tool_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	msg := db.AIChatMessage{
		SessionID: uint(sessionID),
		Role:      req.Role,
		Content:   req.Content,
		ToolName:  req.ToolName,
	}
	if err := s.db.Create(&msg).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to save message")
		return
	}

	if err := s.db.Model(&db.AIChatSession{}).Where("id = ?", sessionID).Update("updated_at", time.Now()).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update session timestamp")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": msg})
}

// handleAISessionsUpdate renames an AI chat session.
func (s *Server) handleAISessionsUpdate(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid session id")
		return
	}

	username, _ := c.Get("user")
	var session db.AIChatSession
	if err := s.db.Where("id = ? AND owner = ?", sessionID, username).First(&session).Error; err != nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Title != "" {
		session.Title = req.Title
	}

	if err := s.db.Save(&session).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": session})
}

// handleAISessionsDelete deletes an AI chat session and its messages.
func (s *Server) handleAISessionsDelete(c *gin.Context) {
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid session id")
		return
	}

	username, _ := c.Get("user")
	var session db.AIChatSession
	if err := s.db.Where("id = ? AND owner = ?", sessionID, username).First(&session).Error; err != nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}

	if err := s.db.Where("session_id = ?", sessionID).Delete(&db.AIChatMessage{}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete session messages")
		return
	}
	if err := s.db.Delete(&session).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete session")
		return
	}
	s.LogAuditRecord(c, "ai_session_delete", "ai", "", "Deleted AI chat session", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}
