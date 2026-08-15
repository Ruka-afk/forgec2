package db

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
)

// TestTaskResultEncryptionAtRest verifies that task Result/Error are encrypted
// for storage (FC2ENC: marker) and transparently decrypted on load via the
// GORM AfterFind hook, so command output containing credentials is never
// persisted as plaintext (H3).
func TestTaskResultEncryptionAtRest(t *testing.T) {
	// 32-byte hex key derived from a known value.
	const key = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	crypto.InitLootEncryption(key)
	defer crypto.InitLootEncryption("") // clear so other tests start clean

	task := Task{Result: "secret-output-with-creds", Error: "explode"}
	task.EncryptTaskFields()

	if !strings.HasPrefix(task.Result, "FC2ENC:") {
		t.Fatalf("Result should be encrypted at rest, got %q", task.Result)
	}
	if !strings.HasPrefix(task.Error, "FC2ENC:") {
		t.Fatalf("Error should be encrypted at rest, got %q", task.Error)
	}

	// AfterFind (the GORM load hook) must restore plaintext.
	if err := task.AfterFind(nil); err != nil {
		t.Fatalf("AfterFind returned error: %v", err)
	}
	if task.Result != "secret-output-with-creds" {
		t.Fatalf("Result not decrypted: got %q", task.Result)
	}
	if task.Error != "explode" {
		t.Fatalf("Error not decrypted: got %q", task.Error)
	}
}

// TestTaskResultAfterFindLegacyPlaintext ensures legacy plaintext rows survive
// the AfterFind hook untouched (backward compatibility for pre-encryption data).
func TestTaskResultAfterFindLegacyPlaintext(t *testing.T) {
	task := Task{Result: "already-plaintext", Error: ""}
	if err := task.AfterFind(nil); err != nil {
		t.Fatalf("AfterFind returned error: %v", err)
	}
	if task.Result != "already-plaintext" {
		t.Fatalf("legacy Result mutated: got %q", task.Result)
	}
}
