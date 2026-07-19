package server

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── P0-3: rportfwd server-side relay (binds local port, relays via beacon) ─
func (s *Server) handleRPortFwdRelayStart(c *gin.Context) {
	// Similar to SOCKS relay: server binds port, tunnels through beacon
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	localPortStr := c.PostForm("lport")
	if localPortStr == "" {
		localPortStr = "1081"
	}
	forwardTarget := c.PostForm("target")
	if forwardTarget == "" {
		respondError(c, http.StatusBadRequest, "target host:port required")
		return
	}

	lport, err := strconv.Atoi(localPortStr)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid port")
		return
	}

	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()

	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}

	key := fmt.Sprintf("%s:%d", id, lport)
	if _, exists := s.rportfwdListeners[key]; exists {
		respondError(c, http.StatusBadRequest, "rportfwd already active for this agent:port")
		return
	}

	relay := newRPortFwdRelay(s, id, lport, forwardTarget)
	s.rportfwdListeners[key] = relay
	go relay.start()

	slog.Info("Reverse port forward relay started", "agent", id, "lport", lport, "target", forwardTarget)
	s.LogAuditRecord(c, "rportfwd_relay_start", "agent", id, fmt.Sprintf("rportfwd relay :%d -> %s via %s", lport, forwardTarget, id), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("rportfwd relay :%d -> %s via %s", lport, forwardTarget, id)})
}

func (s *Server) handleRPortFwdStatus(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()

	for _, relay := range s.rportfwdListeners {
		if relay.agentID != id {
			continue
		}
		c.JSON(http.StatusOK, gin.H{
			"active": true,
			"port":   relay.localPort,
			"target": relay.forwardTarget,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"active": false})
}

// handleRPortFwdGlobalStatus returns status of all active reverse port forwards.
func (s *Server) handleRPortFwdGlobalStatus(c *gin.Context) {
	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()

	type fwdInfo struct {
		AgentID     string `json:"agent_id"`
		LocalPort   int    `json:"local_port"`
		RemoteHost  string `json:"remote_host"`
		RemotePort  int    `json:"remote_port"`
		Protocol    string `json:"protocol"`
		Active      bool   `json:"active"`
	}
	forwards := make([]fwdInfo, 0, len(s.rportfwdListeners))
	for _, relay := range s.rportfwdListeners {
		host, portStr, _ := strings.Cut(relay.forwardTarget, ":")
		rport, _ := strconv.Atoi(portStr)
		forwards = append(forwards, fwdInfo{
			AgentID:    relay.agentID,
			LocalPort:  relay.localPort,
			RemoteHost: host,
			RemotePort: rport,
			Protocol:   "tcp",
			Active:     relay.listener != nil,
		})
	}
	if forwards == nil {
		forwards = []fwdInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"forwards": forwards})
}

func (s *Server) handleRPortFwdRelayStop(c *gin.Context) {
	id := c.Param("id")
	localPortStr := c.PostForm("lport")
	if localPortStr == "" {
		localPortStr = c.DefaultQuery("lport", "1081")
	}

	key := fmt.Sprintf("%s:%s", id, localPortStr)
	s.rportfwdMu.Lock()
	relay, exists := s.rportfwdListeners[key]
	if exists {
		delete(s.rportfwdListeners, key)
	}
	s.rportfwdMu.Unlock()

	if !exists {
		respondError(c, http.StatusNotFound, "no active rportfwd for this agent:port")
		return
	}
	relay.stop()
	slog.Info("Reverse port forward relay stopped", "agent", id, "lport", localPortStr)
	s.LogAuditRecord(c, "rportfwd_relay_stop", "agent", id, fmt.Sprintf("rportfwd relay stopped :%s", localPortStr), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "rportfwd relay stopped"})
}

// rportfwdRelay manages a local TCP listener that tunnels through a beacon channel.
type rportfwdRelay struct {
	server        *Server
	agentID       string
	localPort     int
	forwardTarget string
	listener      *rportfwdListener
	stopCh        chan struct{}
}

type rportfwdListener struct {
	ln      net.Listener
	connMap map[uint64]net.Conn
	nextID  uint64
	mu      sync.Mutex
}

func newRPortFwdRelay(s *Server, agentID string, lport int, target string) *rportfwdRelay {
	return &rportfwdRelay{
		server:        s,
		agentID:       agentID,
		localPort:     lport,
		forwardTarget: target,
		stopCh:        make(chan struct{}),
	}
}

func (r *rportfwdRelay) start() {
	addr := fmt.Sprintf("0.0.0.0:%d", r.localPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("rportfwd relay listen failed", "addr", addr, "err", err)
		return
	}
	r.listener = &rportfwdListener{
		ln:      ln,
		connMap: make(map[uint64]net.Conn),
	}
	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
		<-r.stopCh
		ln.Close()
	}()

	slog.Info("rportfwd relay listening", "addr", addr, "target", r.forwardTarget, "agent", r.agentID)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go r.handleConn(conn)
	}
}

func (r *rportfwdRelay) handleConn(operatorConn net.Conn) {
	defer operatorConn.Close()

	r.listener.mu.Lock()
	r.listener.nextID++
	connID := r.listener.nextID
	r.listener.connMap[connID] = operatorConn
	r.listener.mu.Unlock()

	defer func() {
		r.listener.mu.Lock()
		delete(r.listener.connMap, connID)
		r.listener.mu.Unlock()
	}()

	// Tell agent to connect to target
	r.server.sendRPortFwdFrame(r.agentID, connID, "rportfwd_connect", []byte(r.forwardTarget))

	// Relay operator->agent via beacon frames
	buf := make([]byte, 10240)
	for {
		n, err := operatorConn.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			r.server.sendRPortFwdFrame(r.agentID, connID, "rportfwd_data", data)
		}
		if err != nil {
			r.server.sendRPortFwdFrame(r.agentID, connID, "rportfwd_close", nil)
			return
		}
	}
}

func (r *rportfwdRelay) stop() {
	close(r.stopCh)
}

// sendRPortFwdFrame enqueues a frame for the agent to pick up on next beacon.
func (s *Server) sendRPortFwdFrame(agentID string, connID uint64, action string, data []byte) {
	s.socksEngine.enqueueFrame(agentID, socksFrame{
		ConnID: connID,
		Action: action,
		Data:   data,
	})
}

// processRPortFwdData handles rportfwd data coming FROM the agent back to the operator.
func (s *Server) processRPortFwdData(agentID string, frame socksFrame) {
	// Find the associated listener
	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()
	for key, relay := range s.rportfwdListeners {
		if strings.HasPrefix(key, agentID+":") {
	// Write data to the operator's TCP connection
			if relay.listener != nil {
				relay.listener.mu.Lock()
				conn, ok := relay.listener.connMap[frame.ConnID]
				relay.listener.mu.Unlock()
				if ok {
					conn.Write(frame.Data)
				}
			}
		}
	}
}

// cleanupStaleRPortFwd removes stale rportfwd listeners on agent disconnect
func (s *Server) cleanupStaleRPortFwd() {
	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()
	for key, relay := range s.rportfwdListeners {
		var agent db.Implant
		if err := s.db.First(&agent, "id = ?", relay.agentID).Error; err != nil {
			relay.stop()
			delete(s.rportfwdListeners, key)
			continue
		}
		if time.Since(agent.LastSeen) > s.offlineThreshold()*2 {
			relay.stop()
			delete(s.rportfwdListeners, key)
		}
	}
}

// Add rportfwdMu and rportfwdListeners to Server (handled via init in server.go)
