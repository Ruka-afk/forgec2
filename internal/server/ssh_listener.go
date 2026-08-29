package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// sshMaxEnvelopeBytes caps the beacon envelope accepted over one SSH session.
// Aligned with the 16 MiB frame cap used by the TCP/SMB transports so an
// anonymous client cannot drive disproportionate memory growth per session.
const sshMaxEnvelopeBytes = 16 * 1024 * 1024

// sshMaxConcurrentSessions bounds simultaneously open SSH sessions. Each
// session can buffer up to sshMaxEnvelopeBytes while reading, so the cap also
// bounds worst-case heap use from unauthenticated connections.
const sshMaxConcurrentSessions = 32

// sshBanner mimics a stock OpenSSH banner so the listener is not trivially
// fingerprinted as a bespoke C2 transport.
const sshBanner = "SSH-2.0-OpenSSH_9.6p1 Debian-4"

// SSHListenerConfig carries the runtime parameters for a beacon SSH listener.
type SSHListenerConfig struct {
	// Addr is the TCP listen address, e.g. ":2222" or "127.0.0.1:22".
	Addr string
	// User optionally pins the accepted username. Empty accepts any user.
	User string
	// Password optionally requires this password. Empty accepts any password
	// or public key (key-only mode). Matches server.ssh_password semantics.
	Password string
	// KeyAuth permits public key authentication. It is always permitted in
	// key-only mode (empty Password).
	KeyAuth bool
	// HostKey is the server host key signer used for the SSH handshake.
	HostKey ssh.Signer
}

// SSHBeaconListener serves beacon envelopes over the SSH transport. The agent
// opens a session, runs `/bin/sh -c 'cat'`, writes the protocol-v2 envelope to
// stdin and reads the response from stdout, so this listener only needs to
// accept a session, read stdin to EOF, and echo the handler result as stdout.
type SSHBeaconListener struct {
	mu      sync.Mutex
	cfg     SSHListenerConfig
	ln      net.Listener
	running bool
	handler func(string, []byte) []byte
}

// NewSSHBeaconListener creates an SSH beacon listener bound to cfg.Addr.
func NewSSHBeaconListener(cfg SSHListenerConfig) *SSHBeaconListener {
	return &SSHBeaconListener{cfg: cfg}
}

// SetHandler sets the beacon processing callback.
func (l *SSHBeaconListener) SetHandler(fn func(string, []byte) []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handler = fn
}

// Start binds the TCP socket and begins accepting SSH connections.
func (l *SSHBeaconListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return nil
	}
	if l.cfg.HostKey == nil {
		return fmt.Errorf("ssh listener: host key not configured")
	}
	ln, err := net.Listen("tcp", l.cfg.Addr)
	if err != nil {
		return fmt.Errorf("ssh listener bind %s: %w", l.cfg.Addr, err)
	}
	l.ln = ln
	l.running = true

	if l.cfg.Password == "" {
		// Documented lab semantics, but too dangerous to stay silent: with no
		// password pinned, ANY username/password/key combination authenticates.
		slog.Error("SSH beacon listener has NO password configured: authentication is open to anyone. Set server.ssh_password for anything beyond a lab.",
			"addr", l.cfg.Addr)
	}

	slog.Info("SSH beacon listener starting", "addr", ln.Addr().String(), "user", l.cfg.User, "key_auth", l.cfg.KeyAuth)
	go l.serve(ln)
	return nil
}

// Addr returns the actual bound address (useful when listening on port 0).
func (l *SSHBeaconListener) Addr() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.ln == nil {
		return ""
	}
	return l.ln.Addr().String()
}

// Stop closes the listening socket. In-flight sessions are left to drain.
func (l *SSHBeaconListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running || l.ln == nil {
		return nil
	}
	l.running = false
	err := l.ln.Close()
	l.ln = nil
	return err
}

// Close implements io.Closer for use with the extraListeners map.
func (l *SSHBeaconListener) Close() error {
	return l.Stop()
}

// IsRunning reports whether the listener is accepting connections.
func (l *SSHBeaconListener) IsRunning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *SSHBeaconListener) serve(ln net.Listener) {
	// Bounded session semaphore: each in-flight session may buffer up to
	// sshMaxEnvelopeBytes, so unauthenticated connection floods must not
	// translate into unbounded heap growth.
	sessions := make(chan struct{}, sshMaxConcurrentSessions)
	consecutiveErrors := 0
	for {
		conn, err := ln.Accept()
		if err != nil {
			l.mu.Lock()
			running := l.running
			l.mu.Unlock()
			if !running {
				return
			}
			// Back off transient accept failures (EMFILE etc.) instead of
			// hot-spinning at full CPU while the process is resource-starved.
			consecutiveErrors++
			time.Sleep(time.Duration(consecutiveErrors) * 50 * time.Millisecond)
			if consecutiveErrors > 20 {
				slog.Error("SSH listener accept failing repeatedly, stopping listener", "addr", l.cfg.Addr)
				l.mu.Lock()
				l.running = false
				l.mu.Unlock()
				ln.Close()
				return
			}
			slog.Error("SSH listener accept error", "addr", l.cfg.Addr, "err", err)
			continue
		}
		consecutiveErrors = 0

		select {
		case sessions <- struct{}{}:
		default:
			// At capacity: reject immediately instead of queueing memory-hungry
			// handshakes.
			slog.Warn("SSH listener at session cap, rejecting connection", "addr", conn.RemoteAddr())
			conn.Close()
			continue
		}
		go func() {
			defer func() { <-sessions }()
			defer func() {
				if r := recover(); r != nil {
					// Handshake/channel code parses hostile peer bytes in a
					// bare goroutine: one panic must not kill the server.
					slog.Error("Panic in SSH connection handler", "remote", conn.RemoteAddr(), "recover", r)
				}
			}()
			l.handleConnection(conn)
		}()
	}
}

// handleConnection performs the SSH handshake and dispatches sessions.
func (l *SSHBeaconListener) handleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	serverCfg, err := buildSSHServerConfig(l.cfg.User, l.cfg.Password, l.cfg.KeyAuth, l.cfg.HostKey)
	if err != nil {
		slog.Error("SSH server config failed", "remote", conn.RemoteAddr(), "err", err)
		return
	}
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, serverCfg)
	if err != nil {
		slog.Debug("SSH handshake failed", "remote", conn.RemoteAddr(), "err", err)
		return
	}
	defer sshConn.Close()
	conn.SetDeadline(time.Time{})

	// Idle-session bound (P2): after handshake all deadlines were cleared and
	// nothing else limits how long an authenticated client can park its
	// semaphore slot doing nothing. With open auth, 32 idle TCP connections
	// could permanently disable this transport. Re-arm an absolute idle
	// deadline; the beacon protocol's request/response cycle refreshes it via
	// the reads below.
	idle := 5 * time.Minute
	conn.SetDeadline(time.Now().Add(idle))
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		// Any channel activity counts as progress; extend the window.
		conn.SetDeadline(time.Now().Add(idle))
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "only session channels are supported")
			continue
		}
		ch, requests, err := newCh.Accept()
		if err != nil {
			continue
		}
		go l.handleSession(sshConn.User(), ch, requests)
	}
}

// handleSession services a single SSH session: it waits for the exec request,
// reads the beacon envelope from stdin to EOF, hands it to the beacon handler
// and writes the response to stdout before closing the channel.
func (l *SSHBeaconListener) handleSession(user string, ch ssh.Channel, requests <-chan *ssh.Request) {
	defer ch.Close()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("SSH session panic", "user", user, "err", r, "stack", string(debug.Stack()))
		}
	}()

	for req := range requests {
		switch req.Type {
		case "exec":
			var payload struct{ Command string }
			ssh.Unmarshal(req.Payload, &payload)
			req.Reply(true, nil)

			// Bounded read: the idle deadline set in handleConnection covers
			// this too, but keep the session loop honest about activity.
			body, err := io.ReadAll(io.LimitReader(ch, sshMaxEnvelopeBytes))
			if err == nil && len(body) > 0 {
				l.dispatch(user, body, ch)
			} else if err != nil {
				slog.Debug("SSH session read failed", "user", user, "err", err)
			}
			ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
			ch.CloseWrite()
			return
		case "shell", "pty-req", "window-change":
			// The beacon transport never needs a pty or an interactive shell.
			req.Reply(false, nil)
		case "env":
			req.Reply(true, nil)
		default:
			req.Reply(false, nil)
		}
	}
}

func (l *SSHBeaconListener) dispatch(user string, body []byte, ch ssh.Channel) {
	l.mu.Lock()
	handler := l.handler
	l.mu.Unlock()
	if handler == nil {
		slog.Error("SSH beacon handler not set", "user", user)
		return
	}
	agentID := envelopeAgentID(body)
	resp := handler(agentID, body)
	if len(resp) == 0 {
		slog.Debug("SSH beacon returned no response", "agent", agentID, "user", user)
		return
	}
	if _, err := ch.Write(resp); err != nil {
		slog.Debug("SSH session write failed", "agent", agentID, "err", err)
	}
}

// envelopeAgentID extracts the beacon UUID from the protocol-v2 envelope JSON.
// Returns "" when the envelope does not carry one (the handler then falls back
// to the per-request UUID field).
func envelopeAgentID(body []byte) string {
	var env struct {
		UUID string `json:"uuid"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.UUID == "" {
		return ""
	}
	return env.UUID
}

// buildSSHServerConfig constructs the server auth config. Auth semantics mirror
// server.ssh_* config: an empty password means "accept any password or key",
// and key auth is additionally gated on SSHKeyAuth when a password is set.
func buildSSHServerConfig(user, password string, keyAuth bool, hostKey ssh.Signer) (*ssh.ServerConfig, error) {
	cfg := &ssh.ServerConfig{
		ServerVersion: sshBanner,
		MaxAuthTries:  6,
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if user != "" && c.User() != user {
				return nil, fmt.Errorf("ssh: unknown user %q", c.User())
			}
			if password != "" {
				want := []byte(password)
				if subtle.ConstantTimeCompare(pass, want) != 1 {
					return nil, fmt.Errorf("ssh: password auth failed")
				}
			}
			return nil, nil
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if user != "" && c.User() != user {
				return nil, fmt.Errorf("ssh: unknown user %q", c.User())
			}
			if password != "" && !keyAuth {
				return nil, fmt.Errorf("ssh: public key auth not permitted")
			}
			return nil, nil
		},
		KeyboardInteractiveCallback: func(c ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			if user != "" && c.User() != user {
				return nil, fmt.Errorf("ssh: unknown user %q", c.User())
			}
			if password != "" {
				return nil, fmt.Errorf("ssh: keyboard-interactive auth not permitted")
			}
			// The agent's no-credentials fallback answers empty questions.
			if _, err := challenge("", "", []string{""}, []bool{false}); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}
	cfg.AddHostKey(hostKey)
	return cfg, nil
}

// loadOrGenerateSSHHostKey loads the ed25519 host key at path, generating and
// persisting a fresh one (mode 0600) when the file is missing or unparseable.
// An empty path returns a fresh in-memory key so listeners remain usable even
// without a data directory.
func loadOrGenerateSSHHostKey(path string) (ssh.Signer, error) {
	if path != "" {
		if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
			if signer, err := ssh.ParsePrivateKey(raw); err == nil {
				return signer, nil
			}
			slog.Warn("SSH host key unparseable, regenerating", "path", path)
		}
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating SSH host key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, fmt.Errorf("wrapping SSH host key: %w", err)
	}

	if path != "" {
		if blk, err := ssh.MarshalPrivateKey(priv, ""); err == nil {
			if dir := filepath.Dir(path); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					slog.Warn("Failed to create SSH host key dir", "dir", dir, "err", err)
				}
			}
			if err := os.WriteFile(path, pem.EncodeToMemory(blk), 0o600); err != nil {
				slog.Warn("Failed to persist SSH host key", "path", path, "err", err)
			} else {
				slog.Info("Generated new SSH host key", "path", path)
			}
		}
	}
	return signer, nil
}

// newSSHListenerConfig builds the listener parameters from server config.
func (s *Server) newSSHListenerConfig(addr string) (SSHListenerConfig, error) {
	hostKey, err := loadOrGenerateSSHHostKey(s.cfg.Server.SSHHostKey)
	if err != nil {
		return SSHListenerConfig{}, err
	}
	return SSHListenerConfig{
		Addr:     addr,
		User:     s.cfg.Server.SSHUser,
		Password: s.cfg.Server.SSHPassword,
		KeyAuth:  s.cfg.Server.SSHKeyAuth,
		HostKey:  hostKey,
	}, nil
}

// startExtraSSHListener starts an SSH beacon listener from a UI-created DB
// record. The key format is "ssh://host:port"; auth comes from server.ssh_*.
func (s *Server) startExtraSSHListener(key string) error {
	addr := key[len("ssh://"):]
	cfg, err := s.newSSHListenerConfig(addr)
	if err != nil {
		return fmt.Errorf("preparing SSH listener: %w", err)
	}
	sl := NewSSHBeaconListener(cfg)
	sl.SetHandler(s.makeBeaconHandler())
	if err := sl.Start(); err != nil {
		return err
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = sl
	s.extraListenersMu.Unlock()
	slog.Info("Extra SSH listener started", "addr", addr, "key", key)
	return nil
}
