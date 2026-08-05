package crypto

import (
	"testing"
)

func TestInitLootEncryption(t *testing.T) {
	InitLootEncryption("test-secret-key-for-testing-only", "")
	lootKeyMu.RLock()
	initialized := lootKey != nil
	lootKeyMu.RUnlock()
	if !initialized {
		t.Fatal("lootKey should be initialized")
	}
}

func TestInitLootEncryptionReentrant(t *testing.T) {
	InitLootEncryption("first-secret", "")
	encA, err := EncryptLoot("secret-value")
	if err != nil {
		t.Fatalf("EncryptLoot failed: %v", err)
	}

	// Re-init with a different derived secret; a re-entrant init must take effect.
	InitLootEncryption("second-secret", "")
	if dec, err := DecryptLoot(encA); err == nil {
		t.Fatalf("ciphertext from the old key should fail to decrypt after re-init, got %q", dec)
	}

	encB, err := EncryptLoot("secret-value")
	if err != nil {
		t.Fatalf("EncryptLoot failed after re-init: %v", err)
	}
	decB, err := DecryptLoot(encB)
	if err != nil {
		t.Fatalf("round-trip with new key failed: %v", err)
	}
	if decB != "secret-value" {
		t.Fatalf("round-trip mismatch: got %q", decB)
	}
}

func TestInitLootEncryptionExplicitKey(t *testing.T) {
	// Explicit key must be honored and stable across re-init calls.
	const hexKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	InitLootEncryption("secret-A", hexKey)
	encA, err := EncryptLoot("value")
	if err != nil {
		t.Fatalf("EncryptLoot failed: %v", err)
	}
	InitLootEncryption("secret-B", hexKey)
	if dec, err := DecryptLoot(encA); err != nil || dec != "value" {
		t.Fatalf("explicit key must remain stable across JWT changes, dec=%q err=%v", dec, err)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	InitLootEncryption("test-secret-key-for-testing-only", "")
	plaintext := "P@ssw0rd!123"
	enc, err := EncryptLoot(plaintext)
	if err != nil {
		t.Fatalf("EncryptLoot failed: %v", err)
	}
	if enc == "" {
		t.Fatal("encrypted string should not be empty")
	}

	dec, err := DecryptLoot(enc)
	if err != nil {
		t.Fatalf("DecryptLoot failed: %v", err)
	}
	if dec != plaintext {
		t.Fatalf("round-trip mismatch: got %q, want %q", dec, plaintext)
	}
}

func TestEncryptEmptyString(t *testing.T) {
	InitLootEncryption("test-secret-key-for-testing-only", "")
	enc, err := EncryptLoot("")
	if err != nil {
		t.Fatalf("EncryptLoot empty should not error: %v", err)
	}
	if enc != "" {
		t.Fatal("encrypted empty string should be empty")
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	InitLootEncryption("test-secret-key-for-testing-only", "")
	// Legacy plaintext credentials should pass through
	dec, err := DecryptLoot("old-plaintext-password")
	if err != nil {
		t.Fatalf("DecryptLoot legacy plaintext should not error: %v", err)
	}
	if dec != "old-plaintext-password" {
		t.Fatalf("legacy passthrough failed: got %q", dec)
	}
}

func TestDecryptInvalidFormat(t *testing.T) {
	InitLootEncryption("test-secret-key-for-testing-only", "")
	_, err := DecryptLoot("FC2ENC:!!!invalid-base64!!!")
	if err == nil {
		t.Fatal("DecryptLoot invalid format should return error")
	}
}
