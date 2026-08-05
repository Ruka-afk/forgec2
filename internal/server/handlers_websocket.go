package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/server/middleware"
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
	flush      chan struct{}
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
	if existing, ok := h.beacons[agentID]; ok && existing != beacon {
		existing.closeOnce.Do(func() {
			close(existing.Send)
		})
	}
	h.beacons[agentID] = beacon
}

// Unregister removes the entry for agentID and closes its Send channel,
// but ONLY if the current entry is the same connection. This prevents one
// connection from closing another connection's channel (cross-connection DoS).
func (h *WebSocketHub) Unregister(agentID string, beacon *WebSocketBeacon) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.beacons[agentID]; ok && existing == beacon {
		beacon.closeOnce.Do(func() {
			close(beacon.Send)
		})
		delete(h.beacons, agentID)
	}
}

// Remove drops the entry for agentID without closing its Send channel.
// Used when a beacon connection migrates to its real agent UUID.
func (h *WebSocketHub) Remove(agentID string, beacon *WebSocketBeacon) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.beacons[agentID]; ok && existing == beacon {
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
		func() {
			defer func() { recover() }()
			select {
			case beacon.Send <- data:
			default:
			}
		}()
	}
}

// handleWebSocketBeacon handles WebSocket beacon connections.
// v2: frame-level envelopes are authenticated by decodeBeaconEnvelope, so the
// upgrade itself requires no separate key check.
func (s *Server) handleWebSocketBeacon(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	// Allow agent to pass its persistent UUID via query parameter
	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = uuid.New().String()
	} else if !isValidAgentID(agentID) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	slog.Info("WebSocket beacon connected", "agent_id", agentID, "remote_addr", conn.RemoteAddr())

	beacon := &WebSocketBeacon{
		Conn:       conn,
		AgentID:    agentID,
		LastSeen:   time.Now(),
		Send:       make(chan []byte, 256),
		BatchQueue: make([][]byte, 0, 16),
		flush:      make(chan struct{}, 1),
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
	s.wg.Add(2)
	go s.wsWritePump(beacon)
	go s.wsReadPump(beacon)
}

func (s *Server) wsWritePump(beacon *WebSocketBeacon) {
	defer s.wg.Done()
	defer func() {
		beacon.BatchMutex.Lock()
		if beacon.BatchTimer != nil {
			beacon.BatchTimer.Stop()
		}
		beacon.BatchMutex.Unlock()
		beacon.Conn.Close()
		s.wsHub.Unregister(beacon.AgentID, beacon)
	}()

	ticker := time.NewTicker(BeaconPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			beacon.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case message, ok := <-beacon.Send:
			if !ok {
				beacon.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			s.enqueueBatchMessage(beacon, message)

		case <-beacon.flush:
			s.flushBatchQueue(beacon)

		case <-ticker.C:
			beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
			if err := beacon.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("WebSocket ping error", "agent_id", beacon.AgentID, "error", err)
				return
			}
		}
	}
}

// signalBatchFlush schedules a flush on the write pump goroutine (non-blocking).
func (s *Server) signalBatchFlush(beacon *WebSocketBeacon) {
	select {
	case beacon.flush <- struct{}{}:
	default:
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
			s.signalBatchFlush(beacon)
		})
		beacon.BatchMutex.Unlock()
	} else if currentLen >= BatchFlushThreshold {
		beacon.BatchMutex.Lock()
		if beacon.BatchTimer != nil {
			beacon.BatchTimer.Stop()
		}
		beacon.BatchMutex.Unlock()
		s.signalBatchFlush(beacon)
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
			slog.Warn("WebSocket gzip compress failed, sending uncompressed", "agent_id", beacon.AgentID, "err", err)
		} else {
			s.sendCompressedMessage(beacon, data)
			return
		}
	}
	s.sendTextMessage(beacon, data)
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
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered from wsReadPump panic", "agent_id", beacon.AgentID, "err", r, "stack", string(debug.Stack()))
		}
	}()
	defer func() {
		s.wsHub.Unregister(beacon.AgentID, beacon)
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

		env, req, kind := s.decodeBeaconEnvelope(message)
		if kind == frameRejected {
			slog.Warn("WebSocket beacon envelope rejected", "agent_id", beacon.AgentID)
			continue
		}

		// Re-register under the agent's real UUID if it differs from connection UUID
		if req.UUID != beacon.AgentID && isValidAgentID(req.UUID) {
			s.wsHub.Remove(beacon.AgentID, beacon)
			beacon.AgentID = req.UUID
			s.wsHub.Register(beacon.AgentID, beacon)
		}

		var respJSON []byte
		if kind == frameEncrypted {
			// Process beacon (auth frames are handled without the pipeline:
			// the agent discards the auth response and re-beacons encrypted,
			// so claiming/fetching tasks here would lose them).
			resp := s.processBeacon(req, "")
			if s.sessionManager.NeedsRekey(req.UUID, BeaconSessionRekeyMessages) {
				resp.Rekey = true
			}
			var ok bool
			respJSON, ok = s.buildBeaconResponse(req.UUID, env.Seq, resp)
			if !ok {
				slog.Error("WebSocket response build error", "agent_id", beacon.AgentID)
				continue
			}
		} else {
			var ok bool
			respJSON, ok = s.processAuthFrame(env, kind)
			if !ok {
				slog.Warn("WebSocket auth frame rejected", "agent_id", beacon.AgentID)
				continue
			}
		}

		func() {
			defer func() { recover() }()
			select {
			case beacon.Send <- respJSON:
			default:
				slog.Warn("WebSocket send channel full", "agent_id", beacon.AgentID)
			}
		}()
	}
}

// WSOperatorSession tracks an operator's WebSocket session.
type WSOperatorSession struct {
	Conn      *websocket.Conn
	UserID    uint
	Username  string
	Page      string
	AgentView string
	LastSeen  time.Time
}

// operatorSessions tracks connected operator WebSocket sessions.
type operatorSessionTracker struct {
	mu       sync.Mutex
	sessions map[uint]*WSOperatorSession
}

func (t *operatorSessionTracker) add(s *WSOperatorSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if existing, ok := t.sessions[s.UserID]; ok && existing != s {
		existing.Conn.Close()
	}
	t.sessions[s.UserID] = s
}

func (t *operatorSessionTracker) remove(userID uint, s *WSOperatorSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cur, ok := t.sessions[userID]; ok && cur == s {
		delete(t.sessions, userID)
	}
}

// BroadcastToOperators sends a message to all connected operator sessions.
// Takes a snapshot of sessions under the lock, then writes without holding it.
func (t *operatorSessionTracker) BroadcastToOperators(msg []byte) {
	t.mu.Lock()
	sessions := make([]*WSOperatorSession, 0, len(t.sessions))
	for _, session := range t.sessions {
		sessions = append(sessions, session)
	}
	t.mu.Unlock()

	for _, session := range sessions {
		session.Conn.SetWriteDeadline(time.Now().Add(OperatorWriteDeadline))
		if err := session.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Warn("Failed to send to operator, dropping connection", "user", session.Username, "error", err)
			session.Conn.Close()
			t.remove(session.UserID, session)
		}
	}
}

// handleOperatorWS handles operator WebSocket connections for real-time updates.
// Requires a valid forgec2_session cookie (unlike agent beacons which use the
// transport envelope for auth).
func (s *Server) handleOperatorWS(c *gin.Context) {
	tokenStr, err := c.Cookie("forgec2_session")
	if err != nil || tokenStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no session token"})
		return
	}
	claims, err := middleware.ParseToken(tokenStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
		return
	}
	if s.isSessionRevoked(tokenStr) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "session_revoked"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Operator WS upgrade failed", "error", err)
		return
	}

	session := &WSOperatorSession{
		Conn:     conn,
		UserID:   claims.UserID,
		Username: claims.Username,
		LastSeen: time.Now(),
	}

	s.wsHubOnce.Do(func() {
		if s.wsHub == nil {
			s.wsHub = NewWebSocketHub()
		}
	})
	s.operatorSessions.add(session)

	defer func() {
		s.operatorSessions.remove(session.UserID, session)
		conn.Close()
	}()

	readDeadline := 90 * time.Second
	conn.SetReadLimit(WSMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(readDeadline))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "page_update":
			if page, ok := msg["page"].(string); ok {
				session.Page = page
			}
		case "agent_view":
			if agentID, ok := msg["agent_id"].(string); ok {
				session.AgentView = agentID
			}
		case "ping":
			conn.WriteJSON(map[string]string{"type": "pong"})
		}
	}
}
