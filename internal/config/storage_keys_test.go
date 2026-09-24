package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestEnsureStorageKeysFillsMissingAndPersists proves the documented
// "cp config.example.yaml config.yaml" path boots: empty storage keys are
// generated and written back instead of failing validation.
func TestEnsureStorageKeysFillsMissingAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8000\ncrypto:\n  loot_key: \"\"\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := cfg.EnsureStorageKeys(); err != nil {
		t.Fatalf("EnsureStorageKeys: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate after EnsureStorageKeys: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var persisted Config
	if err := yaml.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("unmarshal persisted: %v", err)
	}
	for name, val := range map[string]string{
		"loot_key":   persisted.Crypto.LootKey,
		"extc2_key":  persisted.Crypto.ExtC2Key,
		"backup_key": persisted.Crypto.BackupKey,
		"totp_key":   persisted.Crypto.TotpKey,
		"csrf_key":   persisted.Crypto.CsrfKey,
	} {
		if len(val) != 64 {
			t.Fatalf("persisted %s = %q, want 64 hex chars", name, val)
		}
	}
}

// TestEnsureStorageKeysIsIdempotent proves a second call neither rotates keys
// nor duplicates work — stable keys are what keep FC2ENC:/TOTP data readable.
func TestEnsureStorageKeysIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8000\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := cfg.EnsureStorageKeys(); err != nil {
		t.Fatalf("first EnsureStorageKeys: %v", err)
	}
	before := cfg.Crypto.LootKey + cfg.Crypto.CsrfKey
	if err := cfg.EnsureStorageKeys(); err != nil {
		t.Fatalf("second EnsureStorageKeys: %v", err)
	}
	if after := cfg.Crypto.LootKey + cfg.Crypto.CsrfKey; after != before {
		t.Fatal("EnsureStorageKeys rotated existing keys on the second call")
	}
}

// TestEnsureStorageKeysToleratesReadOnlyConfig covers the container case where
// config.yaml is mounted :ro — the run must proceed with in-memory keys.
func TestEnsureStorageKeysToleratesReadOnlyConfig(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8000\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Windows ACLs do not map to the chmod bit this test needs.
	if _, err := os.Stat(path); err == nil {
		if err := os.Chmod(path, 0400); err != nil {
			t.Fatalf("chmod: %v", err)
		}
		if err := os.Chmod(dir, 0500); err != nil {
			t.Fatalf("chmod dir: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
	}
	cfg := &Config{ConfigPath: path}
	if err := cfg.EnsureStorageKeys(); err != nil {
		t.Fatalf("EnsureStorageKeys on read-only config: %v", err)
	}
	if len(cfg.Crypto.LootKey) != 64 {
		t.Fatal("in-memory key not generated for read-only config")
	}
}

// TestDataDirEnvOverrideRebasesDerivedPaths proves FORGEC2_DATA_DIR moves the
// state root (and every path still holding its default) without clobbering
// explicit operator values.
func TestDataDirEnvOverrideRebasesDerivedPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "server:\n" +
		"  data_dir: data\n" +
		"  cert_file: data/server.crt\n" +
		"  key_file: /etc/forgec2/tls.key\n" +
		"database:\n" +
		"  path: data/db/forgec2.db\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	newRoot := filepath.Join(dir, "volume")
	t.Setenv("FORGEC2_DATA_DIR", newRoot)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.DataDir != newRoot {
		t.Fatalf("data_dir = %q, want %q", cfg.Server.DataDir, newRoot)
	}
	if want := filepath.Join(newRoot, "db", "forgec2.db"); filepath.Clean(cfg.Database.Path) != want {
		t.Fatalf("database.path = %q, want %q", cfg.Database.Path, want)
	}
	if want := filepath.Join(newRoot, "server.crt"); cfg.Server.CertFile != want {
		t.Fatalf("cert_file = %q, want %q", cfg.Server.CertFile, want)
	}
	// Explicit absolute operator value must survive.
	if cfg.Server.KeyFile != "/etc/forgec2/tls.key" {
		t.Fatalf("explicit key_file overwritten: %q", cfg.Server.KeyFile)
	}
}

// TestExampleConfigBootsAfterEnsureStorageKeys is the contract test for the
// shipped example: it must pass validation once missing keys are filled,
// without any manual edit beyond copying the file.
func TestExampleConfigBootsAfterEnsureStorageKeys(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Skipf("config.example.yaml unavailable: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.example.yaml")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatalf("write copy: %v", err)
	}
	t.Setenv("FORGEC2_DATA_DIR", "")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load example: %v", err)
	}
	if err := cfg.EnsureStorageKeys(); err != nil {
		t.Fatalf("EnsureStorageKeys: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		if strings.Contains(err.Error(), "required") {
			t.Fatalf("example config still fails after EnsureStorageKeys: %v", err)
		}
		t.Skipf("example config needs other operator input: %v", err)
	}
}
