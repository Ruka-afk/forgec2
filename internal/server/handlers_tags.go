package server

import (
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) handleAPITagList(c *gin.Context) {
	var tags []db.AgentTag
	s.db.Order("name asc").Find(&tags)
	respond(c, gin.H{"tags": tags})
}

func (s *Server) handleAPITagCreate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		respondError(c, http.StatusBadRequest, "name required")
		return
	}
	if req.Color == "" {
		req.Color = "#3498db"
	}
	tag := db.AgentTag{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Color:     req.Color,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.Create(&tag).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Tag creation"))
		return
	}
	s.LogAuditRecord(c, "tag_create", "tag", tag.ID, "Tag "+tag.Name+" created", true, nil)
	respond(c, gin.H{"success": true, "tag": tag})
}

func (s *Server) handleAPITagUpdate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var tag db.AgentTag
	if !s.findOrFail(c, &tag, id, "tag") {
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name != "" {
		tag.Name = req.Name
	}
	if req.Color != "" {
		tag.Color = req.Color
	}
	tag.UpdatedAt = time.Now()
	if err := s.db.Save(&tag).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update tag")
		return
	}
	s.LogAuditRecord(c, "tag_update", "tag", tag.ID, "Tag "+tag.Name+" updated", true, nil)
	respond(c, gin.H{"success": true, "tag": tag})
}

func (s *Server) handleAPITagDelete(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var tag db.AgentTag
	if !s.findOrFail(c, &tag, id, "tag") {
		return
	}
	s.db.Model(&tag).Association("Tags").Clear()
	if err := s.db.Delete(&tag).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Tag deletion"))
		return
	}
	s.LogAuditRecord(c, "tag_delete", "tag", id, "Tag "+tag.Name+" deleted", true, nil)
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAgentTags(c *gin.Context) {
	id := c.Param("id")
	var tags []db.AgentTag
	s.db.Model(&db.Implant{ID: id}).Association("Tags").Find(&tags)
	respond(c, gin.H{"tags": tags})
}

// handleBatchAgentTags returns tags for multiple agents in a single request,
// eliminating the N+1 problem on the agents list page.
func (s *Server) handleBatchAgentTags(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AgentIDs) == 0 {
		respondError(c, http.StatusBadRequest, "agent_ids required")
		return
	}

	type agentTagsRow struct {
		AgentID string        `gorm:"column:agent_id"`
		TagID   string        `gorm:"column:tag_id"`
		Name    string        `gorm:"column:name"`
		Color   string        `gorm:"column:color"`
	}
	var rows []agentTagsRow
	s.db.Table("agent_tag_assignments").
		Select("agent_tag_assignments.agent_id, agent_tags.id as tag_id, agent_tags.name, agent_tags.color").
		Joins("JOIN agent_tags ON agent_tags.id = agent_tag_assignments.tag_id").
		Where("agent_tag_assignments.agent_id IN ?", req.AgentIDs).
		Find(&rows)

	tagsByAgent := make(map[string][]gin.H)
	for _, r := range rows {
		tagsByAgent[r.AgentID] = append(tagsByAgent[r.AgentID], gin.H{
			"id":    r.TagID,
			"name":  r.Name,
			"color": r.Color,
		})
	}

	respond(c, gin.H{"tags": tagsByAgent})
}
