package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
)

const (
	vaultTestKeyA = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	vaultTestKeyB = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
)

// TestVaultReencryptNormalizesPrevKey seeds rows under key A, rotates to B,
// and verifies the job re-encrypts them to B while leaving garbage rows
// untouched and counted as failed.
func TestVaultReencryptNormalizesPrevKey(t *testing.T) {
	crypto.InitLootEncryption(vaultTestKeyA)
	defer crypto.InitLootEncryption(vaultTestKeyA)
	encA, err := crypto.EncryptLoot("rotate-me")
	if err != nil {
		t.Fatalf("encrypt under A: %v", err)
	}
	crypto.InitLootEncryption(vaultTestKeyB)

	s := &Server{db: newContractDB(t)}
	seed := []db.Task{
		{AgentID: "vault-a1", Type: "shell", Status: "completed", Result: encA},
		{AgentID: "vault-a1", Type: "shell", Status: "completed", Result: "FC2ENC:!!!invalid-base64!!!"},
		{AgentID: "vault-a1", Type: "shell", Status: "completed", Result: "legacy plaintext"},
	}
	for i := range seed {
		if err := s.db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	scanned, reencrypted, failed := s.runVaultReencryptOnce()
	if scanned != 2 {
		t.Fatalf("scanned = %d, want 2 (only FC2ENC: rows)", scanned)
	}
	if reencrypted != 1 {
		t.Fatalf("reencrypted = %d, want 1", reencrypted)
	}
	if failed != 1 {
		t.Fatalf("failed = %d, want 1 (garbage row)", failed)
	}

	// Raw row must now decrypt under the active key B.
	var raw struct{ Result string }
	if err := s.db.Table("tasks").Where("id = ?", seed[0].ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Result, "FC2ENC:") {
		t.Fatalf("row lost its ciphertext marker: %q", raw.Result)
	}
	if plain, err := crypto.DecryptLoot(raw.Result); err != nil || plain != "rotate-me" {
		t.Fatalf("re-encrypted row unreadable: %q %v", plain, err)
	}
	if _, state := crypto.DecryptLootState(raw.Result); state != crypto.LootDecryptable {
		t.Fatalf("re-encrypted state = %v, want LootDecryptable", state)
	}

	// Garbage row untouched.
	var rawBad struct{ Result string }
	if err := s.db.Table("tasks").Where("id = ?", seed[1].ID).Scan(&rawBad).Error; err != nil {
		t.Fatalf("raw scan bad: %v", err)
	}
	if rawBad.Result != "FC2ENC:!!!invalid-base64!!!" {
		t.Fatalf("garbage row mutated: %q", rawBad.Result)
	}
}

// TestVaultReencryptNoPrevKeyNoop verifies the job is a no-op without a
// previous key (fresh installs must not churn the vault).
func TestVaultReencryptNoPrevKeyNoop(t *testing.T) {
	crypto.InitLootEncryption(vaultTestKeyA)
	defer crypto.InitLootEncryption(vaultTestKeyA)
	s := &Server{db: newContractDB(t)}
	if scanned, reencrypted, failed := s.runVaultReencryptOnce(); scanned != 0 || reencrypted != 0 || failed != 0 {
		t.Fatalf("expected noop, got scanned=%d reencrypted=%d failed=%d", scanned, reencrypted, failed)
	}
}
