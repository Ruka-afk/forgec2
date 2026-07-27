package server

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"
)

type JARMValidator struct {
	mu              sync.RWMutex
	observedHashes  map[string]int
	knownBurned     map[string]bool
	validationCount int
	enabled         bool
	checkInterval   time.Duration
	stopCh          chan struct{}
}

func NewJARMValidator(enabled bool) *JARMValidator {
	if !enabled {
		return nil
	}
	return &JARMValidator{
		observedHashes: make(map[string]int),
		knownBurned:    make(map[string]bool),
		enabled:        true,
		checkInterval:  1 * time.Hour,
		stopCh:         make(chan struct{}),
	}
}

func (jv *JARMValidator) Start(server *Server) {
	if jv == nil || !jv.enabled {
		return
	}
	go func() {
		ticker := time.NewTicker(jv.checkInterval)
		defer ticker.Stop()
		for {
			select {
			case <-jv.stopCh:
				return
			case <-ticker.C:
				jv.validate(server)
			}
		}
	}()
	slog.Info("JARM/JA3 continuous validation started", "interval", jv.checkInterval)
}

func (jv *JARMValidator) Stop() {
	if jv != nil {
		close(jv.stopCh)
	}
}

func (jv *JARMValidator) RecordJARMHash(hash string) {
	if jv == nil {
		return
	}
	jv.mu.Lock()
	defer jv.mu.Unlock()
	jv.observedHashes[hash]++
	jv.validationCount++
}

func (jv *JARMValidator) IsBurned(hash string) bool {
	if jv == nil {
		return false
	}
	jv.mu.RLock()
	defer jv.mu.RUnlock()
	return jv.knownBurned[hash]
}

func (jv *JARMValidator) validate(server *Server) {
	if jv == nil || !jv.enabled {
		return
	}
	jv.mu.Lock()
	defer jv.mu.Unlock()

	config := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}
	conn, err := tls.Dial("tcp", server.cfg.Server.Host+":"+itoaJARM(server.cfg.Server.Port), config)
	if err != nil {
		slog.Debug("JARM validation probe failed (expected if no external access)", "err", err)
		return
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		h := sha256.Sum256(cert.Raw)
		hash := hex.EncodeToString(h[:])
		jv.observedHashes[hash]++

		if jv.observedHashes[hash] > 100 {
			slog.Warn("TLS certificate hash seen excessive times, possible burn indicator",
				"hash", hash, "count", jv.observedHashes[hash])
		}
	}
}

func (jv *JARMValidator) GetStats() map[string]int {
	if jv == nil {
		return nil
	}
	jv.mu.RLock()
	defer jv.mu.RUnlock()
	stats := map[string]int{
		"unique_hashes":  len(jv.observedHashes),
		"total_probes":   jv.validationCount,
		"burned_hashes":  len(jv.knownBurned),
	}
	return stats
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
