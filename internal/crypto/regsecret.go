package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
)

// RegSecretStore derives and (en/de)crypts v3 per-implant registration
// secrets. A v3 implant carries a unique random 32-byte registration secret
// compiled into its own binary, plus a public secret_id the server uses to
// look up the matching stored secret. The fleet master beacon key is no
// longer embedded in payloads, so extracting a single binary cannot derive
// the registration keys of any other implant.
//
// At rest, stored secrets are sealed with AES-256-GCM using a key derived
// from the master beacon key via HKDF (kept server-side only). This prevents
// a DB snapshot alone from exposing registration secrets in plaintext.
type RegSecretStore struct {
	storeKey []byte
}

// NewRegSecretStore builds the store's sealing key from the server master
// beacon key bytes. master may be nil when v3 is not configured; the store
// then refuses to seal/unseal.
func NewRegSecretStore(master []byte) *RegSecretStore {
	if len(master) == 0 {
		return &RegSecretStore{}
	}
	key := make([]byte, 32)
	r := hkdf.New(sha256.New, master, []byte(regSecretStoreSaltV3), nil)
	if _, err := io.ReadFull(r, key); err != nil {
		return &RegSecretStore{}
	}
	return &RegSecretStore{storeKey: key}
}

// GenerateSecret creates a new random 32-byte registration secret.
func (s *RegSecretStore) GenerateSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// Seal encrypts a registration secret for storage at rest. Returns "" when
// the store has no sealing key.
func (s *RegSecretStore) Seal(secret []byte) (string, error) {
	if s == nil || len(s.storeKey) == 0 {
		return "", errors.New("reg secret store not initialized")
	}
	if len(secret) == 0 {
		return "", nil
	}
	block, err := aes.NewCipher(s.storeKey)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, secret, nil)
	return "FC2REG:" + base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Unseal decrypts a sealed registration secret. Returns nil for an empty
// value or when the store has no sealing key.
func (s *RegSecretStore) Unseal(sealed string) ([]byte, error) {
	if s == nil || len(s.storeKey) == 0 {
		return nil, errors.New("reg secret store not initialized")
	}
	if sealed == "" {
		return nil, nil
	}
	const marker = "FC2REG:"
	if len(sealed) < len(marker) || sealed[:len(marker)] != marker {
		return nil, errors.New("invalid reg secret ciphertext")
	}
	data, err := base64.StdEncoding.DecodeString(sealed[len(marker):])
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(s.storeKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("reg secret ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}