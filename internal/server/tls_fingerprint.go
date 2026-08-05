package server

import (
	"crypto/tls"
	"log/slog"
	"math/rand"
	"sync"
	"time"
)

// tlsProfile is a real, externally-observable server-side TLS fingerprint:
// an ordered cipher-suite preference list, a version range, and ALPN
// next-protos. Rotating between browser-like profiles changes what a
// JA3/JARM scanner sees on the wire. (uTLS cannot be applied server-side;
// the previous implementation claimed randomization but only cloned the
// config, so the fingerprint never actually changed.)
type tlsProfile struct {
	name         string
	minVersion   uint16
	maxVersion   uint16
	cipherSuites []uint16
	nextProtos   []string
}

func (p tlsProfile) tlsConfig() *tls.Config {
	return &tls.Config{
		MinVersion:   p.minVersion,
		MaxVersion:   p.maxVersion,
		CipherSuites: p.cipherSuites,
		NextProtos:   p.nextProtos,
	}
}

// Browser-like profiles. Ordering of the TLS 1.3 suites is advisory in Go
// (GODEBUG defaults apply), but TLS 1.2 suite order and the version range are
// faithfully rendered in the ServerHello, which is what JA3/JARM observe.
var tlsProfiles = []tlsProfile{
	{
		name:       "chrome",
		minVersion: tls.VersionTLS12,
		maxVersion: tls.VersionTLS13,
		cipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		nextProtos: []string{"h2", "http/1.1"},
	},
	{
		name:       "firefox",
		minVersion: tls.VersionTLS12,
		maxVersion: tls.VersionTLS13,
		cipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		nextProtos: []string{"h2", "http/1.1"},
	},
	{
		name:       "edge",
		minVersion: tls.VersionTLS12,
		maxVersion: tls.VersionTLS13,
		cipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		nextProtos: []string{"h2", "http/1.1"},
	},
	{
		name:       "safari",
		minVersion: tls.VersionTLS12,
		maxVersion: tls.VersionTLS13,
		cipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		nextProtos: []string{"h2", "http/1.1"},
	},
}

type TLSFingerprintManager struct {
	mu        sync.RWMutex
	profiles  []tlsProfile
	current   int
	rotateAt  time.Time
	enabled   bool
	rotateDur time.Duration
}

func NewTLSFingerprintManager(jarmEnabled, ja3Enabled bool, rotateInterval string) *TLSFingerprintManager {
	if !jarmEnabled && !ja3Enabled {
		return nil
	}

	dur := 24 * time.Hour
	if rotateInterval != "" {
		if parsed, err := time.ParseDuration(rotateInterval); err == nil {
			dur = parsed
		}
	}

	tfm := &TLSFingerprintManager{
		profiles:  tlsProfiles,
		enabled:   true,
		rotateDur: dur,
		rotateAt:  time.Now().Add(dur),
	}
	tfm.current = rand.Intn(len(tfm.profiles))
	return tfm
}

// CurrentProfile returns the active profile (for tests and stats).
func (tfm *TLSFingerprintManager) CurrentProfile() string {
	if tfm == nil || !tfm.enabled {
		return "none"
	}
	tfm.mu.RLock()
	defer tfm.mu.RUnlock()
	if time.Now().After(tfm.rotateAt) {
		go tfm.rotate()
	}
	return tfm.profiles[tfm.current].name
}

func (tfm *TLSFingerprintManager) rotate() {
	tfm.mu.Lock()
	defer tfm.mu.Unlock()
	if time.Now().Before(tfm.rotateAt) {
		return
	}
	prev := tfm.current
	for tfm.current == prev && len(tfm.profiles) > 1 {
		tfm.current = rand.Intn(len(tfm.profiles))
	}
	tfm.rotateAt = time.Now().Add(tfm.rotateDur)
	slog.Info("TLS fingerprint rotated", "profile", tfm.profiles[tfm.current].name)
}

// WrapTLSConfig applies the current browser-like profile to the server TLS
// config: version range, ordered cipher suites, and ALPN. The returned
// config is a clone; the caller's base config is untouched.
func (tfm *TLSFingerprintManager) WrapTLSConfig(base *tls.Config) *tls.Config {
	if tfm == nil || !tfm.enabled {
		return base
	}
	tfm.mu.RLock()
	p := tfm.profiles[tfm.current]
	tfm.mu.RUnlock()

	wrapped := base.Clone()
	profileCfg := p.tlsConfig()
	wrapped.MinVersion = profileCfg.MinVersion
	wrapped.MaxVersion = profileCfg.MaxVersion
	wrapped.CipherSuites = profileCfg.CipherSuites
	wrapped.NextProtos = profileCfg.NextProtos
	return wrapped
}

func (s *Server) initTLSFingerprint() {
	if s.cfg.TLSFingerprint.JARMEnabled || s.cfg.TLSFingerprint.JA3Enabled {
		s.tlsFingerprint = NewTLSFingerprintManager(
			s.cfg.TLSFingerprint.JARMEnabled,
			s.cfg.TLSFingerprint.JA3Enabled,
			s.cfg.TLSFingerprint.JARMRotate,
		)
		if s.tlsFingerprint != nil {
			slog.Info("Server TLS fingerprint rotation enabled (cipher order/version/ALPN)",
				"profile", s.tlsFingerprint.CurrentProfile(),
				"rotate", s.cfg.TLSFingerprint.JARMRotate,
			)
		}
	}
}
