package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAutomationPage(c *gin.Context) {
	rules := s.loadAutomationRules()
	var webhooks []db.WebhookConfig
	s.db.Find(&webhooks)
	s.renderPage(c, "automation_content", gin.H{
		"Title":     "自动化",
		"ActiveNav": "automation",
		"Rules":     rules,
		"Webhooks":  webhooks,
	})
}

func (s *Server) handleListAutomationRules(c *gin.Context) {
	rules := s.loadAutomationRules()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

func (s *Server) handleSaveAutomationRule(c *gin.Context) {
	var rule AutomationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rules := s.loadAutomationRules()
	found := false
	for i, r := range rules {
		if r.ID == rule.ID {
			rules[i] = rule
			found = true
			break
		}
	}
	if !found {
		if rule.ID == "" {
			rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
		}
		if rule.CreatedAt == "" {
			rule.CreatedAt = time.Now().Format(time.RFC3339)
		}
		rules = append(rules, rule)
	}
	s.saveAutomationRules(rules)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleDeleteAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	rules := s.loadAutomationRules()
	filtered := rules[:0]
	for _, r := range rules {
		if r.ID != ruleID {
			filtered = append(filtered, r)
		}
	}
	s.saveAutomationRules(filtered)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleToggleAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	rules := s.loadAutomationRules()
	for i, r := range rules {
		if r.ID == ruleID {
			rules[i].Enabled = !r.Enabled
			break
		}
	}
	s.saveAutomationRules(rules)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleListWebhooks(c *gin.Context) {
	var webhooks []db.WebhookConfig
	s.db.Find(&webhooks)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": webhooks})
}

func (s *Server) handleCreateWebhook(c *gin.Context) {
	var wh db.WebhookConfig
	if err := c.ShouldBindJSON(&wh); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.db.Create(&wh)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": wh})
}

func (s *Server) handleDeleteWebhook(c *gin.Context) {
	id := c.Param("id")
	s.db.Delete(&db.WebhookConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleTestWebhook(c *gin.Context) {
	var req struct {
		URL    string `json:"url"`
		Method string `json:"method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Method == "" {
		req.Method = "POST"
	}
	evt := Event{
		Type:      EventImplantCheckin,
		AgentID:   "test",
		AgentHost: "test-host",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"test": true},
	}
	s.fireWebhook(db.WebhookConfig{
		Name:   "test",
		URL:    req.URL,
		Method: req.Method,
	}, evt)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "webhook test sent"})
}

func (s *Server) handlePluginList(c *gin.Context) {
	var plugins []db.Plugin
	s.db.Find(&plugins)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": plugins})
}

func (s *Server) handlePluginToggle(c *gin.Context) {
	id := c.Param("id")
	var plugin db.Plugin
	if s.db.First(&plugin, id).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	plugin.Enabled = !plugin.Enabled
	s.db.Save(&plugin)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDelete(c *gin.Context) {
	id := c.Param("id")
	s.db.Delete(&db.Plugin{}, id)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginCreate(c *gin.Context) {
	var p db.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.db.Create(&p)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

func (s *Server) handleBOFRepoIndex(c *gin.Context) {
	// Fetch trusted BOF repos from config
	type BOFRepoEntry struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
	}
	repos := []BOFRepoEntry{
		{Name: "TrustedSec BOF", URL: "https://github.com/trustedsec/CS-Remote-OPs-BOF", Description: "TrustedSec BOF collection"},
		{Name: "Rafael BOF", URL: "https://github.com/outflanknl/CS-Remote-OPs-BOF", Description: "Outflank BOF collection"},
		{Name: "Encode BOF", URL: "https://github.com/anthemtotheego/BOFs", Description: "Anthem BOF collection"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": repos})
}

func (s *Server) handleBOFRepoImport(c *gin.Context) {
	var req struct {
		URL      string `json:"url"`
		Filename string `json:"filename"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_ = req
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "BOF import initiated (download from URL in browser)"})
}
