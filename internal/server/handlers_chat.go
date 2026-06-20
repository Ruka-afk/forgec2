package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleChatPage(c *gin.Context) {
	user, _ := c.Get("user")
	data := gin.H{
		"ActiveNav":      "chat",
		"Title":          "操作员聊天",
		"CurrentUsername": user,
	}
	s.addUserToData(c, data)
	s.renderPage(c, "chat_content", data)
}

func (s *Server) handleGetChatMessages(c *gin.Context) {
	channel := c.DefaultQuery("channel", "global")
	limit := 100

	var messages []db.ChatMessage
	s.db.Where("channel = ?", channel).Order("created_at desc").Limit(limit).Find(&messages)

	// Reverse to chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
	})
}

func (s *Server) handleSendChatMessage(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required"`
		Channel string `json:"channel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, _ := c.Get("user")
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	role, _ := c.Get("user_role")

	msg := db.ChatMessage{
		User:      user.(string),
		Message:   req.Message,
		Channel:   req.Channel,
		CreatedAt: time.Now(),
	}
	s.db.Create(&msg)

	// Broadcast via WebSocket to all connected chat clients
	if s.chatHub != nil {
		payload := gin.H{
			"user":       msg.User,
			"message":    msg.Message,
			"channel":    msg.Channel,
			"timestamp":  msg.CreatedAt,
			"created_at": msg.CreatedAt,
			"role":       role,
		}
		data, err := json.Marshal(payload)
		if err == nil {
			s.chatHub.broadcast <- data
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "sent",
		"id":      msg.ID,
	})
}

func (s *Server) handleDeleteChatMessage(c *gin.Context) {
	id := c.Param("id")

	var msg db.ChatMessage
	if err := s.db.First(&msg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "message not found"})
		return
	}

	user, _ := c.Get("user")
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Only allow deletion by sender or admin
	if msg.User != user.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	s.db.Delete(&msg)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
