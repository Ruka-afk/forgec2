package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"sync"
)

var (
	lootKey       []byte
	lootKeyOnce   sync.Once
	extc2Key      []byte
	extc2KeyOnce  sync.Once
)

// InitLootEncryption initializes the loot encryption key from the JWT secret.
func InitLootEncryption(jwtSecret string) {
	lootKeyOnce.Do(func() {
		h := sha256.Sum256([]byte(jwtSecret))
		lootKey = h[:32]
	})
}

// InitExtC2Encryption initializes a separate encryption key for ExtC2 channels,
// derived independently from the JWT secret to limit key compromise blast radius.
func InitExtC2Encryption(jwtSecret string) {
	extc2KeyOnce.Do(func() {
		h := sha256.Sum256([]byte("extc2:" + jwtSecret))
		extc2Key = h[:32]
	})
}

// EncryptLoot encrypts a plaintext string using AES-256-GCM.
func EncryptLoot(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if lootKey == nil {
		return "", errors.New("loot encryption not initialized")
	}
	block, err := aes.NewCipher(lootKey)
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
	if lootKey == nil {
		return "", errors.New("loot encryption not initialized")
	}
	// Detect encrypted vs legacy plaintext
	const marker = "FC2ENC:"
	if len(s) < len(marker) || s[:len(marker)] != marker {
		return s, nil // legacy plaintext
	}
	data, err := base64.StdEncoding.DecodeString(s[len(marker):])
	if err != nil {
		return s, nil // not valid ciphertext — treat as plaintext
	}
	block, err := aes.NewCipher(lootKey)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return s, nil // too short
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return s, nil // decryption failed — legacy plaintext
	}
	return string(plaintext), nil
}

func EncryptExtC2(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if extc2Key == nil {
		return "", errors.New("extc2 encryption not initialized")
	}
	block, err := aes.NewCipher(extc2Key)
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
	if extc2Key == nil {
		return "", errors.New("extc2 encryption not initialized")
	}
	const marker = "FC2EXT:"
	if len(s) < len(marker) || s[:len(marker)] != marker {
		return s, nil
	}
	data, err := base64.StdEncoding.DecodeString(s[len(marker):])
	if err != nil {
		return s, nil
	}
	block, err := aes.NewCipher(extc2Key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return s, nil
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return s, nil
	}
	return string(plaintext), nil
}
