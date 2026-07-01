package server

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAutomationPage(c *gin.Context) {
	rules := s.loadAutomationRules()
	var webhooks []db.WebhookConfig
	s.db.Find(&webhooks)
	s.renderPage(c, "automation_content", gin.H{
		"Title": "Automation",
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
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	if rule.CreatedAt == "" {
		rule.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if err := s.saveAutomationRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func (s *Server) handleUpdateAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	var rule AutomationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = ruleID
	if err := s.saveAutomationRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func (s *Server) handleDeleteAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	if err := s.deleteAutomationRule(ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleToggleAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	rules := s.loadAutomationRules()
	for i, r := range rules {
		if r.ID == ruleID {
			rules[i].Enabled = !r.Enabled
			s.saveAutomationRule(rules[i])
			break
		}
	}
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
	search := c.Query("search")
	category := c.Query("category")
	pluginType := c.Query("type")

	var plugins []db.Plugin
	query := s.db.Model(&db.Plugin{})

	if search != "" {
		query = query.Where("name LIKE ? OR description LIKE ? OR author LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if pluginType != "" {
		query = query.Where("type = ?", pluginType)
	}

	query.Find(&plugins)

	for i := range plugins {
		summary, _ := s.marketplace.GetRatingSummary(plugins[i].ID)
		plugins[i].RatingOverall = summary.Overall
		plugins[i].RatingCount = summary.Count

		status, _ := s.marketplace.GetUpdateStatus(plugins[i].ID)
		plugins[i].UpdateAvailable = status.UpdateAvailable
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": plugins})
}

func (s *Server) handlePluginToggle(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	p.Enabled = !p.Enabled
	s.db.Save(p)
	if p.Name != "" {
		_ = s.pluginManager.SetEnabled(p.Name, p.Enabled)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDelete(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	if p.Name != "" {
		_ = s.pluginManager.Unregister(p.Name)
	}
	s.db.Delete(p)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginCreate(c *gin.Context) {
	var p db.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.db.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.tryRegisterPluginFromDisk(p.Name)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

func (s *Server) handlePluginsPage(c *gin.Context) {
	s.renderPage(c, "plugins_content", gin.H{"Title": "Plugin Marketplace", "ActiveNav": "plugins"})
}

func (s *Server) handlePluginGet(c *gin.Context) {
	id := c.Param("id")
	var plugin db.Plugin
	if err := s.db.First(&plugin, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "plugin not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": plugin})
}

func (s *Server) handlePluginRating(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	summary, err := s.marketplace.GetRatingSummary(pluginID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

func (s *Server) handlePluginReviews(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	reviews, err := s.marketplace.GetReviews(pluginID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": reviews})
}

func (s *Server) handlePluginAddReview(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	username, _ := c.Get("user")
	uname, _ := username.(string)

	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := s.marketplace.AddReview(pluginID, uid, uname, req.Rating, req.Comment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDependencies(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	deps, err := s.marketplace.GetDependencies(pluginID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": deps})
}

func (s *Server) handlePluginUpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	status, err := s.marketplace.GetUpdateStatus(pluginID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	var plugin db.Plugin
	s.db.First(&plugin, pluginID)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"plugin_id":       status.PluginID,
		"latest_version":  status.LatestVersion,
		"current_version": plugin.Version,
		"update_available": status.UpdateAvailable,
		"update_url":      status.UpdateURL,
		"release_notes":   status.ReleaseNotes,
		"last_checked_at": status.LastCheckedAt,
	}})
}

func (s *Server) handlePluginUpdate(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	if err := s.marketplace.UpdatePlugin(pluginID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginExport(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	fmt.Sscanf(id, "%d", &pluginID)

	data, err := s.marketplace.ExportPlugin(pluginID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=plugin-"+id+".json")
	c.Data(http.StatusOK, "application/json", data)
}

func (s *Server) handlePluginImport(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "no file provided"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	plugin, err := s.marketplace.ImportPlugin(data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	s.tryRegisterPluginFromDisk(plugin.Name)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": plugin})
}

func (s *Server) handlePluginCheckUpdates(c *gin.Context) {
	s.marketplace.CheckAllUpdates()
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginUpdateSummary(c *gin.Context) {
	var plugins []db.Plugin
	s.db.Find(&plugins)

	var availableCount int
	var lastChecked time.Time

	for _, p := range plugins {
		status, err := s.marketplace.GetUpdateStatus(p.ID)
		if err == nil {
			if status.UpdateAvailable {
				availableCount++
			}
			if status.LastCheckedAt.After(lastChecked) {
				lastChecked = status.LastCheckedAt
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"available_count":  availableCount,
		"total_plugins":    len(plugins),
		"last_checked":     lastChecked,
	})
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
	if req.URL == "" || req.Filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url and filename are required"})
		return
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "download failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("download failed: HTTP %d", resp.StatusCode)})
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxUploadSize+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read body: " + err.Error()})
		return
	}
	if int64(len(data)) > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file too large (max %d bytes)", MaxUploadSize)})
		return
	}

	name := req.Filename
	if !strings.HasSuffix(strings.ToLower(name), ".o") {
		name += ".o"
	}

	bof := db.BOFFile{
		Name:        name,
		Data:        data,
		Size:        int64(len(data)),
		Description: "Imported from " + req.URL,
		CreatedBy:   c.GetString("username"),
	}
	if err := s.db.Create(&bof).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save BOF: " + err.Error()})
		return
	}

	s.LogAuditRecord(c, "bof_import", "bof", "", fmt.Sprintf("BOF imported: %s (%d bytes) from %s", name, len(data), req.URL), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Imported %s (%d bytes)", name, len(data)),
		"bof_id":  bof.ID,
	})
}
