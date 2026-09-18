package server

import (
	"crypto/tls"
	"slices"
	"testing"
	"time"
)

func TestTLSFingerprintDisabled(t *testing.T) {
	if tfm := NewTLSFingerprintManager(false, false, "", ""); tfm != nil {
		t.Fatal("disabled manager must be nil")
	}
	var nilTFM *TLSFingerprintManager
	if got := nilTFM.CurrentProfile(); got != "none" {
		t.Fatalf("nil manager profile=%q, want none", got)
	}
	base := &tls.Config{MinVersion: tls.VersionTLS12}
	if got := nilTFM.Live(base); got != base {
		t.Fatal("nil manager Live must return base untouched")
	}
}

func TestTLSFingerprintNamedProfilePins(t *testing.T) {
	tfm := NewTLSFingerprintManager(true, true, "24h", "firefox")
	if tfm == nil {
		t.Fatal("manager must exist")
	}
	if len(tfm.pool) != 1 {
		t.Fatalf("pinned pool size=%d, want 1", len(tfm.pool))
	}
	before := tfm.CurrentProfile()
	tfm.rotateAt = time.Now().Add(-time.Second)
	tfm.rotate()
	if got := tfm.CurrentProfile(); got != before || got != "firefox" {
		t.Fatalf("pinned profile rotated %q -> %q", before, got)
	}
}

func TestTLSFingerprintRandomPoolCoversAll(t *testing.T) {
	for _, name := range []string{"", "random", "opera-bogus"} {
		tfm := NewTLSFingerprintManager(true, false, "24h", name)
		if tfm == nil {
			t.Fatalf("manager must exist for %q", name)
		}
		if len(tfm.pool) != len(tlsProfiles) {
			t.Fatalf("pool for %q size=%d, want %d", name, len(tfm.pool), len(tlsProfiles))
		}
	}
}

func TestTLSFingerprintRotateChangesProfile(t *testing.T) {
	tfm := NewTLSFingerprintManager(true, true, "24h", "")
	if tfm == nil {
		t.Fatal("manager must exist")
	}
	before := tfm.CurrentProfile()
	tfm.rotateAt = time.Now().Add(-time.Second)
	tfm.rotate()
	if got := tfm.CurrentProfile(); got == before {
		t.Fatal("rotation must switch to a different profile")
	}
	if time.Until(tfm.rotateAt) <= 0 {
		t.Fatal("rotation must extend the deadline")
	}
}

func TestTLSFingerprintBadIntervalDefaults(t *testing.T) {
	for _, iv := range []string{"", "bogus", "0s", "-1h"} {
		tfm := NewTLSFingerprintManager(true, true, iv, "")
		if tfm == nil {
			t.Fatalf("manager must exist for %q", iv)
		}
		if tfm.rotateDur != 24*time.Hour {
			t.Fatalf("interval %q: rotateDur=%v, want 24h", iv, tfm.rotateDur)
		}
	}
}

func TestTLSFingerprintLiveTracksRotation(t *testing.T) {
	tfm := NewTLSFingerprintManager(true, true, "24h", "")
	if tfm == nil {
		t.Fatal("manager must exist")
	}
	base := &tls.Config{MinVersion: tls.VersionTLS12}
	shim := tfm.Live(base)
	if shim == base {
		t.Fatal("Live must return a shim, not base")
	}
	if shim.GetConfigForClient == nil {
		t.Fatal("shim must negotiate per handshake")
	}
	first, err := shim.GetConfigForClient(nil)
	if err != nil {
		t.Fatalf("per-handshake config: %v", err)
	}
	// Force rotation to a different profile and confirm the NEXT handshake
	// sees it (the old one-time clone never did). Full-slice compare: every
	// profile pair must differ observably or rotation is theater.
	tfm.rotateAt = time.Now().Add(-time.Second)
	second, err := shim.GetConfigForClient(nil)
	if err != nil {
		t.Fatalf("per-handshake config: %v", err)
	}
	if slices.Equal(first.CipherSuites, second.CipherSuites) {
		t.Fatal("handshake config did not track rotation")
	}
	if len(second.Certificates) != 0 {
		t.Fatal("per-handshake clone must not invent certificates")
	}
}

// TestTLSProfilesPairwiseDistinct guards rotation effectiveness: every
// profile pair must differ in cipher order, or rotating between them changes
// nothing on the wire (edge/safari shipped identical once).
func TestTLSProfilesPairwiseDistinct(t *testing.T) {
	for i := 0; i < len(tlsProfiles); i++ {
		for j := i + 1; j < len(tlsProfiles); j++ {
			if slices.Equal(tlsProfiles[i].cipherSuites, tlsProfiles[j].cipherSuites) {
				t.Fatalf("profiles %q and %q are identical", tlsProfiles[i].name, tlsProfiles[j].name)
			}
		}
	}
}

func TestTLSFingerprintSnapshotIsOneShot(t *testing.T) {
	tfm := NewTLSFingerprintManager(true, true, "24h", "chrome")
	if tfm == nil {
		t.Fatal("manager must exist")
	}
	base := &tls.Config{MinVersion: tls.VersionTLS12}
	snap := tfm.Snapshot(base)
	if snap == base {
		t.Fatal("Snapshot must return a clone")
	}
	if snap.GetConfigForClient != nil {
		t.Fatal("Snapshot must not negotiate per handshake (QUIC ignores it)")
	}
	if len(snap.CipherSuites) == 0 || snap.CipherSuites[0] != tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 {
		t.Fatalf("chrome snapshot cipher head wrong: %+v", snap.CipherSuites[:3])
	}
}
