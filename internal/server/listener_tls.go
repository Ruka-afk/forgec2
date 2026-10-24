package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
)

// ── Per-listener TLS posture ────────────────────────────────────────────────
//
// Extra https/tls listeners used to share the server-wide beacon TLS posture
// (server.client_ca_file / server.require_client_cert) with no way to tighten
// a single termination point: a lab C2 that also needs a hardened listener
// either ran mTLS everywhere or nowhere.
//
// The per-listener knobs can only *strengthen* the server-wide posture, never
// relax it: a listener may add a client-certificate requirement, point at its
// own client CA, or raise the TLS floor to 1.3 - but it can never drop the
// server-wide client-cert requirement or go below the TLS 1.2 floor. That
// keeps the fail-closed property the shared posture exists for while allowing
// mixed deployments.

// listenerTLSOverrides are the resolved per-listener TLS knobs. Zero values
// inherit the server-wide posture.
type listenerTLSOverrides struct {
	ClientCAFile      string
	RequireClientCert bool
	MinTLSVersion     string
}

// listenerTLSOverridesFor reads the knobs off a DB row (nil-safe so startup
// paths that only have a key can keep passing nil).
func listenerTLSOverridesFor(l *db.Listener) listenerTLSOverrides {
	if l == nil {
		return listenerTLSOverrides{}
	}
	return listenerTLSOverrides{
		ClientCAFile:      strings.TrimSpace(l.ClientCAFile),
		RequireClientCert: l.RequireClientCert,
		MinTLSVersion:     strings.TrimSpace(l.MinTLSVersion),
	}
}

// minTLSVersionFloor maps the stored/operator spelling to a tls.Version*
// floor. An empty value inherits the server-wide floor (TLS 1.2). ok=false
// means the spelling is invalid and must be rejected at the API boundary
// instead of silently degrading to the default.
func minTLSVersionFloor(v string) (floor uint16, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "1.2", "tls1.2", "tls12", "12":
		return tls.VersionTLS12, true
	case "1.3", "tls1.3", "tls13", "13":
		return tls.VersionTLS13, true
	default:
		return 0, false
	}
}

// loadClientCAPool parses a PEM CA bundle used to verify client certificates.
func loadClientCAPool(path string) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading client CA file %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, fmt.Errorf("client CA file %q contains no usable PEM certificates", path)
	}
	return pool, nil
}

// effectiveClientAuth merges the server-wide and per-listener client-cert
// requirement. The requirement is additive (either side can demand it) and the
// CA resolves per listener first: a listener that brings its own client CA
// verifies against that bundle, everything else falls back to the server-wide
// one. When certificates are required but no CA is configured anywhere the
// caller gets an error - failing the listener start is the only safe answer,
// serving without the requested check is not.
func (s *Server) effectiveClientAuth(o listenerTLSOverrides) (bool, *x509.CertPool, error) {
	s.configMu.RLock()
	serverRequire, serverCA := false, ""
	if s.cfg != nil {
		serverRequire = s.cfg.Server.RequireClientCert
		serverCA = strings.TrimSpace(s.cfg.Server.ClientCAFile)
	}
	s.configMu.RUnlock()
	require := o.RequireClientCert || serverRequire
	if !require {
		return false, nil, nil
	}
	caFile := strings.TrimSpace(o.ClientCAFile)
	if caFile == "" {
		caFile = serverCA
	}
	if caFile == "" {
		return false, nil, fmt.Errorf("client certificates are required but no client CA is configured (listener client_ca_file or server.client_ca_file)")
	}
	pool, err := loadClientCAPool(caFile)
	if err != nil {
		return false, nil, fmt.Errorf("loading client CA for mTLS: %w", err)
	}
	return true, pool, nil
}

// listenerRequiresClientCert reports whether a listener ends up requiring
// verified client certificates once the per-listener knob is merged with the
// server-wide one. The health prober uses this to avoid reporting a healthy
// mTLS listener as broken: the probe presents no client certificate, so a real
// handshake can only fail.
func (s *Server) listenerRequiresClientCert(l *db.Listener) bool {
	if l != nil && l.RequireClientCert {
		return true
	}
	s.configMu.RLock()
	required := s.cfg != nil && s.cfg.Server.RequireClientCert
	s.configMu.RUnlock()
	return required
}

// validateListenerTLSShape rejects TLS knobs that cannot be honoured, at the
// API boundary: they only apply to https/tls listeners, min_tls_version must
// be 1.2 or 1.3, a per-listener CA file must parse, and requiring client
// certificates without any CA (per listener or server-wide) is refused so the
// row can never describe a listener that would come up without the check.
func (s *Server) validateListenerTLSShape(scheme string, o listenerTLSOverrides) error {
	asked := o.ClientCAFile != "" || o.RequireClientCert || o.MinTLSVersion != ""
	if !asked {
		return nil
	}
	if scheme != "https" && scheme != "tls" {
		return fmt.Errorf("TLS settings (client_ca_file/require_client_cert/min_tls_version) only apply to https or tls listeners, not %q", scheme)
	}
	if _, ok := minTLSVersionFloor(o.MinTLSVersion); !ok {
		return fmt.Errorf("min_tls_version must be \"1.2\" or \"1.3\", got %q", o.MinTLSVersion)
	}
	if o.ClientCAFile != "" {
		if _, err := loadClientCAPool(o.ClientCAFile); err != nil {
			return err
		}
	}
	if o.RequireClientCert && o.ClientCAFile == "" {
		s.configMu.RLock()
		serverCA := ""
		if s.cfg != nil {
			serverCA = strings.TrimSpace(s.cfg.Server.ClientCAFile)
		}
		s.configMu.RUnlock()
		if serverCA == "" {
			return fmt.Errorf("require_client_cert needs a client CA: set client_ca_file on the listener or server.client_ca_file")
		}
	}
	return nil
}
