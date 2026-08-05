package payload

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

func TestStageTokenRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	SetStagerKey(key)

	token, sig, keyHex, err := NewStageToken()
	if err != nil {
		t.Fatalf("NewStageToken: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("token length = %d, want 64", len(token))
	}
	if !VerifyStageSignature(token, sig) {
		t.Fatal("valid signature rejected")
	}
	if VerifyStageSignature(token, "tampered-sig") {
		t.Fatal("forged signature accepted")
	}
	if VerifyStageSignature("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", sig) {
		t.Fatal("signature accepted for a different token")
	}

	// Write + read the encrypted blob; confirm only ciphertext is persisted.
	plaintext := []byte("MZ fake stage-2 stage-2 stage-2")
	dir := t.TempDir()
	if err := WriteStage2Blob(dir, token, plaintext); err != nil {
		t.Fatalf("WriteStage2Blob: %v", err)
	}
	got, err := LoadStage2Blob(dir, token)
	if err != nil {
		t.Fatalf("LoadStage2Blob: %v", err)
	}
	if bytes.Equal(got, plaintext) {
		t.Fatal("stage blob stored in plaintext at rest")
	}
	if _, err := os.Stat(Stage2BlobPath(dir, token)); err != nil {
		t.Fatalf("blob path missing: %v", err)
	}

	// Derive the key and verify the round trip decrypts back to plaintext.
	k, err := DeriveStage2Key(token)
	if err != nil {
		t.Fatalf("DeriveStage2Key: %v", err)
	}
	if hex.EncodeToString(k) != keyHex {
		t.Fatal("keyHex does not match derived key")
	}
	dec, err := DecryptStage2Payload(got, k)
	if err != nil {
		t.Fatalf("DecryptStage2Payload: %v", err)
	}
	if !bytes.Equal(dec, plaintext) {
		t.Fatal("round trip mismatch")
	}

	// Decrypting with a different token's key must fail (AES-GCM authenticity).
	otherToken, _, _, err := NewStageToken()
	if err != nil {
		t.Fatalf("NewStageToken: %v", err)
	}
	otherKey, err := DeriveStage2Key(otherToken)
	if err != nil {
		t.Fatalf("DeriveStage2Key: %v", err)
	}
	if _, err := DecryptStage2Payload(got, otherKey); err == nil {
		t.Fatal("decryption with wrong key unexpectedly succeeded")
	}
}

func TestOriginFromC2URL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://10.0.0.1:8080/api/v1/beacon", "http://10.0.0.1:8080"},
		{"https://c2.example.com:8443/beacon?a=b", "https://c2.example.com:8443"},
		{"http://10.0.0.2:9999", "http://10.0.0.2:9999"},
		{"not-a-url", "not-a-url"},
	}
	for _, c := range cases {
		if got := OriginFromC2URL(c.in); got != c.want {
			t.Fatalf("OriginFromC2URL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}