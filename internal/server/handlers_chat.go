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
		"ActiveNav":       "chat",
		"Title": "Operator Chat",
		"CurrentUsername": user,
	}
	s.renderPage(c, "chat_content", data)
}

func (s *Server) handleGetChatMessages(c *gin.Context) {
	limit := 100

	var messages []db.ChatMessage
	s.db.Order("created_at desc").Limit(limit).Find(&messages)

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
		CreatedAt: time.Now(),
	}
	s.db.Create(&msg)

	if s.chatHub != nil {
		payload := gin.H{
			"user":       msg.User,
			"message":    msg.Message,
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

	if msg.User != user.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	s.db.Delete(&msg)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
