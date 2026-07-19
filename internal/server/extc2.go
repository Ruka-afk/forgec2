package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

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

func (s *Server) handleExtC2Receive(c *gin.Context) {
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

	// Parse as BeaconRequest
	var br beaconRequest
	if err := json.Unmarshal(raw, &br); err != nil {
		c.JSON(http.StatusBadRequest, extC2ReceiveResponse{Error: sanitizeError(err, "JSON decode")})
		return
	}
	if req.BeaconID != "" {
		br.UUID = req.BeaconID
	}

	resp := s.processBeacon(br, "")
	respJSON, err := json.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, extC2ReceiveResponse{Error: sanitizeError(err, "Response encoding")})
		return
	}

	c.JSON(http.StatusOK, extC2ReceiveResponse{
		Success: true,
		Data:    base64.StdEncoding.EncodeToString(respJSON),
	})
}

func (s *Server) handleExtC2Send(c *gin.Context) {
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

	slog.Info("External C2 send", "beacon_id", req.BeaconID, "data_len", len(req.Data))
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
	defer func() {
		s.extC2ChannelsMu.Lock()
		delete(s.extC2Channels, channelID)
		s.extC2ChannelsMu.Unlock()
	}()

	slog.Info("External C2 WebSocket connected", "channel_id", channelID)

	// Send task queue reader in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s.extC2TaskMu.Lock()
				tasks := s.extC2TaskQueue[channelID]
				s.extC2TaskQueue[channelID] = nil
				s.extC2TaskMu.Unlock()
				for _, task := range tasks {
					msg, _ := json.Marshal(task)
					ch.mu.Lock()
					conn.WriteMessage(websocket.TextMessage, msg)
					ch.mu.Unlock()
				}
			}
		}
	}()

	// Read loop: receive results from external agent
	for {
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
}

func (s *Server) processExternalC2Result(agentID string, taskID uint, result string) {
	if taskID == 0 {
		return
	}
	s.db.Model(&db.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":     "completed",
		"result":     result,
		"updated_at": time.Now(),
	})
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
	s.extC2TaskQueue[channelID] = append(s.extC2TaskQueue[channelID], task)
}

