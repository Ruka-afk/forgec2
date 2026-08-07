package server

import (
	"net/http"

	"github.com/forgec2/forgec2/internal/malleable"
	"github.com/gin-gonic/gin"
)

func (s *Server) applyMalleableProfile(c *gin.Context, body []byte) {
	s.configMu.RLock()
	mp := s.cfg.Malleable
	s.configMu.RUnlock()
	if !mp.Enabled {
		c.Data(http.StatusOK, "application/json", body)
		return
	}

	// Apply named profile preset if set
	if mp.ProfileName != "" {
		presets := malleable.PredefinedProfiles()
		if profile, ok := presets[mp.ProfileName]; ok {
			s.applyProfilePreset(c, body, profile)
			return
		}
	}

	statusCode := mp.StatusCode
	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusOK
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

func (s *Server) applyProfilePreset(c *gin.Context, body []byte, profile *malleable.Profile) {
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

	c.Status(http.StatusOK)
	c.Writer.WriteString(wrapped)
}
