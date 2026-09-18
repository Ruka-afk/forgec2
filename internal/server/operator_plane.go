package server

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// Operator-plane hardening: IP allowlist + mTLS for login and the
// authenticated operator surface (/api, pages, operator WS). Beacon
// ingestion (/th, malleable NoRoute fallthrough, /ws/beacon, extc2,
// payloads, phishing, health) is deliberately untouched — implants and
// redirectors must keep working from untrusted networks.

// operatorIPAllowed matches a client IP against operator_allowed_cidrs
// entries (each a CIDR or bare IP, v4/v6). Empty allowlist = disabled.
func operatorIPAllowed(allowlist []string, clientIP string) bool {
	if len(allowlist) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return false
	}
	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if pfx, err := netip.ParsePrefix(entry); err == nil {
			if pfx.Contains(addr) {
				return true
			}
			continue
		}
		if ip, err := netip.ParseAddr(entry); err == nil && ip == addr {
			return true
		}
	}
	return false
}

// loadOperatorCAPool reads the operator client-CA bundle once (restart to
// rotate). Failures leave the pool nil, and verification then fails closed.
func (s *Server) loadOperatorCAPool() {
	s.operatorCALoadOnce.Do(func() {
		path := ""
		if s.cfg != nil {
			path = s.cfg.Server.OperatorClientCAFile
		}
		if path == "" {
			return
		}
		pem, err := os.ReadFile(path)
		if err != nil {
			slog.Error("Operator mTLS: failed to read client CA file", "path", path, "err", err)
			return
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			slog.Error("Operator mTLS: no valid certs in client CA file", "path", path)
			return
		}
		s.operatorCAPool = pool
	})
}

// verifyOperatorCert validates a presented client certificate against the
// operator CA pool. The TLS layer only REQUESTS certs (beacons must keep
// working cert-less); enforcement happens here.
func (s *Server) verifyOperatorCert(state *tls.ConnectionState) bool {
	if state == nil || len(state.PeerCertificates) == 0 {
		return false
	}
	s.loadOperatorCAPool()
	pool := s.operatorCAPool
	if pool == nil {
		return false
	}
	intermediates := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	opts := x509.VerifyOptions{
		Roots:         pool,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err := state.PeerCertificates[0].Verify(opts); err != nil {
		slog.Warn("Operator mTLS: client certificate rejected", "err", err)
		return false
	}
	return true
}

// operatorPlaneGuard enforces operator_allowed_cidrs and operator_mtls on
// the operator surface. Both are fail-closed; both are no-ops when unset.
func (s *Server) operatorPlaneGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		var allowlist []string
		var mtls bool
		if s.cfg != nil {
			allowlist = s.cfg.Server.OperatorAllowedCIDRs
			mtls = s.cfg.Server.OperatorMTLS
		}
		if len(allowlist) > 0 && !operatorIPAllowed(allowlist, c.ClientIP()) {
			slog.Warn("Operator plane denied by IP allowlist", "ip", c.ClientIP(), "path", c.Request.URL.Path)
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "operator access denied from this network"})
			c.Abort()
			return
		}
		if mtls && !s.verifyOperatorCert(c.Request.TLS) {
			slog.Warn("Operator plane denied: no verified client certificate", "ip", c.ClientIP(), "path", c.Request.URL.Path)
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "operator client certificate required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
