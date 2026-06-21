package server

import (
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleTemplatesPage renders the command templates page
func (s *Server) handleTemplatesPage(c *gin.Context) {
	stats := s.getNavStats()

	// Get all templates
	var templates []db.CommandTemplate
	s.db.Order("category, name").Find(&templates)

	// Group by category
	categories := make(map[string][]db.CommandTemplate)
	for _, t := range templates {
		categories[t.Category] = append(categories[t.Category], t)
	}

	data := gin.H{
		"Title":      "ForgeC2 - Command Templates",
		"ActiveNav":  "templates",
		"Stats":      stats,
		"Templates":  templates,
		"Categories": categories,
	}
	s.renderPage(c, "templates_content", data)
}

// handleCreateTemplate creates a new command template
func (s *Server) handleCreateTemplate(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Category    string `json:"category" binding:"required"`
		Command     string `json:"command" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	template := db.CommandTemplate{
		Name:        req.Name,
		Category:    req.Category,
		Command:     req.Command,
		Description: req.Description,
	}

	if err := s.db.Create(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"template": template,
	})
}

// handleDeleteTemplate deletes a command template
func (s *Server) handleDeleteTemplate(c *gin.Context) {
	templateID := c.Param("id")

	if err := s.db.Delete(&db.CommandTemplate{}, templateID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleGetTemplatesByCategory returns templates by category
func (s *Server) handleGetTemplatesByCategory(c *gin.Context) {
	category := c.Param("category")

	var templates []db.CommandTemplate
	s.db.Where("category = ?", category).Order("name").Find(&templates)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}
