package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
)

// Listener self-check heartbeat: periodically probes every enabled listener
// with a synthetic connection so a dead transport is detected within minutes
// instead of being discovered by missing beacons. Complements the QUIC/DNS
// accept-loop fixes by keeping Status honest.

type listenerHealth struct {
	ListenerID uint      `json:"listener_id"`
	Name       string    `json:"name"`
	Scheme     string    `json:"scheme"`
	Addr       string    `json:"addr"`
	LastCheck  time.Time `json:"last_check"`
	OK         bool      `json:"ok"`
	Failures   int       `json:"consecutive_failures"`
	Error      string    `json:"error,omitempty"`
	Skipped    bool      `json:"skipped"` // transports without a synthetic probe yet
}

type listenerHealthTracker struct {
	mu    sync.Mutex
	state map[uint]*listenerHealth
}

const (
	listenerHealthInterval = 5 * time.Minute
	listenerProbeTimeout   = 5 * time.Second
	listenerFailThreshold  = 2 // consecutive failures before status flips
)

var lhTracker = &listenerHealthTracker{state: map[uint]*listenerHealth{}}

func (t *listenerHealthTracker) snapshot() []listenerHealth {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]listenerHealth, 0, len(t.state))
	for _, h := range t.state {
		out = append(out, *h)
	}
	return out
}

func (s *Server) startListenerHealthLoop() {
	slog.Info("Listener health heartbeat starting", "interval", listenerHealthInterval)
	ticker := time.NewTicker(listenerHealthInterval)
	// First pass shortly after boot so status is populated fast.
	go func() {
		time.Sleep(15 * time.Second)
		s.checkAllListeners()
	}()
	for {
		select {
		case <-s.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			s.checkAllListeners()
		}
	}
}

func (s *Server) checkAllListeners() {
	var listeners []db.Listener
	if err := s.db.Where("enabled = ?", true).Find(&listeners).Error; err != nil {
		slog.Error("Listener health: query failed", "err", err)
		return
	}
	for _, ln := range listeners {
		h := s.probeListener(&ln)
		lhTracker.mu.Lock()
		prev := lhTracker.state[ln.ID]
		wasOK := prev != nil && prev.OK
		lhTracker.state[ln.ID] = h
		lhTracker.mu.Unlock()

		// Transition to failed: flip DB status + notify once.
		if !h.OK && !h.Skipped && h.Failures >= listenerFailThreshold && wasOK {
			s.db.Model(&db.Listener{}).Where("id = ?", ln.ID).Update("status", "error")
			s.DispatchNotification(&db.Notification{
				Type:     "listener_health",
				Title:    "Listener unreachable: " + ln.Name,
				Message:  fmt.Sprintf("%s listener %s:%d failed %d consecutive health checks (%s)", ln.Scheme, ln.Host, ln.Port, h.Failures, h.Error),
				Severity: "critical",
			})
			s.broadcastOperatorEvent(map[string]interface{}{
				"type": "listener_health", "listener_id": ln.ID,
				"name": ln.Name, "status": "failed", "error": h.Error,
			})
			slog.Warn("Listener health FAILED", "name", ln.Name, "addr", fmt.Sprintf("%s:%d", ln.Host, ln.Port), "err", h.Error)
		} else if h.OK && prev != nil && !wasOK && !prev.Skipped {
			// Recovery: restore honest running state.
			s.db.Model(&db.Listener{}).Where("id = ?", ln.ID).Update("status", "running")
			s.broadcastOperatorEvent(map[string]interface{}{
				"type": "listener_health", "listener_id": ln.ID,
				"name": ln.Name, "status": "recovered",
			})
			slog.Info("Listener health recovered", "name", ln.Name)
		}
	}
}

// handleListenerHealth exposes the latest self-check results.
// GET /api/listeners/health
func (s *Server) handleListenerHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "health": lhTracker.snapshot()})
}

func (s *Server) probeListener(ln *db.Listener) *listenerHealth {
	h := &listenerHealth{
		ListenerID: ln.ID,
		Name:       ln.Name,
		Scheme:     ln.Scheme,
		Addr:       net.JoinHostPort(ln.Host, strconv.Itoa(ln.Port)),
		LastCheck:  time.Now(),
	}

	switch ln.Scheme {
	case "http":
		h.OK = probeTCP(h.Addr)
		if !h.OK {
			h.Error = "tcp connect failed"
		}
	case "https", "tls":
		ok, err := probeTLS(h.Addr)
		h.OK = ok
		if !ok {
			h.Error = err.Error()
		}
	case "dns":
		ok, err := probeDNS(ln.Host, ln.Port, ln.DNSDomain)
		h.OK = ok
		if !ok {
			h.Error = err.Error()
		}
	default:
		// tcp/tcp-smb? icmp/quic/grpc/ssh/extc2 — no cheap synthetic probe yet.
		h.Skipped = true
		h.OK = true // do not count unknown transports as failures
	}
	return h
}

func probeTCP(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, listenerProbeTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func probeTLS(addr string) (bool, error) {
	conn, err := net.DialTimeout("tcp", addr, listenerProbeTimeout)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	// InsecureVerify here is intentional: we only need the handshake to
	// complete to prove a TLS listener is alive; self-signed C2 certs are
	// expected to fail chain validation.
	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, ServerName: "probe"})
	if err := tlsConn.SetDeadline(time.Now().Add(listenerProbeTimeout)); err != nil {
		return false, err
	}
	if err := tlsConn.HandshakeContext(context.Background()); err != nil {
		return false, err
	}
	_ = tlsConn.Close()
	return true, nil
}

func probeDNS(host string, port int, zone string) (bool, error) {
	if zone == "" {
		zone = "health-check.probe"
	}
	target := fmt.Sprintf("%s:%d", host, port)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("probe."+zone), dns.TypeTXT)
	client := &dns.Client{Net: "udp", Timeout: listenerProbeTimeout}
	_, _, err := client.ExchangeContext(context.Background(), m, target)
	if err != nil {
		return false, err
	}
	// The ForgeC2 handler answers every query (even unknown UUIDs) — any
	// response proves the UDP socket is being served.
	return true, nil
}
