package crypto

import (
	"strings"
	"testing"
)

const testHexKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func TestInitLootEncryption(t *testing.T) {
	InitLootEncryption(testHexKey)
	lootKeyMu.RLock()
	initialized := lootKey != nil
	lootKeyMu.RUnlock()
	if !initialized {
		t.Fatal("lootKey should be initialized")
	}
}

func TestInitLootEncryptionInvalidKeyClearsLoot(t *testing.T) {
	// When no key is active, invalid key should leave it nil (clear).
	lootKeyMu.Lock()
	lootKey = nil
	lootKeyMu.Unlock()
	InitLootEncryption("not-valid-hex!")
	lootKeyMu.RLock()
	cleared := lootKey == nil
	lootKeyMu.RUnlock()
	if !cleared {
		t.Fatal("invalid key should leave loot key nil when none was active")
	}
	// When a key is already active, invalid reload must keep existing key
	// (see loot.go:31-37 — prevents wiping decryptability of stored FC2ENC rows).
	InitLootEncryption(testHexKey)
	InitLootEncryption("not-valid-hex!")
	lootKeyMu.RLock()
	kept := lootKey != nil
	lootKeyMu.RUnlock()
	if !kept {
		t.Fatal("invalid key should keep existing loot key when one is already active")
	}
	InitLootEncryption(testHexKey)
}

func TestInitLootEncryptionReentrant(t *testing.T) {
	InitLootEncryption(testHexKey)
	encA, err := EncryptLoot("secret-value")
	if err != nil {
		t.Fatalf("EncryptLoot failed: %v", err)
	}

	// Re-init with a different explicit key; a re-entrant init must take effect.
	InitLootEncryption(strings.Repeat("ff", 32))
	// Rotation retains the previous key: old rows stay readable (prev-key
	// fallback) instead of going dark.
	if dec, err := DecryptLoot(encA); err != nil || dec != "secret-value" {
		t.Fatalf("rotated-away ciphertext must stay readable via prev key, dec=%q err=%v", dec, err)
	}
	if _, state := DecryptLootState(encA); state != LootDecryptablePrevKey {
		t.Fatalf("old-key ciphertext state = %v, want LootDecryptablePrevKey", state)
	}
	if LootKeyID() == PrevLootKeyID() {
		t.Fatal("active and previous key fingerprints must differ after rotation")
	}
	InitLootEncryption(testHexKey)
	if dec, err := DecryptLoot(encA); err != nil || dec != "secret-value" {
		t.Fatalf("restoring the original key must restore decryptability, dec=%q err=%v", dec, err)
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

func TestDecryptLootStateTristate(t *testing.T) {
	InitLootEncryption(testHexKey)
	if _, state := DecryptLootState(""); state != LootEmpty {
		t.Fatalf("empty state = %v, want LootEmpty", state)
	}
	if plain, state := DecryptLootState("legacy-plaintext"); state != LootPlaintextLegacy || plain != "legacy-plaintext" {
		t.Fatalf("legacy state = %v %q", state, plain)
	}
	enc, err := EncryptLoot("v")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, state := DecryptLootState(enc); state != LootDecryptable {
		t.Fatalf("fresh ciphertext state = %v, want LootDecryptable", state)
	}
	if _, state := DecryptLootState("FC2ENC:!!!invalid-base64!!!"); state != LootUndecryptable {
		t.Fatalf("garbage state = %v, want LootUndecryptable", state)
	}
}

func TestReencryptLootNormalizesPrevKey(t *testing.T) {
	InitLootEncryption(testHexKey)
	enc, err := EncryptLoot("rotate-me")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	InitLootEncryption(strings.Repeat("ab", 32))
	normalized, changed, err := ReencryptLoot(enc)
	if err != nil {
		t.Fatalf("reencrypt: %v", err)
	}
	if !changed {
		t.Fatal("previous-key value must be re-encrypted")
	}
	if _, state := DecryptLootState(normalized); state != LootDecryptable {
		t.Fatalf("normalized state = %v, want LootDecryptable", state)
	}
	// Active-key values pass through untouched.
	again, changed, err := ReencryptLoot(normalized)
	if err != nil || changed || again != normalized {
		t.Fatalf("active-key value must pass through: changed=%v err=%v", changed, err)
	}
	// Garbage errors out instead of being laundered.
	if _, _, err := ReencryptLoot("FC2ENC:!!!invalid-base64!!!"); err == nil {
		t.Fatal("garbage must error, not normalize")
	}
	InitLootEncryption(testHexKey)
}

func TestInitLootEncryptionExplicitKey(t *testing.T) {
	// The configured key must be honored verbatim and remain stable: no
	// derivation from the JWT secret exists anymore.
	InitLootEncryption(testHexKey)
	encA, err := EncryptLoot("value")
	if err != nil {
		t.Fatalf("EncryptLoot failed: %v", err)
	}
	InitLootEncryption(testHexKey)
	if dec, err := DecryptLoot(encA); err != nil || dec != "value" {
		t.Fatalf("explicit key must remain stable across init calls, dec=%q err=%v", dec, err)
	}
}

func TestInitExtC2Encryption(t *testing.T) {
	InitExtC2Encryption(testHexKey)
	extc2KeyMu.RLock()
	initialized := extc2Key != nil
	extc2KeyMu.RUnlock()
	if !initialized {
		t.Fatal("extc2Key should be initialized")
	}
	InitExtC2Encryption(strings.Repeat("ab", 32))
	if _, err := EncryptExtC2("value"); err != nil {
		t.Fatalf("EncryptExtC2 failed: %v", err)
	}
	InitExtC2Encryption(testHexKey)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	InitLootEncryption(testHexKey)
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
	InitLootEncryption(testHexKey)
	enc, err := EncryptLoot("")
	if err != nil {
		t.Fatalf("EncryptLoot empty should not error: %v", err)
	}
	if enc != "" {
		t.Fatal("encrypted empty string should be empty")
	}
}

func TestDecryptLegacyPlaintext(t *testing.T) {
	InitLootEncryption(testHexKey)
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
	InitLootEncryption(testHexKey)
	_, err := DecryptLoot("FC2ENC:!!!invalid-base64!!!")
	if err == nil {
		t.Fatal("DecryptLoot invalid format should return error")
	}
}
