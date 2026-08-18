package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
)

// TestCreateTaskEncryptsSensitiveCommandAtRest verifies the at-rest hook
// chain through createTask: password_spray (a sprayed domain password) is
// ciphertext in the raw tasks table while plaintext in memory, and shell
// commands stay plaintext for operator search.
func TestCreateTaskEncryptsSensitiveCommandAtRest(t *testing.T) {
	crypto.InitLootEncryption(testStorageKeyHex)
	s := newTasksTestServer(t)

	task, err := s.createTask("agent-enc", "password_spray", "P@ssw0rd!|CORP||0", "", "", "jsmith", 0, 0)
	if err != nil {
		t.Fatalf("createTask: %v", err)
	}
	if !strings.HasPrefix(task.Command, "FC2ENC:") {
		t.Fatalf("spray command must be encrypted by createTask, got %q", task.Command)
	}

	var raw struct {
		Command string
		Data    string
	}
	if err := s.db.Table("tasks").Where("id = ?", task.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Command, "FC2ENC:") || !strings.HasPrefix(raw.Data, "FC2ENC:") {
		t.Fatalf("raw row must hold ciphertext: %q / %q", raw.Command, raw.Data)
	}

	// Loading through the model restores plaintext (as dispatch/UI paths see it).
	var loaded db.Task
	if err := s.db.First(&loaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loaded.Command != "P@ssw0rd!|CORP||0" || loaded.Data != "jsmith" {
		t.Fatalf("sensitive fields not decrypted on load: %q / %q", loaded.Command, loaded.Data)
	}

	// Non-sensitive types stay searchable at rest.
	shellTask, err := s.createTask("agent-enc", "shell", "whoami", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("createTask shell: %v", err)
	}
	if shellTask.Command != "whoami" {
		t.Fatalf("shell command must stay plaintext, got %q", shellTask.Command)
	}
}

// TestSensitiveTaskTypesDerivedFromAtRestSet locks in the single source of
// truth: the wire-encryption set is the at-rest set plus the explicitly
// searchable operator types, and the credential-bearing types newly added
// (token_make password, lateral spec, execute_assembly payload) are covered
// by BOTH wire and at-rest encryption.
func TestSensitiveTaskTypesDerivedFromAtRestSet(t *testing.T) {
	for _, k := range []string{
		"password_spray", "token_make", "lateral", "mimikatz", "creds",
		"dcsync", "execute_assembly", "bof", "peloader", "inject", "spawn",
		"shinject", "shspawn", "powerpick", "reg_set", "clipboard_set",
	} {
		if !db.SensitiveTaskTypes[k] {
			t.Errorf("at-rest set missing %q", k)
		}
		if !sensitiveTaskTypes[k] {
			t.Errorf("wire set must cover at-rest type %q", k)
		}
	}
	for _, k := range []string{"shell", "ps", "upload", "download_url",
		"kerberoast", "lsa_bypass", "cookie_export", "vpn_creds", "wifi_creds"} {
		if !sensitiveTaskTypes[k] {
			t.Errorf("wire set missing %q", k)
		}
		if db.SensitiveTaskTypes[k] {
			t.Errorf("searchable/wire-only type %q must not be at-rest encrypted", k)
		}
	}
	if sensitiveTaskTypes["ls"] {
		t.Error("'ls' must never be treated as sensitive")
	}
}