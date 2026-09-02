package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// handleAISessionBranch creates an independent session containing the visible
// history through the selected message. The parent is never modified.
func (s *Server) handleAISessionBranch(c *gin.Context) {
	principal, ok := s.currentAIPrincipal(c)
	if !ok {
		respondError(c, http.StatusForbidden, "AI use permission required")
		return
	}
	sessionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid session id")
		return
	}
	var req struct {
		MessageID uint   `json:"message_id"`
		Title     string `json:"title"`
	}
	if err := bindLimitedAISessionJSON(c, aiSessionMetadataMaxBody, &req); err != nil || req.MessageID == 0 {
		respondError(c, http.StatusBadRequest, "message_id is required")
		return
	}
	var parent db.AIChatSession
	if err := s.db.Where("id = ? AND owner_id = ? AND tenant_id = ?", sessionID, principal.UserID, principal.TenantID).First(&parent).Error; err != nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}
	var forkMessage db.AIChatMessage
	if err := s.db.Where("id = ? AND session_id = ?", req.MessageID, parent.ID).First(&forkMessage).Error; err != nil {
		respondError(c, http.StatusNotFound, "branch message not found")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = truncateString(parent.Title+" (branch)", aiSessionTitleMaxRunes)
	}
	child := db.AIChatSession{
		TenantID: principal.TenantID, OwnerID: principal.UserID, Owner: principal.Username,
		Title: title, ProfileID: parent.ProfileID, ParentSessionID: &parent.ID,
		ForkMessageID: &forkMessage.ID, ContextAgentID: parent.ContextAgentID,
		WritePolicy: parent.WritePolicy, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&child).Error; err != nil {
			return err
		}
		var source []db.AIChatMessage
		if err := tx.Where("session_id = ? AND id <= ?", parent.ID, forkMessage.ID).Order("id ASC").Find(&source).Error; err != nil {
			return err
		}
		copies := make([]db.AIChatMessage, 0, len(source))
		for _, message := range source {
			copies = append(copies, db.AIChatMessage{
				SessionID: child.ID, RunID: message.RunID, Role: message.Role,
				Content: message.Content, ToolName: message.ToolName, CreatedAt: message.CreatedAt,
			})
		}
		if len(copies) > 0 {
			return tx.CreateInBatches(&copies, 100).Error
		}
		return nil
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to branch session")
		return
	}
	s.LogOperatorAction(c, OperatorAction{Action: "ai_session_branch", Resource: "ai", TargetID: fmt.Sprint(child.ID), RiskLevel: "low", Details: fmt.Sprintf("parent=%d message=%d", parent.ID, forkMessage.ID)})
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": child})
}
