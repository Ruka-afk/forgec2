package server

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/url"

	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleImportProfile(c *gin.Context) {
	file, err := c.FormFile("profile")
	if err != nil {
		respondError(c, http.StatusBadRequest, "profile file required")
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

	s.renderPageOrJSON(c, data)
}
