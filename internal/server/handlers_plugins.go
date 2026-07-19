package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/gin-gonic/gin"
)

func (s *Server) resolvePluginRecord(id string) (*db.Plugin, error) {
	var p db.Plugin
	if err := s.db.First(&p, id).Error; err == nil {
		return &p, nil
	}
	if err := s.db.Where("name = ?", id).First(&p).Error; err == nil {
		return &p, nil
	}
	return nil, fmt.Errorf("plugin not found")
}

func (s *Server) tryRegisterPluginFromDisk(name string) {
	if name == "" || s.pluginManager == nil {
		return
	}
	manifestPath := filepath.Join(s.pluginManager.PluginDir(name), "manifest.yaml")
	manifest, err := plugin.LoadManifest(manifestPath)
	if err != nil {
		return
	}
	if err := s.pluginManager.Register(manifest); err != nil {
		slog.Error("failed to register plugin", "name", name, "error", err)
	}
}

func (s *Server) handlePluginExecuteInfo(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	runtime, err := s.pluginManager.Get(p.Name)
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}

	manifest, ok := pluginManifest(runtime)
	if !ok {
		respondError(c, http.StatusInternalServerError, "plugin has no manifest")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"plugin": gin.H{
			"id":          p.ID,
			"name":        manifest.Name,
			"version":     manifest.Version,
			"type":        manifest.Type,
			"description": manifest.Description,
			"author":      manifest.Author,
			"category":    manifest.Category,
			"params":      manifest.Params,
			"events":      manifest.Events,
			"config":      manifest.Config,
		},
	})
}

func (s *Server) handlePluginExecute(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		AgentID string                 `json:"agent_id"`
		Params  map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := s.pluginManager.ExecuteCommand(c.Request.Context(), p.Name, req.AgentID, req.Params)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "result": result})
}

func (s *Server) handlePluginReport(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Params map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	report, err := s.pluginManager.GenerateReport(c.Request.Context(), p.Name, req.Params)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"report": gin.H{
			"title":   report.Title,
			"format":  report.Format,
			"content": string(report.Content),
		},
	})
}

func (s *Server) handlePluginInstall(c *gin.Context) {
	manifestFile, err := c.FormFile("manifest")
	if err != nil {
		respondError(c, http.StatusBadRequest, "manifest file is required")
		return
	}

	mf, err := manifestFile.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open manifest")
		return
	}
	defer mf.Close()

	manifestData, err := io.ReadAll(mf)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read manifest")
		return
	}

	var manifest plugin.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		respondError(c, http.StatusBadRequest, "invalid manifest json: "+err.Error())
		return
	}

	// Sanitize plugin name and entry to prevent path traversal
	if strings.ContainsAny(manifest.Name, `/\..`) || manifest.Name == "" {
		respondError(c, http.StatusBadRequest, "invalid plugin name: must not contain path separators or dots")
		return
	}
	if manifest.Entry != "" && strings.ContainsAny(manifest.Entry, `/\..`) {
		respondError(c, http.StatusBadRequest, "invalid entry point: must not contain path separators or dots")
		return
	}

	pluginDir := s.pluginManager.PluginDir(manifest.Name)
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create plugin directory")
		return
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), manifestData, 0600); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to write manifest")
		return
	}

	scriptFile, err := c.FormFile("script")
	if err == nil {
		sf, err := scriptFile.Open()
		if err != nil {
			respondError(c, http.StatusInternalServerError, "failed to open script")
			return
		}
		defer sf.Close()
		scriptData, err := io.ReadAll(sf)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "failed to read script")
			return
		}
		if err := os.WriteFile(filepath.Join(pluginDir, manifest.Entry), scriptData, 0600); err != nil {
			respondError(c, http.StatusInternalServerError, "failed to write script")
			return
		}
	}

	if err := s.pluginManager.Register(&manifest); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.db.Model(&db.Plugin{}).Where("name = ?", manifest.Name).Updates(map[string]interface{}{
		"enabled": true,
		"version": manifest.Version,
	}).Error; err != nil {
		slog.Error("Failed to update plugin record after install", "plugin", manifest.Name, "err", err)
	}

	s.LogAuditRecord(c, "plugin_install", "plugin", manifest.Name, fmt.Sprintf("installed plugin %s %s", manifest.Name, manifest.Version), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "plugin": manifest.Name})
}

func (s *Server) handlePluginEnable(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	if err := s.pluginManager.SetEnabled(p.Name, true); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Model(p).Update("enabled", true).Error; err != nil {
		slog.Error("Failed to persist plugin enable", "plugin", p.Name, "err", err)
	}
	s.LogAuditRecord(c, "plugin_enable", "plugin", p.Name, "enabled plugin", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDisable(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	if err := s.pluginManager.SetEnabled(p.Name, false); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.db.Model(p).Update("enabled", false).Error; err != nil {
		slog.Error("Failed to persist plugin disable", "plugin", p.Name, "err", err)
	}
	s.LogAuditRecord(c, "plugin_disable", "plugin", p.Name, "disabled plugin", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// pluginManifest extracts the manifest from an external plugin wrapper.
func pluginManifest(p plugin.Plugin) (*plugin.Manifest, bool) {
	type manifestHolder interface {
		Manifest() *plugin.Manifest
	}
	if mh, ok := p.(manifestHolder); ok {
		return mh.Manifest(), true
	}
	return nil, false
}
