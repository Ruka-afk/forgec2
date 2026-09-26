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
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
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
		ErrTOTPRequired, ErrTOTPInvalid, ErrBackupCodeExhausted,
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
		{ErrTOTPRequired, "ErrTOTPRequired"},
		{ErrTOTPInvalid, "ErrTOTPInvalid"},
		{ErrBackupCodeExhausted, "ErrBackupCodeExhausted"},
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

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return database
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

// TestOpsecSleepMaskAlertEscalatesThreat proves the memory-scanner alert path
// feeds the adaptive manager: repeated sleep-mask integrity failures push the
// agent to ThreatCritical, after which credential ops are rejected.
func TestOpsecSleepMaskAlertEscalatesThreat(t *testing.T) {
	s := newTasksTestServer(t)
	s.opsecAdaptive = opsec.NewAdaptiveManager()

	s.autoSwitchSleepMask("agent-mask", "sleep_mask_integrity_failure: mask=advanced page=2")
	if level := s.opsecAdaptive.GetThreatLevel("agent-mask"); level != opsec.ThreatNormal {
		t.Fatalf("1 alert should stay ThreatNormal, got %d", level)
	}

	s.autoSwitchSleepMask("agent-mask", "sleep_mask_integrity_failure: mask=advanced page=2")
	if level := s.opsecAdaptive.GetThreatLevel("agent-mask"); level != opsec.ThreatElevated {
		t.Fatalf("2 alerts should yield ThreatElevated, got %d", level)
	}

	for i := 0; i < 8; i++ {
		s.autoSwitchSleepMask("agent-mask", "sleep_mask_integrity_failure: mask=advanced page=2")
	}
	if level := s.opsecAdaptive.GetThreatLevel("agent-mask"); level != opsec.ThreatCritical {
		t.Fatalf("10 alerts should yield ThreatCritical, got %d", level)
	}

	// Auto-switch (set_sleep_mask) is not credential access: it must still
	// dispatch on a hostile host, while a credential op is now blocked.
	if _, err := s.createTask("agent-mask", "mimikatz", "sekurlsa::logonpasswords", "", "", "", 0, 0); err == nil {
		t.Fatal("mimikatz must be blocked at ThreatCritical")
	}
	if _, err := s.createTask("agent-mask", "set_sleep_mask", "zilean", "", "", "", 0, 0); err != nil {
		t.Fatalf("set_sleep_mask must not be blocked: %v", err)
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

func base64Encode(data []byte) string {
	const table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, 0, (len(data)+2)/3*4)
	for i := 0; i < len(data); i += 3 {
		var b0, b1, b2 byte
		b0 = data[i]
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		result = append(result, table[b0>>2])
		result = append(result, table[((b0&0x03)<<4)|(b1>>4)])
		if i+1 < len(data) {
			result = append(result, table[((b1&0x0f)<<2)|(b2>>6)])
		} else {
			result = append(result, '=')
		}
		if i+2 < len(data) {
			result = append(result, table[b2&0x3f])
		} else {
			result = append(result, '=')
		}
	}
	return string(result)
}
