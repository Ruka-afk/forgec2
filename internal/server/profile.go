package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/forgec2/forgec2/internal/malleable"
	"github.com/gin-gonic/gin"
)

func (s *Server) applyMalleableProfile(c *gin.Context, resp *beaconResponse) {
	mp := s.cfg.Malleable
	if !mp.Enabled {
		return
	}

	// Apply named profile preset if set
	if mp.ProfileName != "" {
		presets := malleable.PredefinedProfiles()
		if profile, ok := presets[mp.ProfileName]; ok {
			s.applyProfilePreset(c, resp, profile)
			return
		}
	}

	statusCode := mp.StatusCode
	if statusCode < 100 || statusCode > 599 {
		statusCode = 200
	}

	body, err := json.Marshal(resp)
	if err != nil {
		return
	}

	wrapped := string(body)
	if mp.Prepend != "" {
		wrapped = mp.Prepend + wrapped
	}
	if mp.Append != "" {
		wrapped = wrapped + mp.Append
	}

	for k, v := range mp.Headers {
		c.Header(k, v)
	}

	ct := mp.ContentType
	if ct == "" {
		ct = "application/json"
	}
	c.Header("Content-Type", ct)

	c.Status(statusCode)
	c.Writer.WriteString(wrapped)
}

func (s *Server) applyProfilePreset(c *gin.Context, resp *beaconResponse, profile *malleable.Profile) {
	body, err := json.Marshal(resp)
	if err != nil {
		return
	}

	// Apply output transforms
	if profile.HttpPost.Output != nil {
		transformed, err := profile.HttpPost.Output.Apply(body, true)
		if err == nil {
			body = transformed
		}
	}

	wrapped := string(body)

	for k, v := range profile.HttpPost.Headers {
		c.Header(k, v)
	}

	ct := profile.HttpPost.Headers["Content-Type"]
	if ct == "" {
		ct = "text/plain"
	}
	c.Header("Content-Type", ct)

	c.Status(200)
	c.Writer.WriteString(wrapped)
}

func (s *Server) MalleableInfo() string {
	mp := s.cfg.Malleable
	if !mp.Enabled {
		return "Malleable C2 profile is disabled"
	}

	if mp.ProfileName != "" {
		return fmt.Sprintf("Profile preset: %s\n", mp.ProfileName)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Status: %d | Content-Type: %s\n", mp.StatusCode, mp.ContentType))
	if mp.Prepend != "" {
		b.WriteString(fmt.Sprintf("Prepend: %d bytes\n", len(mp.Prepend)))
	}
	if mp.Append != "" {
		b.WriteString(fmt.Sprintf("Append: %d bytes\n", len(mp.Append)))
	}
	for k, v := range mp.Headers {
		b.WriteString(fmt.Sprintf("Header: %s: %s\n", k, v))
	}
	return b.String()
}
