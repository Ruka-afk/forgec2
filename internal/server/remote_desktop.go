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

// RDSession represents a remote desktop session for an agent.
type RDSession struct {
	AgentID   string
	Operators map[string]*websocket.Conn
	// writeMu serializes writes per operator connection. gorilla/websocket
	// allows only one concurrent writer per connection, and frames can be
	// produced concurrently (agent screen_frame POST + operator session).
	writeMu map[string]*sync.Mutex
	mu      sync.RWMutex
}

// RDHub manages all remote desktop sessions.
type RDHub struct {
	sessions map[string]*RDSession
	mu       sync.RWMutex
}

var rdHub = &RDHub{
	sessions: make(map[string]*RDSession),
}

// Join adds an operator to a remote desktop session.
func (h *RDHub) Join(agentID, operatorID string, conn *websocket.Conn) {
	h.mu.Lock()
	session, exists := h.sessions[agentID]
	if !exists {
		session = &RDSession{
			AgentID:   agentID,
			Operators: make(map[string]*websocket.Conn),
			writeMu:   make(map[string]*sync.Mutex),
		}
		h.sessions[agentID] = session
	}
	h.mu.Unlock()

	session.mu.Lock()
	session.Operators[operatorID] = conn
	session.writeMu[operatorID] = &sync.Mutex{}
	session.mu.Unlock()

	slog.Info("Operator joined remote desktop", "agent_id", agentID, "user", operatorID)
}

// Leave removes an operator from a remote desktop session.
func (h *RDHub) Leave(agentID, operatorID string) {
	h.mu.RLock()
	session, exists := h.sessions[agentID]
	h.mu.RUnlock()
	if !exists {
		return
	}

	session.mu.Lock()
	delete(session.Operators, operatorID)
	delete(session.writeMu, operatorID)
	empty := len(session.Operators) == 0
	session.mu.Unlock()

	if empty {
		h.mu.Lock()
		delete(h.sessions, agentID)
		h.mu.Unlock()
	}
}

// BroadcastFrame sends a screen frame to all operators in a session.
func (h *RDHub) BroadcastFrame(agentID string, frameData []byte) {
	h.mu.RLock()
	session, exists := h.sessions[agentID]
	h.mu.RUnlock()
	if !exists {
		return
	}

	session.mu.RLock()
	operators := make(map[string]*websocket.Conn, len(session.Operators))
	locks := make(map[string]*sync.Mutex, len(session.writeMu))
	for id, conn := range session.Operators {
		operators[id] = conn
	}
	for id, mu := range session.writeMu {
		locks[id] = mu
	}
	session.mu.RUnlock()

	for operatorID, conn := range operators {
		mu := locks[operatorID]
		if mu == nil {
			// Operator left concurrently; skip.
			continue
		}
		conn.SetWriteDeadline(time.Now().Add(RemoteDesktopWriteDeadline))
		mu.Lock()
		err := conn.WriteMessage(websocket.BinaryMessage, frameData)
		mu.Unlock()
		if err != nil {
			slog.Debug("Failed to send frame", "user", operatorID, "error", err)
			h.Leave(agentID, operatorID)
		}
	}
}

// SendInput sends an input event to the agent via the beacon WebSocket.
func (h *RDHub) SendInput(s *Server, agentID string, inputType string, data map[string]interface{}) {
	if beacon := s.wsHub.Get(agentID); beacon != nil {
		payload := map[string]interface{}{
			"type":       "remote_input",
			"input_type": inputType,
			"data":       data,
		}
		if payloadJSON, err := json.Marshal(payload); err == nil {
			func() {
				defer func() { recover() }()
				select {
				case beacon.Send <- payloadJSON:
				default:
				}
			}()
		}
	}
}

// handleRDWebSocket handles operator WebSocket connections for remote desktop.
func (s *Server) handleRDWebSocket(c *gin.Context) {
	// Remote desktop streams screen content and relays operator input to agents,
	// so viewer-role users must not access it.
	if !s.requireOperator(c) {
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("RD WebSocket upgrade failed", "error", err)
		return
	}

	agentID := c.Query("agent_id")
	operatorID := c.Query("user_id")
	if agentID == "" || operatorID == "" {
		conn.Close()
		return
	}

	rdHub.Join(agentID, operatorID, conn)
	defer rdHub.Leave(agentID, operatorID)

	conn.SetReadLimit(WSMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(RemoteDesktopReadDeadline))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(RemoteDesktopReadDeadline))
		return nil
	})

	// Listen for input events from operator
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var inputEvent struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(message, &inputEvent); err != nil {
			continue
		}

		switch inputEvent.Type {
		case "mouse_move", "mouse_click", "mouse_wheel", "key_down", "key_up":
			rdHub.SendInput(s, agentID, inputEvent.Type, inputEvent.Data)
		}
	}
}

// handleRDAPIGetFrame returns the latest screen frame for an agent (HTTP fallback).
func (s *Server) handleRDAPIGetFrame(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	agentID := c.Param("id")

	// Get frame from the frame buffer
	frame := getFrameBuffer(agentID)
	if frame == nil {
		respondError(c, http.StatusNotFound, "no frame available")
		return
	}

	c.Data(http.StatusOK, "image/png", frame)
}

// frameBuffers stores the latest screen frame for each agent.
var (
	frameBuffers  = make(map[string][]byte)
	frameBufferMu sync.RWMutex
)

// getFrameBuffer returns the latest frame for an agent.
func getFrameBuffer(agentID string) []byte {
	frameBufferMu.RLock()
	defer frameBufferMu.RUnlock()
	return frameBuffers[agentID]
}

// handleRDAPIScreenshot triggers a screenshot request for an agent.
func (s *Server) handleRDAPIScreenshot(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "screenshot", "screenshot", "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create screenshot task")
		return
	}

	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": "screenshot requested"})
}
