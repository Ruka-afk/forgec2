package server

import (
	"crypto/tls"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
)

type TLSFingerprintManager struct {
	mu        sync.RWMutex
	profiles  []utls.ClientHelloID
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
		profiles: []utls.ClientHelloID{
			utls.HelloChrome_Auto,
			utls.HelloFirefox_Auto,
			utls.HelloEdge_Auto,
			utls.HelloSafari_Auto,
		},
		enabled:   true,
		rotateDur: dur,
		rotateAt:  time.Now().Add(dur),
	}
	tfm.current = rand.Intn(len(tfm.profiles))
	return tfm
}

func (tfm *TLSFingerprintManager) GetClientHelloID() utls.ClientHelloID {
	if tfm == nil || !tfm.enabled {
		return utls.HelloRandomized
	}
	tfm.mu.RLock()
	defer tfm.mu.RUnlock()
	if time.Now().After(tfm.rotateAt) {
		go tfm.rotate()
	}
	return tfm.profiles[tfm.current]
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
	slog.Info("TLS fingerprint rotated", "profile", tfm.current)
}

func (tfm *TLSFingerprintManager) WrapTLSConfig(base *tls.Config) *tls.Config {
	if tfm == nil || !tfm.enabled {
		return base
	}
	wrapped := base.Clone()
	origGetCertificate := wrapped.GetCertificate

	wrapped.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if origGetCertificate != nil {
			return origGetCertificate(hello)
		}
		return nil, nil
	}

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
			slog.Info("TLS fingerprint randomization enabled",
				"jarm", s.cfg.TLSFingerprint.JARMEnabled,
				"ja3", s.cfg.TLSFingerprint.JA3Enabled,
				"rotate", s.cfg.TLSFingerprint.JARMRotate,
			)
		}
	}
}
