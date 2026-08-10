package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
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

func TestSSHKeyRotator_GenerateKey(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	testDB := newTestDB(t)
	rotator := NewSSHKeyRotator(testDB, dir, time.Hour)

	err := rotator.RotateKey()
	if err != nil {
		t.Fatalf("RotateKey: %v", err)
	}

	if rotator.KeyCount() != 1 {
		t.Errorf("expected KeyCount 1, got %d", rotator.KeyCount())
	}

	privFiles, _ := filepath.Glob(filepath.Join(dir, "*_ed25519"))
	if len(privFiles) < 1 {
		t.Fatalf("expected at least 1 private key file, got %d", len(privFiles))
	}
	var privFile string
	for _, f := range privFiles {
		if filepath.Ext(f) != ".pub" {
			privFile = f
			break
		}
	}
	if privFile == "" {
		t.Fatal("no non-pub private key file found")
	}
	privData, err := os.ReadFile(privFile)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	if len(privData) == 0 {
		t.Error("private key file is empty")
	}
	privStr := string(privData)
	if len(privStr) < 10 || (privStr[:15] != "-----BEGIN OPENSSH" && privStr[:11] != "-----BEGIN ") {
		t.Errorf("private key does not look like PEM/OpenSSH format, starts with: %q", privStr[:min(30, len(privStr))])
	}

	pubFiles, _ := filepath.Glob(filepath.Join(dir, "*_ed25519.pub"))
	if len(pubFiles) != 1 {
		t.Fatalf("expected 1 public key file, got %d", len(pubFiles))
	}
	pubData, err := os.ReadFile(pubFiles[0])
	if err != nil {
		t.Fatalf("read public key: %v", err)
	}
	if len(pubData) == 0 {
		t.Error("public key file is empty")
	}

	err = rotator.RotateKey()
	if err != nil {
		t.Fatalf("second RotateKey: %v", err)
	}
	if rotator.KeyCount() != 2 {
		t.Errorf("expected KeyCount 2, got %d", rotator.KeyCount())
	}

	privFiles2, _ := filepath.Glob(filepath.Join(dir, "*_ed25519"))
	if len(privFiles2) != 2 {
		t.Errorf("expected 2 private key files after second rotation, got %d", len(privFiles2))
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

func TestTransportObfuscation_DomainRotation(t *testing.T) {
	t.Helper()
	dto := NewDNSTransportObfuscator()
	dto.SetDomains([]string{"a.example.com", "b.example.com", "c.example.com"})

	initial := dto.GetCurrentDomain()
	if initial == "" {
		t.Fatal("GetCurrentDomain returned empty after SetDomains")
	}

	rotated := false
	for i := 0; i < 50; i++ {
		dto.rotateDomain()
		if dto.GetCurrentDomain() != initial {
			rotated = true
			break
		}
	}
	if !rotated {
		t.Error("domain never rotated after 50 attempts")
	}
}

func TestTransportObfuscation_FakeQueryRate(t *testing.T) {
	t.Helper()
	dto := NewDNSTransportObfuscator()
	dto.fakeQueryRate = 0.5

	const trials = 1000
	fakeCount := 0
	for i := 0; i < trials; i++ {
		if dto.ShouldSendFakeQuery() {
			fakeCount++
		}
	}

	rate := float64(fakeCount) / float64(trials)
	if rate < 0.2 || rate > 0.8 {
		t.Errorf("fake query rate %.2f outside expected range [0.2, 0.8] for configured rate 0.5", rate)
	}

	ico := NewICMPTransportObfuscator()
	ico.decoyRate = 0.3
	decoyCount := 0
	for i := 0; i < trials; i++ {
		if ico.ShouldSendDecoy() {
			decoyCount++
		}
	}
	decoyRate := float64(decoyCount) / float64(trials)
	if decoyRate < 0.05 || decoyRate > 0.6 {
		t.Errorf("decoy rate %.2f outside expected range for configured rate 0.3", decoyRate)
	}
}

func TestTLSCertMonitor_Stats(t *testing.T) {
	t.Helper()
	jv := NewTLSCertMonitor(true)
	if jv == nil {
		t.Fatal("NewTLSCertMonitor returned nil for enabled=true")
	}

	stats := jv.GetStats()
	if stats["unique_hashes"] != 0 {
		t.Errorf("initial unique_hashes = %d, want 0", stats["unique_hashes"])
	}
	if stats["total_probes"] != 0 {
		t.Errorf("initial total_probes = %d, want 0", stats["total_probes"])
	}

	jv.RecordCertHash("hash-aaa")
	jv.RecordCertHash("hash-bbb")
	jv.RecordCertHash("hash-aaa")

	stats = jv.GetStats()
	if stats["unique_hashes"] != 2 {
		t.Errorf("unique_hashes = %d, want 2", stats["unique_hashes"])
	}
	if stats["total_probes"] != 3 {
		t.Errorf("total_probes = %d, want 3", stats["total_probes"])
	}

	if NewTLSCertMonitor(false) != nil {
		t.Error("NewTLSCertMonitor(false) should return nil")
	}

	var nilMonitor *TLSCertMonitor
	nilMonitor.RecordCertHash("noop")
	if nilMonitor.GetStats() != nil {
		t.Error("nil TLSCertMonitor.GetStats() should return nil")
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

	actions := []string{"mimikatz", "dcsync", "kerberoast", "shinject"}
	for _, action := range actions {
		if !am.ShouldBlockAction("agent-z", action) {
			t.Errorf("expected %q to be blocked at ThreatCritical", action)
		}
	}

	if am.ShouldBlockAction("agent-z", "ls") {
		t.Error("non-high-risk action 'ls' should not be blocked")
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


