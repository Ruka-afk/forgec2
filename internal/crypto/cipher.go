package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const (
	NonceSize   = 8
	KeySize     = 32
	MagicBytes  = "FC20"
)

var (
	ErrShortData = errors.New("cipher data too short")
	ErrBadMagic  = errors.New("invalid magic bytes")
)

// StreamCipher provides a simple stream cipher for C2 beacon payloads.
// Uses a shared 32-byte key with per-message random nonce.
// Keystream: SHA256(key || nonce || counter) expanded via AES-CTR-like chaining.
type StreamCipher struct {
	key [KeySize]byte
}

// NewStreamCipher creates a cipher with a 32-byte key.
// If key is nil or shorter than KeySize, a random key is generated.
func NewStreamCipher(key []byte) *StreamCipher {
	c := &StreamCipher{}
	if len(key) >= KeySize {
		copy(c.key[:], key[:KeySize])
	} else {
		rand.Read(c.key[:])
	}
	return c
}

// Encrypt encrypts plaintext. Returns: magic(4) + nonce(8) + ciphertext.
func (sc *StreamCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	keystream := sc.generateKeystream(nonce, len(plaintext))
	ciphertext := make([]byte, 0, 4+NonceSize+len(plaintext))
	ciphertext = append(ciphertext, []byte(MagicBytes)...)
	ciphertext = append(ciphertext, nonce...)
	for i, p := range plaintext {
		ciphertext = append(ciphertext, p^keystream[i])
	}
	return ciphertext, nil
}

// Decrypt decrypts data produced by Encrypt.
func (sc *StreamCipher) Decrypt(data []byte) ([]byte, error) {
	if len(data) < 4+NonceSize {
		return nil, ErrShortData
	}
	if string(data[:4]) != MagicBytes {
		return nil, ErrBadMagic
	}
	nonce := data[4 : 4+NonceSize]
	ciphertext := data[4+NonceSize:]

	keystream := sc.generateKeystream(nonce, len(ciphertext))
	plaintext := make([]byte, len(ciphertext))
	for i, c := range ciphertext {
		plaintext[i] = c ^ keystream[i]
	}
	return plaintext, nil
}

// generateKeystream produces a keystream of the given length using
// SHA256(nonce || key || counter) in a streaming fashion.
func (sc *StreamCipher) generateKeystream(nonce []byte, length int) []byte {
	keystream := make([]byte, 0, length)
	counter := uint32(0)
	h := sha256.New()
	for len(keystream) < length {
		h.Reset()
		h.Write(nonce)
		h.Write(sc.key[:])
		binary.LittleEndian.PutUint32(nonce[4:], counter)
		h.Write(nonce[4:8])
		keystream = append(keystream, h.Sum(nil)...)
		counter++
	}
	return keystream[:length]
}

// GetKey returns a copy of the cipher key
func (sc *StreamCipher) GetKey() []byte {
	k := make([]byte, KeySize)
	copy(k, sc.key[:])
	return k
}
