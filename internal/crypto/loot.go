package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	lootKeyMu sync.RWMutex
	lootKey   []byte

	extc2KeyMu sync.RWMutex
	extc2Key   []byte
)

// InitLootEncryption initializes (or re-initializes) the loot encryption key.
// It is intentionally re-entrant so that JWT rotation can safely derive a new
// key without requiring a process restart. Callers must ensure that when the
// JWT secret changes, existing ciphertext is re-encrypted (or a stable
// lootKeyHex is configured) before/after this call.
// If lootKeyHex is non-empty, uses that 32-byte hex key directly.
// Otherwise, derives the key from the JWT secret (backward compatible).
func InitLootEncryption(jwtSecret, lootKeyHex string) {
	lootKeyMu.Lock()
	defer lootKeyMu.Unlock()
	if lootKeyHex != "" {
		if b, err := hex.DecodeString(lootKeyHex); err == nil && len(b) == 32 {
			lootKey = b
			return
		}
	}
	h := sha256.Sum256([]byte(jwtSecret))
	lootKey = h[:32]
}

// InitExtC2Encryption initializes (or re-initializes) a separate encryption key
// for ExtC2 channels, derived independently to limit key compromise blast radius.
// If extc2KeyHex is non-empty, uses that 32-byte hex key directly (recommended).
// Otherwise derives the key from the JWT secret (backward compatible).
func InitExtC2Encryption(jwtSecret, extc2KeyHex string) {
	extc2KeyMu.Lock()
	defer extc2KeyMu.Unlock()
	if extc2KeyHex != "" {
		if b, err := hex.DecodeString(extc2KeyHex); err == nil && len(b) == 32 {
			extc2Key = b
			return
		}
	}
	h := sha256.Sum256([]byte("extc2:" + jwtSecret))
	extc2Key = h[:32]
}

// EncryptLoot encrypts a plaintext string using AES-256-GCM.
func EncryptLoot(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	lootKeyMu.RLock()
	key := lootKey
	lootKeyMu.RUnlock()
	if key == nil {
		return "", errors.New("loot encryption not initialized")
	}
	block, err := aes.NewCipher(key)
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
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	return "FC2ENC:" + base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

// DecryptLoot decrypts a ciphertext string using AES-256-GCM.
// Falls back to returning the raw string if it's not encrypted (backward compat
// with old plaintext credentials in the database).
func DecryptLoot(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	lootKeyMu.RLock()
	key := lootKey
	lootKeyMu.RUnlock()
	if key == nil {
		return "", errors.New("loot encryption not initialized")
	}
	// Detect encrypted vs legacy plaintext
	const marker = "FC2ENC:"
	if len(s) < len(marker) || s[:len(marker)] != marker {
		return s, nil // legacy plaintext
	}
	data, err := base64.StdEncoding.DecodeString(s[len(marker):])
	if err != nil {
		return "", fmt.Errorf("decryption failed: invalid ciphertext encoding: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("decryption failed: ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	return string(plaintext), nil
}

func EncryptExtC2(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	extc2KeyMu.RLock()
	key := extc2Key
	extc2KeyMu.RUnlock()
	if key == nil {
		return "", errors.New("extc2 encryption not initialized")
	}
	block, err := aes.NewCipher(key)
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
	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)
	return "FC2EXT:" + base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func DecryptExtC2(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	extc2KeyMu.RLock()
	key := extc2Key
	extc2KeyMu.RUnlock()
	if key == nil {
		return "", errors.New("extc2 encryption not initialized")
	}
	const marker = "FC2EXT:"
	if len(s) < len(marker) || s[:len(marker)] != marker {
		return s, nil
	}
	data, err := base64.StdEncoding.DecodeString(s[len(marker):])
	if err != nil {
		return "", fmt.Errorf("extc2 decryption failed: invalid ciphertext encoding: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("extc2 decryption failed: ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("extc2 decryption failed: %w", err)
	}
	return string(plaintext), nil
}
