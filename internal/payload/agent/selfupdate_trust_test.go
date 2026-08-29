package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// withPinnedKey temporarily sets the compile-time trust root and restores it.
func withPinnedKey(t *testing.T, hexKey string) {
	t.Helper()
	prev := updatePinnedPubKeyHex
	updatePinnedPubKeyHex = hexKey
	t.Cleanup(func() { updatePinnedPubKeyHex = prev })
}

func mustKeyPair(t *testing.T) (pubHex string, priv ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return hex.EncodeToString(pub), priv
}

// TestSelfUpdateRefusesWhenUnpinned pins the fail-closed contract: an implant
// built without a stamped trust root refuses self_update outright, even when
// the task carries a well-formed envelope.
func TestSelfUpdateRefusesWhenUnpinned(t *testing.T) {
	withPinnedKey(t, "")
	envelope, _ := json.Marshal(map[string]string{
		"url":       "https://c2.example/agent.exe",
		"signature": strings.Repeat("ab", 64),
	})
	if got := selfUpdate(string(envelope)); !strings.Contains(got, "no update signing key pinned") {
		t.Fatalf("got %q", got)
	}
}

// TestSelfUpdateBareURLGuidance covers the legacy bare-URL command: it must
// produce signed-envelope guidance instead of a cryptic JSON parse error.
func TestSelfUpdateBareURLGuidance(t *testing.T) {
	withPinnedKey(t, "aa")
	if got := selfUpdate("https://c2.example/agent.exe"); !strings.Contains(got, "bare URL") {
		t.Fatalf("got %q", got)
	}
}

// TestSelfUpdateBadSignatureLength rejects short signatures before any
// network or filesystem activity.
func TestSelfUpdateBadSignatureLength(t *testing.T) {
	withPinnedKey(t, "aa")
	envelope, _ := json.Marshal(map[string]string{
		"url": "https://c2.example/x", "signature": "aabb",
	})
	if got := selfUpdate(string(envelope)); !strings.Contains(got, "invalid signature length") {
		t.Fatalf("got %q", got)
	}
}

// TestVerifyUpdateSignatureRoundTrip exercises the exact production
// verification call against a real keypair, including tamper rejection.
func TestVerifyUpdateSignatureRoundTrip(t *testing.T) {
	pubHex, priv := mustKeyPair(t)
	digest := sha256.Sum256([]byte("fake next binary"))
	sig := ed25519.Sign(priv, digest[:])

	if !verifyUpdateSignature(pubHex, digest[:], sig) {
		t.Fatal("valid signature rejected")
	}
	bad := append([]byte{}, sig...)
	bad[0] ^= 0xFF
	if verifyUpdateSignature(pubHex, digest[:], bad) {
		t.Fatal("tampered signature accepted")
	}
}

// TestSelfUpdateDownloadFailurePath walks the envelope parse → pinned-key →
// download → verification sequence against a local server. The served bytes
// are deliberately signed with the digest of DIFFERENT content so the flow
// stops at verification failure — full pipeline coverage with zero file
// replacement side effects.
func TestSelfUpdateDownloadFailurePath(t *testing.T) {
	pubHex, priv := mustKeyPair(t)
	withPinnedKey(t, pubHex)

	served := []byte("this is not the binary you signed for")
	wrongDigest := sha256.Sum256([]byte("something else entirely"))
	sig := ed25519.Sign(priv, wrongDigest[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(served)
	}))
	defer srv.Close()

	envelope, _ := json.Marshal(map[string]string{"url": srv.URL + "/next.bin", "signature": hex.EncodeToString(sig)})
	if got := selfUpdate(string(envelope)); !strings.Contains(got, "signature verification failed") {
		t.Fatalf("got %q", got)
	}
	_ = protocol.TaskTypeShell // anchor: agent package shares protocol registry
}
