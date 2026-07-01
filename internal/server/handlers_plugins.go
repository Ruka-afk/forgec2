package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

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
	_ = s.pluginManager.Register(manifest)
}

func (s *Server) handlePluginExecuteInfo(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	runtime, err := s.pluginManager.Get(p.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}

	manifest, ok := pluginManifest(runtime)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "plugin has no manifest"})
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
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	var req struct {
		AgentID string                 `json:"agent_id"`
		Params  map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	result, err := s.pluginManager.ExecuteCommand(c.Request.Context(), p.Name, req.AgentID, req.Params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "result": result})
}

func (s *Server) handlePluginReport(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	var req struct {
		Params map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	report, err := s.pluginManager.GenerateReport(c.Request.Context(), p.Name, req.Params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
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
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "manifest file is required"})
		return
	}

	mf, err := manifestFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to open manifest"})
		return
	}
	defer mf.Close()

	manifestData, err := io.ReadAll(mf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to read manifest"})
		return
	}

	var manifest plugin.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid manifest json: " + err.Error()})
		return
	}

	pluginDir := s.pluginManager.PluginDir(manifest.Name)
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create plugin directory"})
		return
	}

	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), manifestData, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to write manifest"})
		return
	}

	scriptFile, err := c.FormFile("script")
	if err == nil {
		sf, err := scriptFile.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to open script"})
			return
		}
		defer sf.Close()
		scriptData, err := io.ReadAll(sf)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to read script"})
			return
		}
		if err := os.WriteFile(filepath.Join(pluginDir, manifest.Entry), scriptData, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to write script"})
			return
		}
	}

	if err := s.pluginManager.Register(&manifest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	s.db.Model(&db.Plugin{}).Where("name = ?", manifest.Name).Updates(map[string]interface{}{
		"enabled": true,
		"version": manifest.Version,
	})

	s.LogAuditRecord(c, "plugin_install", "plugin", manifest.Name, fmt.Sprintf("installed plugin %s %s", manifest.Name, manifest.Version), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "plugin": manifest.Name})
}

func (s *Server) handlePluginEnable(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := s.pluginManager.SetEnabled(p.Name, true); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	s.db.Model(p).Update("enabled", true)
	s.LogAuditRecord(c, "plugin_enable", "plugin", p.Name, "enabled plugin", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handlePluginDisable(c *gin.Context) {
	p, err := s.resolvePluginRecord(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	if err := s.pluginManager.SetEnabled(p.Name, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	s.db.Model(p).Update("enabled", false)
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