package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

// TestVerifyResponseMAC validates the v2 auth-response MAC verification:
// HMAC(regKey, agentUUID || seq || server_pub).
func TestVerifyResponseMAC(t *testing.T) {
	var savedKey = agentRegKey
	var savedUUID = agentUUID
	defer func() {
		agentRegKey = savedKey
		agentUUID = savedUUID
	}()

	agentRegKey = deriveAgentRegKey("aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899", "uuid-1")
	agentUUID = "11111111-2222-4333-8444-555555555555"
	pub := "base64serverpublickey"
	const seq = uint64(7)

	mac := hmac.New(sha256.New, agentRegKey)
	mac.Write([]byte(agentUUID))
	mac.Write([]byte("7"))
	mac.Write([]byte(pub))
	good := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !verifyResponseMAC(seq, pub, good) {
		t.Fatal("expected MAC to verify with matching pub")
	}
	if verifyResponseMAC(seq, "attackerpublickey", good) {
		t.Fatal("MAC should fail when the server pub key differs")
	}
	if verifyResponseMAC(seq+1, pub, good) {
		t.Fatal("MAC should fail when the seq differs")
	}
	if verifyResponseMAC(seq, pub, base64.StdEncoding.EncodeToString([]byte("garbage"))) {
		t.Fatal("MAC should fail for garbage input")
	}
	if verifyResponseMAC(seq, pub, "not-base64!") {
		t.Fatal("MAC should fail for non-base64 input")
	}
	if verifyResponseMAC(seq, pub, "") {
		t.Fatal("MAC should fail when MAC absent")
	}

	// No registration key => fail closed (never trust on first use).
	agentRegKey = nil
	if verifyResponseMAC(seq, pub, good) {
		t.Fatal("nil reg key must fail closed")
	}
}
