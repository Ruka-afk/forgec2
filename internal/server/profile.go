package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// applyMalleableProfile modifies the beacon response and HTTP context
// according to the configured malleable C2 profile.
func (s *Server) applyMalleableProfile(c *gin.Context, resp *beaconResponse) {
	mp := s.cfg.Malleable
	if !mp.Enabled {
		return
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

	// Set custom headers
	for k, v := range mp.Headers {
		c.Header(k, v)
	}

	// Override Content-Type
	ct := mp.ContentType
	if ct == "" {
		ct = "application/json"
	}
	c.Header("Content-Type", ct)

	// Suppress default gin JSON content-type by writing raw bytes
	c.Status(statusCode)
	c.Writer.WriteString(wrapped)
}

// BuildMalleableInfo returns a human-readable summary of the active profile.
func (s *Server) MalleableInfo() string {
	mp := s.cfg.Malleable
	if !mp.Enabled {
		return "Malleable C2 profile is disabled"
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
