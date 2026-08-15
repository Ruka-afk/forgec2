package server

import (
	"bytes"
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

// stripMalleableRequest removes the request-side malleable prepend/append that
// the agent wraps around its OUTGOING beacon body (see wrapMalleableRequest on
// the agent). Mirrors stripMalleableWrapping but uses the request-side tokens
// from the server's malleable config. The operation is the inverse of the
// agent's wrap, so the enclosed JSON envelope is recovered unchanged. When no
// request-side transform is configured the body is returned untouched.
func (s *Server) stripMalleableRequest(raw []byte) []byte {
	s.configMu.RLock()
	prepend := s.cfg.Malleable.RequestPrepend
	appendStr := s.cfg.Malleable.RequestAppend
	s.configMu.RUnlock()

	switch {
	case prepend == "" && appendStr == "":
		return raw
	case prepend == "":
		return bytes.TrimSuffix(raw, []byte(appendStr))
	case appendStr == "":
		return bytes.TrimPrefix(raw, []byte(prepend))
	default:
		raw = bytes.TrimPrefix(raw, []byte(prepend))
		return bytes.TrimSuffix(raw, []byte(appendStr))
	}
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
