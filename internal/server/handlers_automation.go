package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// validateExternalURL guards server-initiated fetches against SSRF. It accepts
// http(s) URLs whose resolved destination is a public IP: loopback, RFC1918/
// ULA-private, link-local, multicast, unspecified and IPv4-mapped-v6 addresses
// are all rejected. Resolving the hostname covers encoded/decimal-octet
// literals and DNS rebinding on the lookup the server itself performs.
func validateExternalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("URL has no host")
	}
	var addrs []netip.Addr
	if ip, perr := netip.ParseAddr(host); perr == nil {
		addrs = append(addrs, ip.Unmap())
	} else {
		resolved, lerr := net.LookupIP(host)
		if lerr != nil {
			return fmt.Errorf("host resolution failed: %w", lerr)
		}
		for _, ip := range resolved {
			if a, ok := netip.AddrFromSlice(ip); ok {
				addrs = append(addrs, a.Unmap())
			}
		}
		if len(addrs) == 0 {
			return errors.New("host resolved to no addresses")
		}
	}
	for _, a := range addrs {
		if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
			a.IsLinkLocalMulticast() || a.IsUnspecified() || a.IsMulticast() {
			return fmt.Errorf("blocked non-public address %s", a)
		}
	}
	return nil
}

// ssrfSafeClient returns an HTTP client that validates every redirect hop
// before following it, so a public fetch cannot pivot into internal targets.
func ssrfSafeClient(base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}
	c := &http.Client{
		Timeout:   base.Timeout,
		Transport: base.Transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return validateExternalURL(req.URL.String())
		},
	}
	return c
}

func (s *Server) handleAutomationPage(c *gin.Context) {
	rules := s.loadAutomationRules()
	var webhooks []db.WebhookConfig
	if err := s.db.Limit(500).Find(&webhooks).Error; err != nil {
		slog.Error("Failed to list webhooks for automation page", "err", err)
	}
	s.renderPageOrJSON(c, gin.H{
		"Title":     "Automation",
		"ActiveNav": "automation",
		"Rules":     rules,
		"Webhooks":  webhooks,
	})
}

func (s *Server) handleListAutomationRules(c *gin.Context) {
	p := parsePagination(c, 50, 200)
	var total int64
	if err := s.db.Model(&db.AutomationRule{}).Count(&total).Error; err != nil {
		slog.Error("Failed to count automation rules", "err", err)
	}
	var dbRules []db.AutomationRule
	if err := s.db.Offset(p.Offset).Limit(p.PageSize).Find(&dbRules).Error; err != nil {
		slog.Error("Failed to list automation rules", "err", err)
	}
	var rules []AutomationRule
	for _, dr := range dbRules {
		var conditions []RuleCondition
		if dr.Conditions != "" {
			if err := json.Unmarshal([]byte(dr.Conditions), &conditions); err != nil {
				slog.Warn("Automation: unmarshal conditions", "rule", dr.Name, "error", err)
			}
		}
		var actions []RuleAction
		if dr.Actions != "" {
			if err := json.Unmarshal([]byte(dr.Actions), &actions); err != nil {
				slog.Warn("Automation: unmarshal actions", "rule", dr.Name, "error", err)
			}
		}
		rules = append(rules, AutomationRule{
			ID:         dr.ID,
			Name:       dr.Name,
			Enabled:    dr.Enabled,
			EventType:  dr.EventType,
			Conditions: conditions,
			Actions:    actions,
			CreatedAt:  dr.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules, "total": total, "page": p.Page, "page_size": p.PageSize})
}

func (s *Server) handleSaveAutomationRule(c *gin.Context) {
	var rule AutomationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Rule operation"))
		return
	}
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule_%d", time.Now().UnixNano())
	}
	if rule.CreatedAt == "" {
		rule.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if err := s.saveAutomationRule(rule); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Rule operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func (s *Server) handleUpdateAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	var rule AutomationRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Rule operation"))
		return
	}
	rule.ID = ruleID
	if err := s.saveAutomationRule(rule); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Rule operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func (s *Server) handleDeleteAutomationRule(c *gin.Context) {
	ruleID := c.Param("id")
	if err := s.deleteAutomationRule(ruleID); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Rule operation"))
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
	if err := s.db.Limit(500).Find(&webhooks).Error; err != nil {
		slog.Error("Failed to list webhooks", "err", err)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": webhooks})
}

func (s *Server) handleCreateWebhook(c *gin.Context) {
	var wh db.WebhookConfig
	if err := c.ShouldBindJSON(&wh); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Webhook operation"))
		return
	}
	if err := validateWebhookURL(wh.URL); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Webhook operation"))
		return
	}
	if err := s.db.Create(&wh).Error; err != nil {
		slog.Error("Failed to create webhook", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create webhook")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": wh})
}

func (s *Server) handleDeleteWebhook(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.WebhookConfig{}, id).Error; err != nil {
		slog.Error("Failed to delete webhook", "id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to delete webhook")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleTestWebhook(c *gin.Context) {
	var req struct {
		URL    string `json:"url"`
		Method string `json:"method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Webhook operation"))
		return
	}
	if req.Method == "" {
		req.Method = "POST"
	}
	if err := validateWebhookURL(req.URL); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Webhook operation"))
		return
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
		query = query.Where("(name LIKE ? ESCAPE '\\' OR description LIKE ? ESCAPE '\\' OR author LIKE ? ESCAPE '\\')",
			"%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%", "%"+escapeLike(search)+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if pluginType != "" {
		query = query.Where("type = ?", pluginType)
	}

	if err := query.Limit(200).Find(&plugins).Error; err != nil {
		slog.Error("Failed to list marketplace plugins", "err", err)
	}

	s.marketplace.BatchEnrichPlugins(plugins)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": plugins})
}

func (s *Server) handlePluginToggle(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "plugin not found")
		return
	}
	p.Enabled = !p.Enabled
	if err := s.db.Save(p).Error; err != nil {
		slog.Error("Failed to save plugin toggle", "plugin_id", p.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to toggle plugin")
		return
	}
	if p.Name != "" {
		if err := s.pluginManager.SetEnabled(p.Name, p.Enabled); err != nil {
			slog.Error("Automation: failed to set plugin enabled state", "plugin_name", p.Name, "enabled", p.Enabled, "error", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDelete(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, "plugin not found")
		return
	}
	if p.Name != "" {
		if err := s.pluginManager.Unregister(p.Name); err != nil {
			slog.Error("Automation: failed to unregister plugin", "plugin_name", p.Name, "error", err)
		}
	}
	if err := s.db.Delete(p).Error; err != nil {
		slog.Error("Failed to delete plugin", "plugin_id", p.ID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to delete plugin")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginCreate(c *gin.Context) {
	var p db.Plugin
	if err := c.ShouldBindJSON(&p); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Plugin operation"))
		return
	}
	if err := s.db.Create(&p).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	s.tryRegisterPluginFromDisk(p.Name)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": p})
}

func (s *Server) handlePluginsPage(c *gin.Context) {
	s.renderPageOrJSON(c, gin.H{"Title": "Plugin Marketplace", "ActiveNav": "plugins"})
}

func (s *Server) handlePluginGet(c *gin.Context) {
	id := c.Param("id")
	var plugin db.Plugin
	if err := s.db.First(&plugin, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "plugin not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": plugin})
}

func (s *Server) handlePluginRating(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	summary, err := s.marketplace.GetRatingSummary(pluginID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": summary})
}

func (s *Server) handlePluginReviews(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	reviews, err := s.marketplace.GetReviews(pluginID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": reviews})
}

func (s *Server) handlePluginAddReview(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	username, _ := c.Get("user")
	uname, _ := username.(string)

	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Plugin operation"))
		return
	}

	if err := s.marketplace.AddReview(pluginID, uid, uname, req.Rating, req.Comment); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handlePluginRate submits a rating for a plugin.
func (s *Server) handlePluginRate(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	username, _ := c.Get("user")
	uname, _ := username.(string)

	var req struct {
		Rating int `json:"rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Plugin operation"))
		return
	}
	if req.Rating < 1 || req.Rating > 5 {
		respondError(c, http.StatusBadRequest, "rating must be 1-5")
		return
	}

	if err := s.marketplace.AddReview(pluginID, uid, uname, req.Rating, ""); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDependencies(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	deps, err := s.marketplace.GetDependencies(pluginID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": deps})
}

func (s *Server) handlePluginUpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	status, err := s.marketplace.GetUpdateStatus(pluginID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}

	var plugin db.Plugin
	if err := s.db.First(&plugin, pluginID).Error; err != nil {
		respondError(c, http.StatusNotFound, "plugin not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"plugin_id":        status.PluginID,
		"latest_version":   status.LatestVersion,
		"current_version":  plugin.Version,
		"update_available": status.UpdateAvailable,
		"update_url":       status.UpdateURL,
		"release_notes":    status.ReleaseNotes,
		"last_checked_at":  status.LastCheckedAt,
	}})
}

func (s *Server) handlePluginUpdate(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	if err := s.marketplace.UpdatePlugin(pluginID); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Plugin operation"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginExport(c *gin.Context) {
	id := c.Param("id")
	var pluginID uint
	if _, err := fmt.Sscanf(id, "%d", &pluginID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid plugin id")
		return
	}

	data, err := s.marketplace.ExportPlugin(pluginID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=plugin-"+id+".json")
	c.Data(http.StatusOK, "application/json", data)
}

func (s *Server) handlePluginImport(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		respondError(c, http.StatusBadRequest, "no file provided")
		return
	}
	if file.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("file too large (max %d bytes)", MaxUploadSize))
		return
	}

	f, err := file.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Plugin operation"))
		return
	}

	plugin, err := s.marketplace.ImportPlugin(data)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Plugin operation"))
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
	if err := s.db.Limit(200).Find(&plugins).Error; err != nil {
		slog.Error("Failed to list plugins for update summary", "err", err)
	}

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
		"success":         true,
		"available_count": availableCount,
		"total_plugins":   len(plugins),
		"last_checked":    lastChecked,
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
		respondError(c, http.StatusBadRequest, sanitizeError(err, "BOF operation"))
		return
	}
	if req.URL == "" || req.Filename == "" {
		respondError(c, http.StatusBadRequest, "url and filename are required")
		return
	}

	// Validate URL to prevent SSRF: scheme + full IP-space check of the
	// resolved host (loopback/RFC1918/ULA/link-local/multicast all blocked).
	if err := validateExternalURL(req.URL); err != nil {
		respondError(c, http.StatusBadRequest, "url rejected: "+err.Error())
		return
	}

	resp, err := ssrfSafeClient(s.httpClient).Get(req.URL)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "BOF operation"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("download failed: HTTP %d", resp.StatusCode))
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxUploadSize+1))
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "BOF operation"))
		return
	}
	if int64(len(data)) > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("file too large (max %d bytes)", MaxUploadSize))
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
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "BOF operation"))
		return
	}

	s.LogAuditRecord(c, "bof_import", "bof", "", fmt.Sprintf("BOF imported: %s (%d bytes) from %s", name, len(data), req.URL), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Imported %s (%d bytes)", name, len(data)),
		"bof_id":  bof.ID,
	})
}

// bofRatings stores in-memory ratings for BOF repo items
var (
	bofRatings   = map[string]int{}
	bofRatingsMu sync.RWMutex
)

// handleBOFRepoRate rates a BOF repo item.
func (s *Server) handleBOFRepoRate(c *gin.Context) {
	itemID := c.Param("id")
	if itemID == "" {
		respondError(c, http.StatusBadRequest, "item id required")
		return
	}

	var req struct {
		Rating int `json:"rating"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Rating < 0 || req.Rating > 5 {
		respondError(c, http.StatusBadRequest, "rating must be 0-5")
		return
	}

	bofRatingsMu.Lock()
	bofRatings[itemID] = req.Rating
	bofRatingsMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"success": true})
}
