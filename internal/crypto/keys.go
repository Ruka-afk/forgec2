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

	// v3 per-implant registration secrets (see RegSecretStore). Each built
	// implant embeds a unique random secret instead of the fleet master key,
	// so extracting one binary no longer compromises the whole engagement.
	regSecretStoreSaltV3 = "forgec2-regstore-v3"

	// fileChainSalt / fileChainInfo derive the per-implant file-transfer
	// integrity key from the registration key (see DeriveFileChainKey).
	// The agent mirrors this in internal/payload/agent/agent_fileops.go.
	fileChainSalt = "forgec2-filechain-v1"
	fileChainInfo = "file-transfer"
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
// HMAC-SHA256(regKey, uuid || id_pub || ts || seq). The seq is bound into the
// MAC so a captured registration frame cannot be replayed with an inflated
// sequence number to burn the server-side replay window.
func ComputeRegHMAC(regKey []byte, agentID, identityPub string, ts int64, seq uint64) []byte {
	mac := hmac.New(sha256.New, regKey)
	mac.Write([]byte(agentID))
	mac.Write([]byte(identityPub))
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[7-i] = byte(ts >> (8 * i))
	}
	mac.Write(buf[:])
	for i := 0; i < 8; i++ {
		buf[7-i] = byte(seq >> (8 * i))
	}
	mac.Write(buf[:])
	return mac.Sum(nil)
}

// DeriveFileChainKey derives the per-implant key for the chunked file-transfer
// integrity chain from the agent's registration key. Both the server and the
// agent (standard-library HKDF mirror) derive the identical value.
func DeriveFileChainKey(regKey []byte) []byte {
	if len(regKey) == 0 {
		return nil
	}
	out := make([]byte, 32)
	r := hkdf.New(sha256.New, regKey, []byte(fileChainSalt), []byte(fileChainInfo))
	if _, err := io.ReadFull(r, out); err != nil {
		return nil
	}
	return out
}

// FileChunkMAC computes the next link of the chunked file-transfer integrity
// chain: HMAC-SHA256(chainKey, prevMAC || chunkData). The chain is seeded with
// 32 zero bytes for the first chunk, so any missing, reordered, or tampered
// chunk breaks the chain at the server.
func FileChunkMAC(chainKey, prevMAC, chunkData []byte) []byte {
	if len(chainKey) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, chainKey)
	mac.Write(prevMAC)
	mac.Write(chunkData)
	return mac.Sum(nil)
}

// Kill-switch derivation parameters. The agent mirrors these byte-for-byte
// (internal/payload/agent/cipher.go) — see agent_killswitch_test.go.
const (
	killSwitchSalt = "forgec2-killswitch-v1"
	killSwitchInfo = "killswitch"
)

// DeriveKillSwitchKey derives the per-implant key that authenticates the fleet
// kill-switch broadcast from the agent's registration key. Only a party holding
// the registration key (the server, and the implant itself) can produce or
// verify a kill-switch token.
func DeriveKillSwitchKey(regKey []byte) []byte {
	if len(regKey) == 0 {
		return nil
	}
	out := make([]byte, 32)
	r := hkdf.New(sha256.New, regKey, []byte(killSwitchSalt), []byte(killSwitchInfo))
	if _, err := io.ReadFull(r, out); err != nil {
		return nil
	}
	return out
}

// KillSwitchHMAC authenticates a kill-switch token: HMAC-SHA256(ksKey, token).
// The token is regenerated on every arm so old broadcasts cannot be replayed.
func KillSwitchHMAC(ksKey, token []byte) []byte {
	if len(ksKey) == 0 {
		return nil
	}
	mac := hmac.New(sha256.New, ksKey)
	mac.Write(token)
	return mac.Sum(nil)
}
