package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
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
	sort.Slice(out, func(i, j int) bool {
		return out[i].ListenerID < out[j].ListenerID
	})
	return out
}

// observe records a probe and carries forward the consecutive failure count.
// A successful or unsupported probe starts a fresh sequence.
func (t *listenerHealthTracker) observe(h *listenerHealth) {
	t.mu.Lock()
	defer t.mu.Unlock()

	previous := t.state[h.ListenerID]
	if h.OK || h.Skipped {
		h.Failures = 0
	} else {
		h.Failures = 1
		if previous != nil && !previous.OK && !previous.Skipped {
			h.Failures = previous.Failures + 1
		}
	}
	t.state[h.ListenerID] = h
}

// prune removes disabled or deleted listeners from the API snapshot.
func (t *listenerHealthTracker) prune(active map[uint]struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.state {
		if _, ok := active[id]; !ok {
			delete(t.state, id)
		}
	}
}

func (s *Server) startListenerHealthLoop() {
	slog.Info("Listener health heartbeat starting", "interval", listenerHealthInterval)
	ticker := time.NewTicker(listenerHealthInterval)
	initial := time.NewTimer(15 * time.Second)
	defer ticker.Stop()
	defer initial.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-initial.C:
			// First pass shortly after boot so status is populated fast.
			s.checkAllListeners()
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
	active := make(map[uint]struct{}, len(listeners))
	for i := range listeners {
		active[listeners[i].ID] = struct{}{}
	}
	lhTracker.prune(active)

	ctx := s.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	for _, ln := range listeners {
		if err := ctx.Err(); err != nil {
			return
		}
		h := s.probeListener(ctx, &ln)
		lhTracker.observe(h)

		// Use a conditional update as the transition gate. It both retries a
		// failed persistence on the next pass and prevents duplicate alerts if
		// two health sweeps overlap.
		if !h.OK && !h.Skipped && h.Failures >= listenerFailThreshold {
			res := s.db.Model(&db.Listener{}).
				Where("id = ? AND status <> ?", ln.ID, "error").
				Update("status", "error")
			if res.Error != nil {
				slog.Error("Listener health: failed to persist failure", "listener_id", ln.ID, "err", res.Error)
				continue
			}
			if res.RowsAffected == 0 {
				continue
			}
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
		} else if h.OK && !h.Skipped {
			// Recovery: restore honest running state.
			res := s.db.Model(&db.Listener{}).
				Where("id = ? AND status = ?", ln.ID, "error").
				Update("status", "running")
			if res.Error != nil {
				slog.Error("Listener health: failed to persist recovery", "listener_id", ln.ID, "err", res.Error)
				continue
			}
			if res.RowsAffected == 0 {
				continue
			}
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

func (s *Server) probeListener(ctx context.Context, ln *db.Listener) *listenerHealth {
	h := &listenerHealth{
		ListenerID: ln.ID,
		Name:       ln.Name,
		Scheme:     ln.Scheme,
		Addr:       net.JoinHostPort(ln.Host, strconv.Itoa(ln.Port)),
		LastCheck:  time.Now(),
	}

	switch ln.Scheme {
	case "http":
		h.OK = probeTCP(ctx, h.Addr)
		if !h.OK {
			h.Error = "tcp connect failed"
		}
	case "https", "tls":
		ok, err := probeTLS(ctx, h.Addr)
		h.OK = ok
		if !ok {
			h.Error = err.Error()
		}
	case "dns":
		ok, err := probeDNS(ctx, ln.Host, ln.Port, ln.DNSDomain)
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

func probeTCP(ctx context.Context, addr string) bool {
	dialer := net.Dialer{Timeout: listenerProbeTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func probeTLS(ctx context.Context, addr string) (bool, error) {
	dialer := net.Dialer{Timeout: listenerProbeTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
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
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return false, err
	}
	_ = tlsConn.Close()
	return true, nil
}

func probeDNS(ctx context.Context, host string, port int, zone string) (bool, error) {
	if zone == "" {
		zone = "health-check.probe"
	}
	target := fmt.Sprintf("%s:%d", host, port)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("probe."+zone), dns.TypeTXT)
	client := &dns.Client{Net: "udp", Timeout: listenerProbeTimeout}
	_, _, err := client.ExchangeContext(ctx, m, target)
	if err != nil {
		return false, err
	}
	// The ForgeC2 handler answers every query (even unknown UUIDs) — any
	// response proves the UDP socket is being served.
	return true, nil
}
