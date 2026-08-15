//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestSelfUpdatePinnedKeyTrustRoot verifies that when a compile-time pinned
// update key is configured, self_update verifies ONLY against it and ignores
// any key supplied in the task — closing the "any task issuer can run code"
// gap.
func TestSelfUpdatePinnedKeyTrustRoot(t *testing.T) {
	goodPub, goodPriv, _ := ed25519.GenerateKey(rand.Reader)
	sum := sha256.Sum256([]byte("implant-binary-bytes"))
	hash := sum[:]
	sig := ed25519.Sign(goodPriv, hash)

	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)

	orig := updatePinnedPubKeyHex
	defer func() { updatePinnedPubKeyHex = orig }()
	updatePinnedPubKeyHex = hex.EncodeToString(goodPub)

	if !verifyUpdateSignature(updatePinnedPubKeyHex, hash, sig) {
		t.Fatal("pinned key should verify its own signature")
	}
	// A different (task-supplied) key must NOT be accepted while pinned.
	if verifyUpdateSignature(hex.EncodeToString(wrongPub), hash, sig) {
		t.Fatal("task-supplied key must be ignored when a pinned key is set")
	}
}
