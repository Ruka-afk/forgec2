package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/forgec2/forgec2/internal/malleable"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleImportProfile(c *gin.Context) {
	file, err := c.FormFile("profile")
	if err != nil {
		respondError(c, http.StatusBadRequest, "profile file required")
		return
	}
	if file.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("profile file too large (max %d bytes)", MaxUploadSize))
		return
	}
	f, err := file.Open()
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to open profile file")
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to read profile file")
		return
	}
	profile, err := payload.SaveImportedProfile(s.implantDataDir(), raw)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Profile import"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "profile": profile})
}

func (s *Server) handleListProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "profiles": payload.ListProfilePresets(s.implantDataDir())})
}

// handleSaveProfile persists a profile JSON (create or overwrite a custom
// profile in data/profiles/) so editor/duplicate changes survive restarts
// instead of living only in the browser tab.
func (s *Server) handleSaveProfile(c *gin.Context) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, MaxUploadSize+1))
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to read request body")
		return
	}
	if len(raw) > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("profile too large (max %d bytes)", MaxUploadSize))
		return
	}
	profile, err := payload.SaveImportedProfile(s.implantDataDir(), raw)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Profile save"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "profile": profile})
}

// handleImportProfileText accepts pasted profile content (JSON or CS text).
// Body: {name, format: auto|json|cs, content}. CS text uses name when the
// pasted text carries no name.
func (s *Server) handleImportProfileText(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Format  string `json:"format"`
		Content string `json:"content"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize)
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "name, format and content are required")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(c, http.StatusBadRequest, "content is required")
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "auto"
	}
	raw := []byte(content)
	if format == "cs" || (format == "auto" && looksLikeCS(content)) {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "imported"
		}
		v2, err := malleable.ParseCSFull(name, content)
		if err != nil {
			respondError(c, http.StatusBadRequest, sanitizeError(err, "CS parse"))
			return
		}
		if req.Name != "" {
			v2.Name = strings.TrimSpace(req.Name)
		}
		out, _ := json.Marshal(v2)
		raw = out
	}
	profile, err := payload.SaveImportedProfile(s.implantDataDir(), raw)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Profile import"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "profile": profile})
}

func looksLikeCS(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "http-get") || strings.Contains(low, "http-post") || strings.Contains(low, "http-config")
}

// handleValidateProfile dry-runs a profile without saving.
// Body: {name, format: auto|json|cs, content} -> {valid, profile(v2), wire, warnings}.
func (s *Server) handleValidateProfile(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Format  string `json:"format"`
		Content string `json:"content"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize)
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "name, format and content are required")
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(c, http.StatusBadRequest, "content is required")
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "auto"
	}
	var v2 *malleable.ProfileV2
	var warnings []string
	if format == "cs" || (format == "auto" && looksLikeCS(content)) {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "imported"
		}
		p, err := malleable.ParseCSFull(name, content)
		if err != nil {
			warnings = append(warnings, err.Error())
		}
		v2 = p
		if req.Name != "" && v2 != nil {
			v2.Name = strings.TrimSpace(req.Name)
		}
	} else {
		p, err := malleable.MigrateProfileJSON([]byte(content), strings.TrimSpace(req.Name))
		if err != nil {
			respondError(c, http.StatusBadRequest, sanitizeError(err, "Profile validation"))
			return
		}
		v2 = p
	}
	if v2 == nil {
		respondError(c, http.StatusBadRequest, "unable to parse profile")
		return
	}
	if err := malleable.ValidateProfileV2(v2); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Profile validation"))
		return
	}
	// Round-trip sample through the ServerOutput chain.
	sample := `{"type":"beacon","id":"abc"}`
	encoded := sample
	if len(v2.ServerOutput) > 0 {
		tb := &malleable.TransformBlock{}
		for _, st := range v2.ServerOutput {
			tb.Transforms = append(tb.Transforms, malleable.Transform{Type: st.Type, Value: st.Value})
		}
		if enc, err := tb.Apply([]byte(sample), true); err == nil {
			encoded = string(enc)
			if dec, err := tb.Apply(enc, false); err != nil || string(dec) != sample {
				warnings = append(warnings, "server_output chain does not round-trip")
			}
		} else {
			warnings = append(warnings, "server_output encode failed: "+err.Error())
		}
	}
	// Round-trip each placement chain the same way.
	for _, pl := range v2.Placements {
		tb := &malleable.TransformBlock{}
		for _, st := range malleable.ParseWire(pl.Chain) {
			tb.Transforms = append(tb.Transforms, malleable.Transform{Type: st.Type, Value: st.Value})
		}
		if len(tb.Transforms) == 0 {
			continue
		}
		enc, err := tb.Apply([]byte(sample), true)
		if err != nil {
			warnings = append(warnings, "placement "+pl.Target+" encode failed: "+err.Error())
			continue
		}
		if dec, err := tb.Apply(enc, false); err != nil || string(dec) != sample {
			warnings = append(warnings, "placement "+pl.Target+" chain does not round-trip")
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"profile": v2,
		"wire": gin.H{
			"server_output":   malleable.StepsToWire(v2.ServerOutput),
			"client_metadata": malleable.StepsToWire(v2.ClientMetadata),
			"client_id":       malleable.StepsToWire(v2.ClientID),
		},
		"sample":   gin.H{"input": sample, "encoded": encoded},
		"warnings": warnings,
	}})
}

func (s *Server) handleDeleteProfile(c *gin.Context) {
	name, _ := url.PathUnescape(c.Param("name"))
	if name == "" || name == "default" {
		respondError(c, http.StatusBadRequest, "cannot delete default profile")
		return
	}
	if err := payload.DeleteProfile(s.implantDataDir(), name); err != nil {
		respondError(c, http.StatusNotFound, sanitizeError(err, "Profile deletion"))
		return
	}
	respond(c, gin.H{"success": true})
}

func (s *Server) handleGeneratePage(c *gin.Context) {
	listeners := s.getListeners()
	presetsJSON, ok := marshalJSONSafe(payload.ListProfilePresets(s.implantDataDir()))
	if !ok {
		respondError(c, http.StatusInternalServerError, "failed to marshal presets")
		return
	}

	stats := s.getNavStats(c)
	data := gin.H{
		"Title":              "ForgeC2 - Generate Agent",
		"ActiveNav":          "generate",
		"DefaultInt":         s.cfg.Implant.DefaultInterval,
		"DefaultJitter":      s.cfg.Implant.DefaultJitter,
		"DefaultUA":          s.cfg.Implant.DefaultUA,
		"DefaultSkipTLS":     s.cfg.Implant.DefaultSkipTLS,
		"Listeners":          listeners,
		"ProfilePresetsJSON": template.JS(strings.ReplaceAll(string(presetsJSON), "</script>", "<\\/script>")),
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}
