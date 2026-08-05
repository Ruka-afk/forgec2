//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
)

// The agent's kill-switch key derivation MUST mirror the server's
// (internal/crypto.DeriveKillSwitchKey / KillSwitchHMAC) byte-for-byte,
// otherwise every armed beacon response would be rejected as forged and the
// fleet could never be torn down via broadcast.

func TestKillSwitchKeyMirrorsServer(t *testing.T) {
	regKey := make([]byte, 32)
	for i := range regKey {
		regKey[i] = byte(255 - i)
	}
	want := crypto.DeriveKillSwitchKey(regKey)
	if want == nil {
		t.Fatal("server DeriveKillSwitchKey returned nil")
	}
	got := hkdfSHA256(regKey, []byte(killSwitchSalt), []byte(killSwitchInfo))
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("agent kill-switch key %x != server key %x", got, want)
	}
}

func TestKillSwitchMACMirrorsServer(t *testing.T) {
	regKey := []byte("kill-switch-mirror-reg-key")
	ksKey := hkdfSHA256(regKey, []byte(killSwitchSalt), []byte(killSwitchInfo))
	token := []byte("some-random-per-arm-token")
	want := crypto.KillSwitchHMAC(crypto.DeriveKillSwitchKey(regKey), token)
	mac := hmac.New(sha256.New, ksKey)
	mac.Write(token)
	got := mac.Sum(nil)
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Errorf("agent MAC %x != server MAC %x", got, want)
	}
}

func TestVerifyKillSwitch(t *testing.T) {
	regKey := make([]byte, 32)
	for i := range regKey {
		regKey[i] = byte(i * 3)
	}
	orig := agentRegKey
	agentRegKey = regKey
	defer func() { agentRegKey = orig }()

	ksKey := crypto.DeriveKillSwitchKey(regKey)
	token := make([]byte, 32)
	for i := range token {
		token[i] = byte(0xAA ^ i)
	}
	tokenHex := hex.EncodeToString(token)
	macHex := hex.EncodeToString(crypto.KillSwitchHMAC(ksKey, token))

	if !verifyKillSwitch(tokenHex, macHex) {
		t.Fatal("valid kill-switch broadcast rejected")
	}
	// Tampered token must fail.
	if verifyKillSwitch(tokenHex, hex.EncodeToString(crypto.KillSwitchHMAC(ksKey, []byte("other-token")))) {
		t.Fatal("accepted broadcast for a different token")
	}
	// Tampered MAC must fail.
	if verifyKillSwitch(tokenHex, macHex[:len(macHex)-2]+"00") {
		t.Fatal("accepted tampered MAC")
	}
	// Malformed inputs must fail, never panic.
	if verifyKillSwitch("zz", macHex) {
		t.Fatal("accepted malformed token")
	}
	if verifyKillSwitch(tokenHex, "deadbeef") {
		t.Fatal("accepted short MAC")
	}
	if verifyKillSwitch("", "") {
		t.Fatal("accepted empty broadcast")
	}
	agentRegKey = nil
	if verifyKillSwitch(tokenHex, macHex) {
		t.Fatal("accepted broadcast without reg key")
	}
}
