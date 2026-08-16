package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"testing"
)

// TestPerTaskKeyRoundTrip proves the per-task result encryption (agent side)
// decrypts correctly on the server side: the agent seals Output with the
// operator-issued AES-256-GCM key (nonce prepended), and decryptTaskKeyOutput
// recovers it. This mirrors internal/payload/agent.applyTaskKeyEncryption.
func TestPerTaskKeyRoundTrip(t *testing.T) {
	keyB64, err := generateTaskKey()
	if err != nil {
		t.Fatalf("generateTaskKey: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(raw) != 32 {
		t.Fatalf("key not 32 bytes: %v", err)
	}

	plaintext := "SECRET-CRED-OUTPUT-12345"

	// Agent-side seal (mirror of applyTaskKeyEncryption).
	block, err := aes.NewCipher(raw)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	sealed := base64.StdEncoding.EncodeToString(ct)

	// Server-side open.
	out, err := decryptTaskKeyOutput(keyB64, sealed)
	if err != nil {
		t.Fatalf("decryptTaskKeyOutput: %v", err)
	}
	if out != plaintext {
		t.Fatalf("round-trip mismatch: got %q want %q", out, plaintext)
	}

	// Wrong key must fail.
	if _, err := decryptTaskKeyOutput(base64.StdEncoding.EncodeToString(make([]byte, 32)), sealed); err == nil {
		t.Fatal("expected decryption failure with wrong key")
	}
}
