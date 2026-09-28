package server

import (
	"strings"
	"testing"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

const extc2ResultTestKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// lootUninitialized reports whether the process-wide loot key is currently
// unset, in which case EncryptLoot fails. The key is a package global shared
// by every test in this binary, so callers must probe first and skip rather
// than assume a state they cannot control.
func lootUninitialized() bool {
	_, err := forgecrypto.EncryptLoot("probe")
	return err != nil
}

func seedExtC2PendingTask(t *testing.T, s *Server, agentID string) uint {
	t.Helper()
	task := db.Task{AgentID: agentID, Type: "shell", Command: "whoami", Status: "pending"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task.ID
}

func rawTaskRow(t *testing.T, s *Server, id uint) (result, errStr, status string) {
	t.Helper()
	var raw struct {
		Result string
		Error  string
		Status string
	}
	if err := s.db.Table("tasks").Where("id = ?", id).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	return raw.Result, raw.Error, raw.Status
}

// The success path: with a working vault, an ExtC2 result lands encrypted at
// rest and the task completes with no error marker.
func TestProcessExternalC2ResultEncryptsAtRest(t *testing.T) {
	forgecrypto.InitLootEncryption(extc2ResultTestKey)
	s := &Server{db: testutil.SetupTestDB(t)}
	id := seedExtC2PendingTask(t, s, "extc2-ok")

	s.processExternalC2Result("extc2-ok", id, "r1", "sekret-output")

	result, errStr, status := rawTaskRow(t, s, id)
	if status != "completed" {
		t.Fatalf("status = %q, want completed", status)
	}
	if !strings.HasPrefix(result, "FC2ENC:") {
		t.Fatalf("result stored as plaintext: %q", result)
	}
	if errStr != "" {
		t.Fatalf("error marker set on a healthy write: %q", errStr)
	}
	if v, err := forgecrypto.DecryptLoot(result); err != nil || v != "sekret-output" {
		t.Fatalf("stored ciphertext does not round-trip: %q, %v", v, err)
	}
}

// The failure path: with no vault key, the output must be dropped and the
// task flagged — never stored as plaintext with a clean "completed".
// Skips when another test in this binary already initialized the global key,
// because that state cannot be undone; the skip keeps the test from ever
// passing vacuously.
func TestProcessExternalC2ResultDropsOutputOnVaultFailure(t *testing.T) {
	if !lootUninitialized() {
		t.Skip("loot key already initialized by another test in this binary")
	}
	s := &Server{db: testutil.SetupTestDB(t)}
	id := seedExtC2PendingTask(t, s, "extc2-novault")

	s.processExternalC2Result("extc2-novault", id, "r1", "sekret-output")

	result, errStr, status := rawTaskRow(t, s, id)
	if status != "completed" {
		t.Fatalf("status = %q, want completed", status)
	}
	if result != "" {
		t.Fatalf("output stored despite vault failure: %q", result)
	}
	if strings.Contains(result, "sekret-output") {
		t.Fatal("plaintext command output leaked into the DB")
	}
	if errStr != "vault encryption unavailable" {
		t.Fatalf("error marker = %q, want the vault-unavailable marker", errStr)
	}
}

// encryptCredNotes must fail closed as well: harvested browser credentials
// must never be written as plaintext when the vault is down.
func TestEncryptCredNotesFailsClosed(t *testing.T) {
	if !lootUninitialized() {
		t.Skip("loot key already initialized by another test in this binary")
	}
	if got, err := encryptCredNotes("browser|https://x|user|pass"); err == nil || got != "" {
		t.Fatalf("encryptCredNotes = %q, %v; want empty string and an error", got, err)
	}

	forgecrypto.InitLootEncryption(extc2ResultTestKey)
	if got, err := encryptCredNotes("browser|https://x|user|pass"); err != nil || got == "" || !strings.HasPrefix(got, "FC2ENC:") {
		t.Fatalf("encryptCredNotes with vault = %q, %v; want ciphertext", got, err)
	}
}
