package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/hkdf"
)

// Key derivation for the v2 beacon protocol. The shared beacon key is never
// transmitted on the wire; it only seeds per-agent registration keys via HKDF,
// and session keys are derived from the X25519 identity key agreement.
// The agent side re-implements the same HKDF-SHA256 derivation in
// internal/payload/agent/cipher.go using only the standard library, so both
// sides must keep these parameters identical.

const (
	regKeySalt  = "forgec2-reg-v2"
	sessKeySalt = "forgec2-session-v2"
)

// DeriveRegistrationKey derives the per-agent registration key from the
// master beacon key bytes. Master on the server is cfg.Server.BeaconKey
// (decoded hex); the agent derives the identical value from its compiled-in
// copy of the same key.
func DeriveRegistrationKey(master []byte, agentID string) []byte {
	if len(master) == 0 {
		return nil
	}
	out := make([]byte, 32)
	r := hkdf.New(sha256.New, master, []byte(regKeySalt), []byte(agentID))
	if _, err := io.ReadFull(r, out); err != nil {
		return nil
	}
	return out
}

// DeriveRegistrationKeyFromHex decodes a hex master key and derives the
// per-agent registration key. Returns nil for an invalid master key.
func DeriveRegistrationKeyFromHex(masterHex, agentID string) []byte {
	master, err := hex.DecodeString(masterHex)
	if err != nil || len(master) == 0 {
		return nil
	}
	return DeriveRegistrationKey(master, agentID)
}

// DeriveSessionKey derives the AES-256-GCM session key from the X25519 shared
// secret, bound to the agent identity (replaces the bare SHA-256 of the
// shared secret used by the v1 protocol).
func DeriveSessionKey(sharedSecret []byte, agentID string) []byte {
	if len(sharedSecret) == 0 {
		return nil
	}
	out := make([]byte, 32)
	r := hkdf.New(sha256.New, sharedSecret, []byte(sessKeySalt), []byte(agentID))
	if _, err := io.ReadFull(r, out); err != nil {
		return nil
	}
	return out
}

// ComputeRegHMAC authenticates a v2 registration handshake:
// HMAC-SHA256(regKey, uuid || id_pub || ts).
func ComputeRegHMAC(regKey []byte, agentID, identityPub string, ts int64) []byte {
	mac := hmac.New(sha256.New, regKey)
	mac.Write([]byte(agentID))
	mac.Write([]byte(identityPub))
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[7-i] = byte(ts >> (8 * i))
	}
	mac.Write(buf[:])
	return mac.Sum(nil)
}
