package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// WebSocketListener handles WebSocket-based C2 transport for agents.
// This provides an alternative to HTTP beacons for stealthy communication.
type WebSocketListener struct {
	server   *Server
	upgrader websocket.Upgrader
	mu       sync.Mutex
	sessions map[string]*WSOperatorSession
}

// WSOperatorSession tracks an operator's WebSocket session.
type WSOperatorSession struct {
	Conn      *websocket.Conn
	UserID    string
	Page      string
	AgentView string
	LastSeen  time.Time
}

var wsListener *WebSocketListener

// Close closes all active operator sessions.
func (wl *WebSocketListener) Close() {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	for _, session := range wl.sessions {
		session.Conn.Close()
	}
	wl.sessions = make(map[string]*WSOperatorSession)
}

// InitWebSocketListener initializes the WebSocket listener.
func InitWebSocketListener(s *Server) *WebSocketListener {
	if wsListener != nil {
		wsListener.Close()
	}
	wsListener = &WebSocketListener{
		server: s,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  WSReadBufSize,
			WriteBufferSize: WSWriteBufSize,
			CheckOrigin: func(r *http.Request) bool {
				return allowedOrigin(s.cfg, r)
			},
		},
		sessions: make(map[string]*WSOperatorSession),
	}
	return wsListener
}

// handleAgentWS handles agent WebSocket connections for C2 transport.
func (wl *WebSocketListener) handleAgentWS(c *gin.Context) {
	if !wl.server.checkBeaconKey(c) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	conn, err := wl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Agent WS upgrade failed", "error", err)
		return
	}

	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = generateAgentID()
	} else if !isValidAgentID(agentID) {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	conn.SetReadLimit(4096)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	wl.server.wg.Add(1)
	go func() {
		defer wl.server.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-wl.server.ctx.Done():
				return
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	slog.Info("Agent WS connected", "agent_id", agentID)

	// Register with the beacon hub
	beacon := &WebSocketBeacon{
		Conn:       conn,
		AgentID:    agentID,
		LastSeen:   time.Now(),
		Send:       make(chan []byte, 256),
		BatchQueue: make([][]byte, 0, 16),
		flush:      make(chan struct{}, 1),
		compress:   true,
	}

	wl.server.wsHubOnce.Do(func() {
		if wl.server.wsHub == nil {
			wl.server.wsHub = NewWebSocketHub()
		}
	})
	wl.server.wsHub.Register(agentID, beacon)

	// Start pumps with proper WaitGroup tracking
	wl.server.wg.Add(2)
	go func() {
		defer wl.server.wg.Done()
		wl.server.wsWritePump(beacon)
	}()
	go func() {
		defer wl.server.wg.Done()
		wl.agentReadPump(beacon)
	}()
}

// agentReadPump reads messages from an agent WebSocket connection.
func (wl *WebSocketListener) agentReadPump(beacon *WebSocketBeacon) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Agent WS read pump panicked", "agent_id", beacon.AgentID, "recover", r)
		}
		wl.server.wsHub.Unregister(beacon.AgentID, beacon)
		beacon.Conn.Close()
	}()

	beacon.Conn.SetReadLimit(WSMaxMessageSize)
	beacon.Conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
	beacon.Conn.SetPongHandler(func(string) error {
		beacon.Conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
		beacon.LastSeen = time.Now()
		return nil
	})

	for {
		_, message, err := beacon.Conn.ReadMessage()
		if err != nil {
			break
		}

		// Process beacon request via the shared transport envelope so WS enforces
		// the same auth (beacon_key, ECDH, forceECDH) as HTTP and TCP.
		envelope, req, useECDH, ok := wl.server.decodeBeaconEnvelope(message, "")
		if !ok {
			slog.Warn("Agent WS envelope rejected", "agent_id", beacon.AgentID)
			continue
		}

		// Re-register under the agent's real UUID if it differs from connection UUID
		if req.UUID != beacon.AgentID && isValidAgentID(req.UUID) {
			wl.server.wsHub.Remove(beacon.AgentID, beacon)
			beacon.AgentID = req.UUID
			wl.server.wsHub.Register(beacon.AgentID, beacon)
		}

		resp := wl.server.processBeacon(req, "")

		respJSON, ok := wl.server.buildBeaconResponse(req, resp, useECDH, envelope.ECDHPub != "" && !useECDH)
		if !ok {
			slog.Error("Agent WS response build error", "agent_id", beacon.AgentID)
			continue
		}

		func() {
			defer func() { recover() }()
			select {
			case beacon.Send <- respJSON:
			default:
				slog.Warn("Agent WS send channel full", "agent_id", beacon.AgentID)
			}
		}()
	}
}

// handleOperatorWS handles operator WebSocket connections for real-time updates.
func (wl *WebSocketListener) handleOperatorWS(c *gin.Context) {
	conn, err := wl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Operator WS upgrade failed", "error", err)
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	session := &WSOperatorSession{
		Conn:     conn,
		UserID:   userID,
		LastSeen: time.Now(),
	}

	wl.mu.Lock()
	wl.sessions[userID] = session
	wl.mu.Unlock()

	defer func() {
		wl.mu.Lock()
		delete(wl.sessions, userID)
		wl.mu.Unlock()
		conn.Close()
	}()

	readDeadline := 90 * time.Second
	conn.SetReadLimit(WSMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(readDeadline))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	// Read loop
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

// BroadcastToOperators sends a message to all connected operator sessions.
// Takes a snapshot of sessions under the lock, then writes without holding it.
func (wl *WebSocketListener) BroadcastToOperators(msg []byte) {
	wl.mu.Lock()
	sessions := make([]*WSOperatorSession, 0, len(wl.sessions))
	for _, session := range wl.sessions {
		sessions = append(sessions, session)
	}
	wl.mu.Unlock()

	for _, session := range sessions {
		session.Conn.SetWriteDeadline(time.Now().Add(OperatorWriteDeadline))
		if err := session.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Warn("Failed to send to operator, dropping connection", "user", session.UserID, "error", err)
			session.Conn.Close()
			wl.mu.Lock()
			delete(wl.sessions, session.UserID)
			wl.mu.Unlock()
		}
	}
}

// generateAgentID generates a unique agent ID using UUID.
func generateAgentID() string {
	return "ws_" + uuid.New().String()
}

// RegisterWSRoutes registers WebSocket transport routes.
func (s *Server) RegisterWSRoutes(r *gin.RouterGroup) {
	listener := InitWebSocketListener(s)
	r.GET("/ws/agent", listener.handleAgentWS)
	r.GET("/ws/operator", listener.handleOperatorWS)
}
