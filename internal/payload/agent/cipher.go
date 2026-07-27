package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
)

const (
	nonceSize  = 8
	keySize    = 32
	magicBytes = "FC20"
)

var (
	errShortData    = errors.New("cipher data too short")
	errBadMagic     = errors.New("invalid magic bytes")
	errNoSessionKey = errors.New("ECDH session not established")
)

// streamCipher is the legacy XOR stream cipher (backward compatible)
// Deprecated: Legacy XOR stream cipher provides no authentication.
// Use ECDH mode (CryptoKey="ecdh:") for AES-256-GCM authenticated encryption.
type streamCipher struct {
	key [keySize]byte
}

func newStreamCipher(key []byte) *streamCipher {
	c := &streamCipher{}
	if len(key) >= keySize {
		copy(c.key[:], key[:keySize])
	} else {
		rand.Read(c.key[:])
	}
	return c
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

// --- ECDH + AES-256-GCM Session (forward-secret encryption) ---

// ecdhSession manages a single ECDH session with the server
type ecdhSession struct {
	privateKey        *ecdh.PrivateKey
	sessionKey        []byte // AES-256-GCM key derived from ECDH shared secret
	msgCount          int
	rotationPending   bool   // agent key was rotated; include new pub key in next beacon
	rotationPubKeyB64 string // new public key to send
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
	return base64.StdEncoding.EncodeToString(es.privateKey.PublicKey().Bytes())
}

// establishFromServerKey completes the ECDH handshake using the server's public key
func (es *ecdhSession) establishFromServerKey(serverPubB64 string) error {
	curve := ecdh.X25519()
	serverPub, err := curve.NewPublicKey(decodeB64(serverPubB64))
	if err != nil {
		return err
	}
	sharedSecret, err := es.privateKey.ECDH(serverPub)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(sharedSecret)
	es.sessionKey = hash[:]
	es.msgCount = 0
	return nil
}

// encryptAESGCM encrypts plaintext with AES-256-GCM using the session key
// Returns: base64(nonce + ciphertext)
func (es *ecdhSession) encryptAESGCM(plaintext []byte) (string, error) {
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

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	es.msgCount++

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptAESGCM decrypts base64(nonce + ciphertext) with AES-256-GCM
func (es *ecdhSession) decryptAESGCM(encoded string) ([]byte, error) {
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
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	es.msgCount++
	return plaintext, nil
}

// needsKeyRotation checks if the session key should be rotated
func (es *ecdhSession) needsKeyRotation() bool {
	return es.sessionKey != nil && es.msgCount >= 100
}

// rotateKeyPair generates a new ECDH key pair for forward secrecy during rotation
func (es *ecdhSession) rotateKeyPair() error {
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	es.privateKey = privateKey
	es.rotationPubKeyB64 = es.publicKeyB64()
	es.rotationPending = true
	es.msgCount = 0
	return nil
}

// needsHandshake returns true if the session hasn't been established yet
func (es *ecdhSession) needsHandshake() bool {
	return es.sessionKey == nil
}

func decodeB64(s string) []byte {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return data
}
