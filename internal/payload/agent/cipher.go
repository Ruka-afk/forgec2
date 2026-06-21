package main

import (
	"crypto/rand"
	"crypto/sha256"
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
	errShortData = errors.New("cipher data too short")
	errBadMagic  = errors.New("invalid magic bytes")
)

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
