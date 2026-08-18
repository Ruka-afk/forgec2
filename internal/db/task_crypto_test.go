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

// TestTaskSensitiveCommandEncryptionAtRest verifies that Command/Data of
// sensitive task types (spray passwords, token_make passwords, lateral specs,
// payload blobs) are encrypted by BeforeCreate/BeforeUpdate and transparently
// decrypted by AfterFind — and that the hooks are idempotent so an in-memory
// decrypted task re-saved via Save() does not leak plaintext back to disk.
func TestTaskSensitiveCommandEncryptionAtRest(t *testing.T) {
	db := setupTestDB(t)

	// Create a sensitive task: hooks must encrypt on first write.
	task := Task{AgentID: "agent-enc", Type: "password_spray", Command: "P@ssw0rd!|CORP||0", Data: "jsmith\nsvc_sql"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create sensitive task: %v", err)
	}
	if !strings.HasPrefix(task.Command, "FC2ENC:") {
		t.Fatalf("sensitive Command must be encrypted at rest, got %q", task.Command)
	}
	if !strings.HasPrefix(task.Data, "FC2ENC:") {
		t.Fatalf("sensitive Data must be encrypted at rest, got %q", task.Data)
	}
	// Raw row must hold ciphertext only.
	var raw struct {
		Command string
		Data    string
	}
	if err := db.Table("tasks").Where("id = ?", task.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if raw.Command != task.Command || raw.Data != task.Data {
		t.Fatalf("DB row does not match encrypted values: %q / %q", raw.Command, raw.Data)
	}

	// Loading via the model restores plaintext (AfterFind).
	var loaded Task
	if err := db.First(&loaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loaded.Command != "P@ssw0rd!|CORP||0" || loaded.Data != "jsmith\nsvc_sql" {
		t.Fatalf("sensitive fields not decrypted on load: %q / %q", loaded.Command, loaded.Data)
	}

	// Re-saving the decrypted in-memory task must not write plaintext back.
	if err := db.Save(&loaded).Error; err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if !strings.HasPrefix(loaded.Command, "FC2ENC:") {
		t.Fatalf("re-saved Command leaked plaintext: %q", loaded.Command)
	}
	var raw2 struct {
		Command string
	}
	if err := db.Table("tasks").Where("id = ?", task.ID).Scan(&raw2).Error; err != nil {
		t.Fatalf("raw scan 2: %v", err)
	}
	if raw2.Command != loaded.Command {
		t.Fatalf("re-saved row mismatch: %q", raw2.Command)
	}
	// Still decryptable after the re-save round trip.
	if err := loaded.AfterFind(nil); err != nil {
		t.Fatalf("AfterFind: %v", err)
	}
	if loaded.Command != "P@ssw0rd!|CORP||0" {
		t.Fatalf("AfterFind after re-save failed: %q", loaded.Command)
	}
}

// TestTaskNonSensitiveCommandStaysPlaintext ensures ordinary task types (shell
// is searchable by operators) are never encrypted at rest.
func TestTaskNonSensitiveCommandStaysPlaintext(t *testing.T) {
	db := setupTestDB(t)

	task := Task{AgentID: "agent-enc", Type: "shell", Command: "whoami /all"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create shell task: %v", err)
	}
	if task.Command != "whoami /all" {
		t.Fatalf("shell Command must stay plaintext at rest, got %q", task.Command)
	}
}
