package main

import (
	"encoding/hex"
	"testing"
)

func TestNewStreamCipherRequiresFullKey(t *testing.T) {
	if _, err := newStreamCipher(nil); err == nil {
		t.Fatal("newStreamCipher(nil) should error")
	}
	short := make([]byte, 16)
	if _, err := newStreamCipher(short); err == nil {
		t.Fatal("newStreamCipher(16-byte key) should error")
	}
	key := make([]byte, keySize)
	sc, err := newStreamCipher(key)
	if err != nil {
		t.Fatalf("newStreamCipher(32-byte key) should succeed: %v", err)
	}
	if sc.key != [keySize]byte{} {
		t.Fatal("key not copied")
	}
}

func TestStreamCipherRoundTrip(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	sc, err := newStreamCipher(key)
	if err != nil {
		t.Fatalf("newStreamCipher: %v", err)
	}
	plaintext := []byte("hello, stream cipher!")
	ct, err := sc.encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if string(ct[:4]) != magicBytes {
		t.Fatalf("missing magic bytes: %x", ct[:4])
	}
	pt, err := sc.decrypt(ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(pt) != string(plaintext) {
		t.Fatalf("round trip mismatch: %q != %q", pt, plaintext)
	}
}
