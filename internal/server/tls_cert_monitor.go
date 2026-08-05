package server

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

// TLSCertMonitor periodically probes the server's own HTTPS endpoint and
// tracks certificate stability over time. This is NOT a JARM validator:
// real JARM hashing requires the active handshake-probing algorithm the
// server implements externally. Here we only report when the served
// certificate fingerprint stays stable across rotations (an informational
// signal operators can use alongside real external JARM tooling).
type TLSCertMonitor struct {
	mu              sync.RWMutex
	observedHashes  map[string]int
	validationCount int
	enabled         bool
	checkInterval   time.Duration
	stopCh          chan struct{}
	startedMu       sync.Mutex
	started         bool
}

func NewTLSCertMonitor(enabled bool) *TLSCertMonitor {
	if !enabled {
		return nil
	}
	return &TLSCertMonitor{
		observedHashes: make(map[string]int),
		enabled:        true,
		checkInterval:  1 * time.Hour,
		stopCh:         make(chan struct{}),
	}
}

func (m *TLSCertMonitor) Start(server *Server) {
	if m == nil || !m.enabled {
		return
	}
	m.startedMu.Lock()
	if m.started {
		m.startedMu.Unlock()
		return
	}
	m.started = true
	m.startedMu.Unlock()
	go func() {
		ticker := time.NewTicker(m.checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.validate(server)
			}
		}
	}()
	slog.Info("TLS certificate stability monitor started", "interval", m.checkInterval)
}

func (m *TLSCertMonitor) Stop() {
	if m != nil {
		select {
		case <-m.stopCh:
		default:
			close(m.stopCh)
		}
	}
}

func (m *TLSCertMonitor) RecordCertHash(hash string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observedHashes[hash]++
	m.validationCount++
}

func (m *TLSCertMonitor) validate(server *Server) {
	if m == nil || !m.enabled {
		return
	}
	config := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}
	conn, err := tls.Dial("tcp", server.cfg.Server.Host+":"+itoaJARM(server.cfg.Server.Port), config)
	if err != nil {
		slog.Debug("TLS certificate probe failed (expected if no external access)", "err", err)
		return
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		h := sha256.Sum256(cert.Raw)
		hash := hex.EncodeToString(h[:])
		m.RecordCertHash(hash)
		m.mu.RLock()
		count := m.observedHashes[hash]
		m.mu.RUnlock()
		if count > 100 {
			slog.Warn("TLS certificate hash stable across many probes (expected for a self-signed deployment)",
				"hash_prefix", hash[:8], "count", count)
		}
	}
}

func (m *TLSCertMonitor) GetStats() map[string]int {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]int{
		"unique_hashes": len(m.observedHashes),
		"total_probes":  m.validationCount,
	}
}

func itoaJARM(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}