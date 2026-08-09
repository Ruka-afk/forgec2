package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// H2CBeaconListener serves the beacon endpoint over cleartext HTTP/2
// (h2c). The agent dials plain TCP and speaks HTTP/2 by prior knowledge, so
// the listener enables Go's built-in unencrypted HTTP/2 support and reuses
// the main router: beacon envelopes hit the exact same handler/rate-limit
// path as the primary HTTP listener.
type H2CBeaconListener struct {
	mu      sync.Mutex
	addr    string
	handler http.Handler
	srv     *http.Server
	ln      net.Listener
	running bool
}

// NewH2CBeaconListener creates an h2c listener that delegates requests to
// handler (typically the server router).
func NewH2CBeaconListener(addr string, handler http.Handler) *H2CBeaconListener {
	return &H2CBeaconListener{addr: addr, handler: handler}
}

// Start binds the TCP socket and serves HTTP/1.1 + unencrypted HTTP/2.
func (l *H2CBeaconListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return nil
	}
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              l.addr,
		Handler:           l.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       HTTPReadTimeout,
		WriteTimeout:      HTTPWriteTimeout,
		IdleTimeout:       HTTPIdleTimeout,
	}
	srv.Protocols = new(http.Protocols)
	srv.Protocols.SetHTTP1(true)
	srv.Protocols.SetUnencryptedHTTP2(true)

	l.ln = ln
	l.srv = srv
	l.running = true

	slog.Info("H2C beacon listener starting", "addr", ln.Addr().String())
	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("H2C listener error", "addr", l.addr, "err", serveErr)
		}
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
	}()
	return nil
}

// Addr returns the actual bound address (useful when listening on port 0).
func (l *H2CBeaconListener) Addr() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ln == nil {
		return ""
	}
	return l.ln.Addr().String()
}

// Stop gracefully shuts the HTTP server down.
func (l *H2CBeaconListener) Stop() error {
	l.mu.Lock()
	srv := l.srv
	l.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := srv.Shutdown(ctx)
	l.mu.Lock()
	l.srv = nil
	l.running = false
	l.mu.Unlock()
	return err
}

// Close implements io.Closer for use with the extraListeners map.
func (l *H2CBeaconListener) Close() error {
	return l.Stop()
}

// IsRunning reports whether the listener is serving connections.
func (l *H2CBeaconListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

// startExtraH2CListener starts an h2c beacon listener from a UI-created DB
// record. The key format is "h2c://host:port".
func (s *Server) startExtraH2CListener(key string) error {
	addr := key[len("h2c://"):]
	hl := NewH2CBeaconListener(addr, s.router)
	if err := hl.Start(); err != nil {
		return err
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = hl
	s.extraListenersMu.Unlock()
	slog.Info("Extra H2C listener started", "addr", addr, "key", key)
	return nil
}
