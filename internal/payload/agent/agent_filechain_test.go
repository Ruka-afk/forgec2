//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
)

// The agent's file-chain key derivation and chunk MAC MUST mirror the server's
// (internal/crypto.DeriveFileChainKey / FileChunkMAC) byte-for-byte, otherwise
// every chunked transfer fails integrity verification.

func TestFileChainKeyMirrorsServer(t *testing.T) {
	regKey := make([]byte, 32)
	for i := range regKey {
		regKey[i] = byte(i)
	}
	want := crypto.DeriveFileChainKey(regKey)
	if want == nil {
		t.Fatal("server DeriveFileChainKey returned nil")
	}
	got := hkdfSHA256(regKey, []byte("forgec2-filechain-v1"), []byte("file-transfer"))
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("agent chain key %x != server chain key %x", got, want)
	}
}

func TestFileChunkMACMirrorsServer(t *testing.T) {
	regKey := []byte("some-per-agent-reg-key-material")
	chainKey := hkdfSHA256(regKey, []byte("forgec2-filechain-v1"), []byte("file-transfer"))
	prev := make([]byte, 32)
	data := []byte("chunk payload")
	want := crypto.FileChunkMAC(chainKey, prev, data)
	got := fileChunkMAC(chainKey, prev, data)
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("agent MAC %x != server MAC %x", got, want)
	}
}

func TestFileChainVerifyAgentSide(t *testing.T) {
	orig := fileChainKeyDerived
	defer func() { fileChainKeyDerived = orig }()

	regKey := []byte("chain-test-reg-key")
	// Force chain key derivation from a deterministic reg key.
	RegSecretStr = base64.StdEncoding.EncodeToString(regKey) // 32 bytes? pad below
	if len(regKey) != 32 {
		// Ensure exactly 32 bytes for loadAgentRegKey to accept.
		padded := make([]byte, 32)
		copy(padded, regKey)
		RegSecretStr = base64.StdEncoding.EncodeToString(padded)
	}
	fileChainKeyDerived = nil
	defer func() { RegSecretStr = "" }()

	ck := fileChainKey()
	if ck == nil {
		t.Fatal("fileChainKey returned nil")
	}

	chunk := []byte("data-for-verification")
	// Server-computed expected link.
	exp := crypto.FileChunkMAC(ck, make([]byte, 32), chunk)
	expHex := hex.EncodeToString(exp)

	if err := verifyFileChunk(77, expHex, chunk); err != nil {
		t.Fatalf("verifyFileChunk valid: %v", err)
	}
	if prev := chainPrev(77); hex.EncodeToString(prev) != expHex {
		t.Errorf("chain did not advance: prev=%x want=%s", prev, expHex)
	}

	if err := verifyFileChunk(77, expHex, []byte("tampered")); err == nil {
		t.Fatal("verified tampered chunk")
	}
	if !strings.HasPrefix(errForVerify(77, expHex, []byte("tampered")), "chunk HMAC mismatch") {
		t.Error("expected mismatch error, got different error")
	}

	// No MAC supplied (legacy) is a no-op pass.
	if err := verifyFileChunk(78, "", chunk); err != nil {
		t.Fatalf("empty MAC should pass: %v", err)
	}
}

// errForVerify is a tiny wrapper so the error check above stays readable.
func errForVerify(taskID uint, expected string, data []byte) string {
	err := verifyFileChunk(taskID, expected, data)
	if err == nil {
		return ""
	}
	return err.Error()
}