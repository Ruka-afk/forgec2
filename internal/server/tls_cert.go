package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// ── Hot-reloadable TLS server certificate ───────────────────────────────
// The loader caches the cert/key pair and re-reads the files whenever their
// mtime changes (checked per handshake — a stat call, no watcher goroutine).
// regenerate/upload handlers therefore take effect for NEW connections
// without a restart; established connections keep the old cert until they
// naturally recycle. mTLS ClientCAs stay static (restart to rotate those).

// tlsExpiryWarnDays / tlsExpiryCritDays bound the expiry monitor alerts.
const (
	tlsExpiryWarnDays = 30
	tlsExpiryCritDays = 7
)

type tlsCertLoader struct {
	certFile string
	keyFile  string

	mu       sync.RWMutex
	cert     *tls.Certificate
	certTime time.Time
	keyTime  time.Time
}

func newTLSCertLoader(certFile, keyFile string) *tlsCertLoader {
	return &tlsCertLoader{certFile: certFile, keyFile: keyFile}
}

// loadNow (re)reads the pair from disk, replacing the cache. Fails closed:
// a bad write never evicts a good cached cert.
func (l *tlsCertLoader) loadNow() error {
	cert, err := tls.LoadX509KeyPair(l.certFile, l.keyFile)
	if err != nil {
		return fmt.Errorf("loading TLS cert pair: %w", err)
	}
	ci, err := os.Stat(l.certFile)
	if err != nil {
		return err
	}
	ki, err := os.Stat(l.keyFile)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.cert = &cert
	l.certTime = ci.ModTime()
	l.keyTime = ki.ModTime()
	l.mu.Unlock()
	return nil
}

// GetCertificate satisfies tls.Config.GetCertificate: serve the cached pair,
// reloading when either file changed underneath us.
func (l *tlsCertLoader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	l.mu.RLock()
	cached := l.cert
	ct, kt := l.certTime, l.keyTime
	l.mu.RUnlock()
	if cached == nil {
		if err := l.loadNow(); err != nil {
			return nil, err
		}
		l.mu.RLock()
		cached = l.cert
		l.mu.RUnlock()
		if cached == nil {
			return nil, fmt.Errorf("no TLS certificate loaded")
		}
		return cached, nil
	}
	ci, err1 := os.Stat(l.certFile)
	ki, err2 := os.Stat(l.keyFile)
	if err1 != nil || err2 != nil || ci.ModTime().After(ct) || ki.ModTime().After(kt) {
		if err := l.loadNow(); err != nil {
			// Keep serving the stale-but-valid cached pair rather than
			// breaking every new handshake on a half-written file.
			slog.Warn("TLS cert reload failed, keeping cached pair", "err", err)
			return cached, nil
		}
		l.mu.RLock()
		cached = l.cert
		l.mu.RUnlock()
	}
	return cached, nil
}

// daysUntilExpiry parses the leaf cert from disk and reports whole days
// until NotAfter (negative when expired).
func (l *tlsCertLoader) daysUntilExpiry() (int, error) {
	pemBytes, err := os.ReadFile(l.certFile)
	if err != nil {
		return 0, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return 0, fmt.Errorf("no PEM block in %s", l.certFile)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return 0, err
	}
	return int(time.Until(cert.NotAfter).Hours() / 24), nil
}

// checkTLSCertExpiry logs and audits approaching expiry. Called at startup
// and daily: <30d warns, <7d is critical. Never fails startup by itself.
func (s *Server) checkTLSCertExpiry(context string) {
	if s == nil || s.tlsCerts == nil {
		return
	}
	days, err := s.tlsCerts.daysUntilExpiry()
	if err != nil {
		slog.Warn("TLS expiry check failed", "context", context, "err", err)
		return
	}
	switch {
	case days < 0:
		slog.Error("TLS certificate EXPIRED, beacons will fail TLS verification", "days_ago", -days, "context", context)
		s.LogAuditRecord(nil, "tls_cert_expired", "settings", "", "TLS certificate expired", false, nil)
	case days < tlsExpiryCritDays:
		slog.Error("TLS certificate expires within a week — regenerate immediately", "days_left", days, "context", context)
		s.LogAuditRecord(nil, "tls_cert_expiring", "settings", "", "TLS certificate expires within 7 days", true, nil)
	case days < tlsExpiryWarnDays:
		slog.Warn("TLS certificate expiring soon", "days_left", days, "context", context)
		s.LogAuditRecord(nil, "tls_cert_expiring", "settings", "", "TLS certificate expires within 30 days", true, nil)
	}
}

// startTLSCertMonitor runs the daily expiry check alongside other loops.
func (s *Server) startTLSCertMonitor() {
	if s.ctx == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.checkTLSCertExpiry("daily")
			}
		}
	}()
}
