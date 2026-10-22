package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestVerifySelfIntegrityDataNonPE(t *testing.T) {
	marker := strings.Repeat("a", 64)
	data := []byte("agent-payload-before-" + marker + "-after")
	idx := bytes.Index(data, []byte(marker))
	calc := append([]byte(nil), data...)
	for i := idx; i < idx+len(marker); i++ {
		calc[i] = 0
	}
	sum := sha256.Sum256(calc)
	expected := hex.EncodeToString(sum[:])
	copy(data[idx:idx+64], []byte(expected))

	if err := verifySelfIntegrityData(data, expected); err != nil {
		t.Fatalf("valid image rejected: %v", err)
	}
	tampered := append([]byte(nil), data...)
	tampered[0] ^= 0xFF
	if err := verifySelfIntegrityData(tampered, expected); err == nil {
		t.Fatal("tampered image accepted")
	}
}

func TestVerifySelfIntegrityDataFailsClosed(t *testing.T) {
	marker := strings.Repeat("a", 64)
	expected := strings.Repeat("b", 64)
	data := []byte("one-" + expected + "-two-" + expected)
	if err := verifySelfIntegrityData(data, expected); err == nil {
		t.Fatal("duplicate marker accepted")
	}
	if err := verifySelfIntegrityData([]byte("short"), "abc"); err == nil {
		t.Fatal("short digest accepted")
	}
	// A malformed PE must fail instead of silently hashing the whole file.
	if err := verifySelfIntegrityData([]byte("MZ"+marker), marker); err == nil {
		t.Fatal("malformed PE accepted")
	}
}
