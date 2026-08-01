package crypto

import (
	"testing"
)

func TestInitLootEncryption(t *testing.T) {
	InitLootEncryption("test-secret-key-for-testing-only", "")
	if lootKey == nil {
		t.Fatal("lootKey should be initialized")
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
