package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/opsec"
)

func TestErrorCodes_AllCodesHaveHTTPMapping(t *testing.T) {
	t.Helper()
	allCodes := []ErrorCode{
		ErrBadRequest, ErrUnauthorized, ErrForbidden, ErrNotFound,
		ErrConflict, ErrRateLimited, ErrInternal, ErrServiceUnavailable,
		ErrDatabase, ErrConfigInvalid, ErrCryptoFailed,
		ErrAgentNotFound, ErrTaskNotFound, ErrPluginNotFound,
		ErrSessionExpired, ErrSessionRevoked, ErrAccountLocked,
		ErrCSRFMismatch, ErrPermissionDenied,
		ErrPasswordComplexity, ErrPasswordHistory, ErrPayloadTooLarge,
		ErrInvalidExtension, ErrInvalidCallbackURL,
		ErrAgentOffline, ErrAgentBusy,
		ErrBuildJobFailed, ErrBuildJobNotFound,
		ErrListenerFailed, ErrListenerNotFound,
		ErrEncryptedFieldFailed, ErrDatabaseMigration,
		ErrConfigReloadFailed, ErrEmergencyStopFailed,
		ErrProfileNotFound, ErrIntegrityViolation,
	}
	for _, code := range allCodes {
		status := code.HTTPStatus()
		if status < 100 || status > 599 {
			t.Errorf("ErrorCode %d mapped to invalid HTTP status %d", int(code), status)
		}
		if status == http.StatusInternalServerError && code != ErrInternal &&
			code != ErrDatabase && code != ErrConfigInvalid && code != ErrCryptoFailed &&
			code != ErrBuildJobFailed && code != ErrListenerFailed &&
			code != ErrEncryptedFieldFailed && code != ErrDatabaseMigration &&
			code != ErrConfigReloadFailed && code != ErrEmergencyStopFailed &&
			code != ErrIntegrityViolation {
			t.Errorf("ErrorCode %d has default 500 status but should have explicit mapping", int(code))
		}
	}
}

func TestErrorCodes_DuplicateCodes(t *testing.T) {
	t.Helper()
	seen := make(map[ErrorCode]string)
	codes := []struct {
		code ErrorCode
		name string
	}{
		{ErrBadRequest, "ErrBadRequest"},
		{ErrUnauthorized, "ErrUnauthorized"},
		{ErrForbidden, "ErrForbidden"},
		{ErrNotFound, "ErrNotFound"},
		{ErrConflict, "ErrConflict"},
		{ErrRateLimited, "ErrRateLimited"},
		{ErrInternal, "ErrInternal"},
		{ErrServiceUnavailable, "ErrServiceUnavailable"},
		{ErrDatabase, "ErrDatabase"},
		{ErrConfigInvalid, "ErrConfigInvalid"},
		{ErrCryptoFailed, "ErrCryptoFailed"},
		{ErrAgentNotFound, "ErrAgentNotFound"},
		{ErrTaskNotFound, "ErrTaskNotFound"},
		{ErrPluginNotFound, "ErrPluginNotFound"},
		{ErrSessionExpired, "ErrSessionExpired"},
		{ErrSessionRevoked, "ErrSessionRevoked"},
		{ErrAccountLocked, "ErrAccountLocked"},
		{ErrCSRFMismatch, "ErrCSRFMismatch"},
		{ErrPermissionDenied, "ErrPermissionDenied"},
		{ErrPasswordComplexity, "ErrPasswordComplexity"},
		{ErrPasswordHistory, "ErrPasswordHistory"},
		{ErrPayloadTooLarge, "ErrPayloadTooLarge"},
		{ErrInvalidExtension, "ErrInvalidExtension"},
		{ErrInvalidCallbackURL, "ErrInvalidCallbackURL"},
		{ErrAgentOffline, "ErrAgentOffline"},
		{ErrAgentBusy, "ErrAgentBusy"},
		{ErrBuildJobFailed, "ErrBuildJobFailed"},
		{ErrBuildJobNotFound, "ErrBuildJobNotFound"},
		{ErrListenerFailed, "ErrListenerFailed"},
		{ErrListenerNotFound, "ErrListenerNotFound"},
		{ErrEncryptedFieldFailed, "ErrEncryptedFieldFailed"},
		{ErrDatabaseMigration, "ErrDatabaseMigration"},
		{ErrConfigReloadFailed, "ErrConfigReloadFailed"},
		{ErrEmergencyStopFailed, "ErrEmergencyStopFailed"},
		{ErrProfileNotFound, "ErrProfileNotFound"},
		{ErrIntegrityViolation, "ErrIntegrityViolation"},
	}
	for _, c := range codes {
		if prev, exists := seen[c.code]; exists {
			t.Errorf("ErrorCode %d (%s) duplicates %s", int(c.code), c.name, prev)
		}
		seen[c.code] = c.name
	}
}

func TestBackupManager_RotateKey(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	origKeyBytes := make([]byte, 32)
	rand.Read(origKeyBytes)
	origKeyHex := hex.EncodeToString(origKeyBytes)

	bm, err := NewBackupManager(nil, "dummy.db", dir, origKeyHex)
	if err != nil {
		t.Fatalf("NewBackupManager: %v", err)
	}

	plaintext := []byte("sensitive database content for testing")

	encrypted, err := bm.encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if string(encrypted) == string(plaintext) {
		t.Error("encrypted data should differ from plaintext")
	}

	decrypted, err := bm.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypt mismatch: got %q, want %q", decrypted, plaintext)
	}

	fbkPath := filepath.Join(dir, "test_backup.fbk")
	if err := os.WriteFile(fbkPath, encrypted, 0600); err != nil {
		t.Fatalf("write test fbk: %v", err)
	}

	newKeyBytes := make([]byte, 32)
	rand.Read(newKeyBytes)
	newKeyHex := hex.EncodeToString(newKeyBytes)

	err = bm.RotateKey(newKeyHex)
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}

	reData, err := os.ReadFile(fbkPath)
	if err != nil {
		t.Fatalf("read re-encrypted backup: %v", err)
	}

	reDecrypted, err := bm.decrypt(reData)
	if err != nil {
		t.Fatalf("decrypt re-encrypted with new key: %v", err)
	}
	if string(reDecrypted) != string(plaintext) {
		t.Errorf("re-decrypted content mismatch: got %q, want %q", reDecrypted, plaintext)
	}

	if string(bm.key) != string(newKeyBytes) {
		t.Error("manager key should be updated after rotation")
	}
}

func TestOpsecAdaptive_ThreatLevel(t *testing.T) {
	t.Helper()
	am := opsec.NewAdaptiveManager()

	if am.GetThreatLevel("agent-unknown") != opsec.ThreatNormal {
		t.Error("unknown agent should have ThreatNormal")
	}

	level := am.RecordIntegrityFailure("agent-x")
	if level != opsec.ThreatNormal {
		t.Errorf("1 failure: threat level = %d, want ThreatNormal(0)", level)
	}

	am.RecordIntegrityFailure("agent-x")
	level = am.GetThreatLevel("agent-x")
	if level != opsec.ThreatElevated {
		t.Errorf("2 failures: threat level = %d, want ThreatElevated(1)", level)
	}

	for i := 0; i < 3; i++ {
		am.RecordIntegrityFailure("agent-x")
	}
	level = am.GetThreatLevel("agent-x")
	if level != opsec.ThreatHigh {
		t.Errorf("5 failures: threat level = %d, want ThreatHigh(2)", level)
	}

	for i := 0; i < 5; i++ {
		am.RecordIntegrityFailure("agent-x")
	}
	level = am.GetThreatLevel("agent-x")
	if level != opsec.ThreatCritical {
		t.Errorf("10 failures: threat level = %d, want ThreatCritical(3)", level)
	}

	sleep, jitter, rotate := am.GetRecommendedSleepParams("agent-x")
	if sleep != 10 {
		t.Errorf("critical sleep = %d, want 10", sleep)
	}
	if jitter != 50 {
		t.Errorf("critical jitter = %d, want 50", jitter)
	}
	if !rotate {
		t.Error("critical should recommend key rotation")
	}
}

func TestOpsecAdaptive_Decay(t *testing.T) {
	t.Helper()
	am := opsec.NewAdaptiveManager()

	for i := 0; i < 5; i++ {
		am.RecordIntegrityFailure("agent-y")
	}
	if am.GetThreatLevel("agent-y") != opsec.ThreatHigh {
		t.Fatalf("expected ThreatHigh after 5 failures")
	}

	state := am.GetState("agent-y")
	if state == nil {
		t.Fatal("GetState returned nil for agent-y")
	}
	state.LastFailureAt = time.Now().Add(-15 * time.Minute)

	am.DecayThreatLevel("agent-y")
	if am.GetThreatLevel("agent-y") == opsec.ThreatHigh {
		t.Error("threat level should have decayed, but was still ThreatHigh")
	}

	for i := 0; i < 5; i++ {
		s := am.GetState("agent-y")
		s.LastFailureAt = time.Now().Add(-15 * time.Minute)
		am.DecayThreatLevel("agent-y")
	}
	if am.GetThreatLevel("agent-y") != opsec.ThreatNormal {
		t.Errorf("fully decayed level = %d, want ThreatNormal(0)", am.GetThreatLevel("agent-y"))
	}

	am.DecayThreatLevel("agent-y")
	if am.GetThreatLevel("agent-y") != opsec.ThreatNormal {
		t.Error("decay below ThreatNormal should not happen")
	}

	am.DecayThreatLevel("nonexistent-agent")
}

func TestOpsecAdaptive_BlockCritical(t *testing.T) {
	t.Helper()
	am := opsec.NewAdaptiveManager()

	if am.ShouldBlockAction("agent-z", "mimikatz") {
		t.Error("should not block actions for unknown agent")
	}

	for i := 0; i < 10; i++ {
		am.RecordIntegrityFailure("agent-z")
	}
	if am.GetThreatLevel("agent-z") != opsec.ThreatCritical {
		t.Fatalf("expected ThreatCritical")
	}

	actions := []string{"mimikatz", "dcsync", "kerberoast", "shinject", "creds", "password_spray"}
	for _, action := range actions {
		if !am.ShouldBlockAction("agent-z", action) {
			t.Errorf("expected %q to be blocked at ThreatCritical", action)
		}
	}

	if am.ShouldBlockAction("agent-z", "ls") {
		t.Error("non-high-risk action 'ls' should not be blocked")
	}
}

// TestOpsecCreateTaskBlockedAtCritical proves the adaptive gate is enforced at
// task creation: once an agent's threat level reaches ThreatCritical,
// credential-access ops (spray/creds/mimikatz/kerberoast) are rejected before
// any task is persisted, benign ops still work, and the block is audited.
func TestOpsecCreateTaskBlockedAtCritical(t *testing.T) {
	s := newTasksTestServer(t)
	s.opsecAdaptive = opsec.NewAdaptiveManager()

	// Normal threat: the same credential op is allowed.
	if _, err := s.createTask("agent-opsec", "password_spray", "P@ss|CORP||0", "", "", "jsmith", 0, 0); err != nil {
		t.Fatalf("spray must be allowed at normal threat: %v", err)
	}

	for i := 0; i < 10; i++ {
		s.opsecAdaptive.RecordIntegrityFailure("agent-opsec")
	}
	if s.opsecAdaptive.GetThreatLevel("agent-opsec") != opsec.ThreatCritical {
		t.Fatalf("expected ThreatCritical")
	}

	blocked := []struct {
		taskType, command, data string
	}{
		{"password_spray", "P@ss|CORP||0", "jsmith"},
		{"creds", "", ""},
		{"mimikatz", "sekurlsa::logonpasswords", ""},
		{"kerberoast", "", ""},
	}
	for _, tc := range blocked {
		if _, err := s.createTask("agent-opsec", tc.taskType, tc.command, "", "", tc.data, 0, 0); err == nil {
			t.Fatalf("expected %s to be blocked at ThreatCritical", tc.taskType)
		} else if !strings.Contains(err.Error(), "adaptive opsec") {
			t.Fatalf("%s error must cite adaptive opsec, got: %v", tc.taskType, err)
		}
	}

	// No blocked task may be persisted, and benign ops still dispatch.
	var count int64
	s.db.Model(&db.Task{}).Where("agent_id = ?", "agent-opsec").Count(&count)
	if count != 1 {
		t.Fatalf("expected only the pre-escalation task in DB, got %d", count)
	}
	if _, err := s.createTask("agent-opsec", "shell", "whoami", "", "", "", 0, 0); err != nil {
		t.Fatalf("shell must not be blocked at critical: %v", err)
	}

	// The blocks are audited.
	var blockLogs int64
	s.db.Model(&db.AuditLog{}).Where("action = ? AND agent_id = ?", "opsec_block", "agent-opsec").Count(&blockLogs)
	if blockLogs != 4 {
		t.Fatalf("expected 4 opsec_block audit entries, got %d", blockLogs)
	}
}

func TestExtC2Encryption_RoundTrip(t *testing.T) {
	t.Helper()
	crypto.InitExtC2Encryption(testStorageKeyHex)

	original := "discord-webhook-token-super-secret-12345"
	encrypted, err := crypto.EncryptExtC2(original)
	if err != nil {
		t.Fatalf("EncryptExtC2: %v", err)
	}
	if encrypted == original {
		t.Error("encrypted should differ from original")
	}
	if len(encrypted) < 7 || encrypted[:7] != "FC2EXT:" {
		t.Errorf("encrypted prefix = %q, want FC2EXT:", encrypted[:min(7, len(encrypted))])
	}

	decrypted, err := crypto.DecryptExtC2(encrypted)
	if err != nil {
		t.Fatalf("DecryptExtC2: %v", err)
	}
	if decrypted != original {
		t.Errorf("decrypted = %q, want %q", decrypted, original)
	}

	empty, err := crypto.EncryptExtC2("")
	if err != nil {
		t.Fatalf("EncryptExtC2 empty: %v", err)
	}
	if empty != "" {
		t.Errorf("EncryptExtC2('') = %q, want empty", empty)
	}

	decEmpty, err := crypto.DecryptExtC2("")
	if err != nil {
		t.Fatalf("DecryptExtC2 empty: %v", err)
	}
	if decEmpty != "" {
		t.Errorf("DecryptExtC2('') = %q, want empty", decEmpty)
	}

	legacy, err := crypto.DecryptExtC2("old-unencrypted-value")
	if err != nil {
		t.Fatalf("DecryptExtC2 legacy: %v", err)
	}
	if legacy != "old-unencrypted-value" {
		t.Errorf("legacy fallback = %q, want 'old-unencrypted-value'", legacy)
	}
}
