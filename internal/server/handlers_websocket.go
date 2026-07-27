package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type batchedMessage struct {
	Messages []json.RawMessage `json:"messages"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  WSReadBufSize,
	WriteBufferSize: WSWriteBufSize,
	// Agent beacon WebSocket — agents are non-browser clients (no Origin header).
	// Accept empty origin unconditionally; if Origin IS present (browser), restrict.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		originHost := u.Hostname()
		return originHost == "localhost" || originHost == "127.0.0.1" || originHost == "::1"
	},
}

// WebSocketBeacon represents an active WebSocket beacon connection
type WebSocketBeacon struct {
	Conn       *websocket.Conn
	AgentID    string
	LastSeen   time.Time
	Send       chan []byte
	BatchQueue [][]byte
	BatchMutex sync.Mutex
	BatchTimer *time.Timer
	closeOnce  sync.Once
	compress   bool
}

// WebSocketHub manages all active WebSocket beacon connections
type WebSocketHub struct {
	beacons map[string]*WebSocketBeacon
	mu      sync.RWMutex
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		beacons: make(map[string]*WebSocketBeacon),
	}
}

func (h *WebSocketHub) Register(agentID string, beacon *WebSocketBeacon) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.beacons[agentID] = beacon
}

func (h *WebSocketHub) Unregister(agentID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if beacon, exists := h.beacons[agentID]; exists {
		beacon.closeOnce.Do(func() {
			close(beacon.Send)
		})
		delete(h.beacons, agentID)
	}
}

func (h *WebSocketHub) Get(agentID string) *WebSocketBeacon {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.beacons[agentID]
}

func (h *WebSocketHub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, beacon := range h.beacons {
		select {
		case beacon.Send <- data:
		default:
			// Channel full, skip
		}
	}
}

// handleWebSocketBeacon handles WebSocket beacon connections
func (s *Server) handleWebSocketBeacon(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	agentID := uuid.New().String()

	slog.Info("WebSocket beacon connected", "agent_id", agentID, "remote_addr", conn.RemoteAddr())

	beacon := &WebSocketBeacon{
		Conn:       conn,
		AgentID:    agentID,
		LastSeen:   time.Now(),
		Send:       make(chan []byte, 256),
		BatchQueue: make([][]byte, 0, 16),
		compress:   true,
	}

	// Initialize WebSocket hub if not exists
	s.wsHubOnce.Do(func() {
		if s.wsHub == nil {
			s.wsHub = NewWebSocketHub()
		}
	})
	s.wsHub.Register(agentID, beacon)

	// Start read and write pumps
	go s.wsWritePump(beacon)
	go s.wsReadPump(beacon)
}

func (s *Server) wsWritePump(beacon *WebSocketBeacon) {
	defer func() {
		beacon.BatchMutex.Lock()
		if beacon.BatchTimer != nil {
			beacon.BatchTimer.Stop()
		}
		beacon.BatchMutex.Unlock()
		beacon.Conn.Close()
		s.wsHub.Unregister(beacon.AgentID)
	}()

	ticker := time.NewTicker(BeaconPingInterval)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-beacon.Send:
			if !ok {
				beacon.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			s.enqueueBatchMessage(beacon, message)

		case <-ticker.C:
			beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
			if err := beacon.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("WebSocket ping error", "agent_id", beacon.AgentID, "error", err)
				return
			}
		}
	}
}

func (s *Server) enqueueBatchMessage(beacon *WebSocketBeacon, message []byte) {
	beacon.BatchMutex.Lock()
	beacon.BatchQueue = append(beacon.BatchQueue, message)
	currentLen := len(beacon.BatchQueue)
	beacon.BatchMutex.Unlock()

	if currentLen == 1 {
		beacon.BatchMutex.Lock()
		beacon.BatchTimer = time.AfterFunc(BatchFlushDelay, func() {
			s.flushBatchQueue(beacon)
		})
		beacon.BatchMutex.Unlock()
	} else if currentLen >= BatchFlushThreshold {
		beacon.BatchMutex.Lock()
		if beacon.BatchTimer != nil {
			beacon.BatchTimer.Stop()
		}
		beacon.BatchMutex.Unlock()
		s.flushBatchQueue(beacon)
	}
}

func (s *Server) flushBatchQueue(beacon *WebSocketBeacon) {
	beacon.BatchMutex.Lock()
	if len(beacon.BatchQueue) == 0 {
		beacon.BatchMutex.Unlock()
		return
	}

	var data []byte
	if len(beacon.BatchQueue) == 1 {
		data = beacon.BatchQueue[0]
	} else {
		batched := batchedMessage{
			Messages: make([]json.RawMessage, len(beacon.BatchQueue)),
		}
		for i, msg := range beacon.BatchQueue {
			batched.Messages[i] = json.RawMessage(msg)
		}
		var err error
		data, err = json.Marshal(batched)
		if err != nil {
			slog.Error("WebSocket batch marshal error", "agent_id", beacon.AgentID, "error", err)
			beacon.BatchQueue = beacon.BatchQueue[:0]
			beacon.BatchMutex.Unlock()
			return
		}
	}

	beacon.BatchQueue = beacon.BatchQueue[:0]
	beacon.BatchMutex.Unlock()

	if beacon.compress && len(data) > 256 {
		var err error
		data, err = gzipCompress(data)
		if err != nil {
			slog.Error("WebSocket gzip compress error", "agent_id", beacon.AgentID, "error", err)
			return
		}
		s.sendCompressedMessage(beacon, data)
	} else {
		s.sendTextMessage(beacon, data)
	}
}

func (s *Server) sendTextMessage(beacon *WebSocketBeacon, data []byte) {
	beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
	w, err := beacon.Conn.NextWriter(websocket.TextMessage)
	if err != nil {
		slog.Error("WebSocket write error", "agent_id", beacon.AgentID, "error", err)
		return
	}
	w.Write(data)
	if err := w.Close(); err != nil {
		slog.Error("WebSocket close writer error", "agent_id", beacon.AgentID, "error", err)
	}
}

func (s *Server) sendCompressedMessage(beacon *WebSocketBeacon, data []byte) {
	beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
	w, err := beacon.Conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		slog.Error("WebSocket compressed write error", "agent_id", beacon.AgentID, "error", err)
		return
	}
	w.Write(data)
	if err := w.Close(); err != nil {
		slog.Error("WebSocket compressed close writer error", "agent_id", beacon.AgentID, "error", err)
	}
}

func (s *Server) wsReadPump(beacon *WebSocketBeacon) {
	defer func() {
		s.wsHub.Unregister(beacon.AgentID)
		beacon.Conn.Close()
		s.handleWSBeaconDisconnect(beacon.AgentID)
	}()

	beacon.Conn.SetReadLimit(WSMaxMessageSize)
	beacon.Conn.SetReadDeadline(time.Now().Add(BeaconReadDeadline))
	beacon.Conn.SetPongHandler(func(string) error {
		beacon.Conn.SetReadDeadline(time.Now().Add(BeaconReadDeadline))
		beacon.LastSeen = time.Now()
		return nil
	})

	for {
		_, message, err := beacon.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket read error", "agent_id", beacon.AgentID, "error", err)
			}
			break
		}

		// Parse beacon request
		var req beaconRequest
		if err := json.Unmarshal(message, &req); err != nil {
			slog.Error("WebSocket JSON parse error", "agent_id", beacon.AgentID, "error", err)
			continue
		}

		req.UUID = beacon.AgentID

		// Process beacon
		resp := s.processBeacon(req, "")

		// Send response
		respJSON, err := json.Marshal(resp)
		if err != nil {
			slog.Error("WebSocket response marshal error", "agent_id", beacon.AgentID, "error", err)
			continue
		}

		select {
		case beacon.Send <- respJSON:
		default:
			slog.Warn("WebSocket send channel full", "agent_id", beacon.AgentID)
		}
	}
}
