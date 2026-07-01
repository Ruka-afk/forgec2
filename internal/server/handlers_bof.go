package server

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleBOFPage(c *gin.Context) {
	stats := s.getNavStats()

	var bofs []db.BOFFile
	s.db.Order("created_at desc").Find(&bofs)

	data := gin.H{
		"Title": "ForgeC2 - BOF Manager",
		"ActiveNav": "bof",
		"BOFFiles":  bofs,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "bof_content", data)
}

func (s *Server) handleBOFUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BOF file (.o) required"})
		return
	}

	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("BOF too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read BOF file"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read BOF data"})
		return
	}

	desc := c.PostForm("description")

	bof := db.BOFFile{
		Name:        file.Filename,
		Data:        data,
		Size:        file.Size,
		Description: desc,
		CreatedBy:   c.GetString("username"),
	}

	if err := s.db.Create(&bof).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save BOF: " + err.Error()})
		return
	}

	slog.Info("BOF uploaded", "name", file.Filename, "size", file.Size, "user", bof.CreatedBy)
	s.LogAuditRecord(c, "bof_upload", "bof", "", fmt.Sprintf("BOF: %s (%d bytes)", file.Filename, file.Size), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "bof": bof})
}

func (s *Server) handleBOFList(c *gin.Context) {
	var bofs []db.BOFFile
	s.db.Order("created_at desc").Find(&bofs)

	results := make([]gin.H, 0, len(bofs))
	for _, b := range bofs {
		results = append(results, gin.H{
			"id":          b.ID,
			"name":        b.Name,
			"size":        b.Size,
			"description": b.Description,
			"created_by":  b.CreatedBy,
			"created_at":  b.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"bofs": results})
}

func (s *Server) handleBOFDelete(c *gin.Context) {
	bofID := c.Param("id")

	var bof db.BOFFile
	if err := s.db.First(&bof, bofID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOF not found"})
		return
	}

	s.db.Delete(&bof)
	slog.Info("BOF deleted", "name", bof.Name, "id", bofID)
	s.LogAuditRecord(c, "bof_delete", "bof", "", fmt.Sprintf("Deleted BOF: %s", bof.Name), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleBOFDownload(c *gin.Context) {
	bofID := c.Param("id")

	var bof db.BOFFile
	if err := s.db.First(&bof, bofID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOF not found"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, bof.Name))
	c.Data(http.StatusOK, "application/octet-stream", bof.Data)
}

func (s *Server) handleBOFRun(c *gin.Context) {
	bofID := c.Param("id")
	agentID := c.PostForm("agent_id")
	args := c.PostForm("args")

	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id required"})
		return
	}

	var bof db.BOFFile
	if err := s.db.First(&bof, bofID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOF not found"})
		return
	}

	if _, ok := s.getAgentOrFail(c, agentID); !ok {
		return
	}

	b64Data := base64.StdEncoding.EncodeToString(bof.Data)

	task, err := s.createTask(agentID, "bof", args, "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("BOF dispatched from library", "agent", agentID, "bof", bof.Name, "args", args)
	s.LogAuditRecord(c, "bof_run", "agent", agentID, fmt.Sprintf("BOF: %s args=%s", bof.Name, args), true, nil)
	s.dispatchTask(c, task, "bof", fmt.Sprintf("BOF: %s args=%s", bof.Name, args))
}

func (s *Server) handleBOFQuickRun(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	file, err := c.FormFile("bof")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bof file (.o) required"})
		return
	}
	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("BOF too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read BOF file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read BOF data"})
		return
	}
	args := c.PostForm("args")
	b64Data := base64.StdEncoding.EncodeToString(data)

	task, err := s.createTask(id, "bof", args, "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("BOF quick execute", "agent", id, "file", file.Filename, "size", len(data), "args", args)
	s.LogAuditRecord(c, "bof_quick", "agent", id, fmt.Sprintf("BOF quick: %s (%d bytes) args=%s", file.Filename, len(data), args), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": fmt.Sprintf("BOF %s dispatched", file.Filename)})
}

func (s *Server) handleBOFRecentResults(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	var limit int
	if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	var tasks []db.Task
	s.db.Where("type = ?", "bof").Preload("Agent").Order("created_at desc").Limit(limit).Find(&tasks)

	results := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		elapsed := ""
		if !t.CreatedAt.IsZero() && t.UpdatedAt.After(t.CreatedAt) {
			d := t.UpdatedAt.Sub(t.CreatedAt)
			if d < time.Minute {
				elapsed = fmt.Sprintf("%ds", int(d.Seconds()))
			} else {
				elapsed = fmt.Sprintf("%dm", int(d.Minutes()))
			}
		}
		results = append(results, gin.H{
			"id":         t.ID,
			"agent_id":   t.AgentID,
			"agent_name": t.Agent.Hostname,
			"args":       t.Command,
			"result":     t.Result,
			"error":      t.Error,
			"status":     t.Status,
			"created_at": t.CreatedAt,
			"elapsed":    elapsed,
		})
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (s *Server) handleBOFEdit(c *gin.Context) {
	bofID := c.Param("id")

	var bof db.BOFFile
	if err := s.db.First(&bof, bofID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "BOF not found"})
		return
	}

	desc := c.PostForm("description")
	if desc != "" {
		bof.Description = desc
	}
	if name := c.PostForm("name"); name != "" {
		bof.Name = name
	}

	if err := s.db.Save(&bof).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update BOF"})
		return
	}

	s.LogAuditRecord(c, "bof_edit", "bof", "", fmt.Sprintf("Edited BOF: %s", bof.Name), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "bof": bof})
}
