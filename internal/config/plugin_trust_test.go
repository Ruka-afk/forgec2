package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPluginTrustEnvOverrides proves the plugin trust policy can be set from
// the environment (containers, CI) without editing config.yaml.
func TestPluginTrustEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8000\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	pub := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	t.Setenv("FORGEC2_PLUGIN_TRUSTED_KEYS", " "+pub+" , ")
	t.Setenv("FORGEC2_PLUGINS_REQUIRE_SIGNED", "true")
	t.Setenv("FORGEC2_PLUGINS_MAX_CONCURRENT", "7")
	t.Setenv("FORGEC2_PLUGINS_MAX_CPU_SECONDS", "45")
	t.Setenv("FORGEC2_PLUGINS_MAX_WRITE_MB", "512")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Plugins.TrustedKeys) != 1 || cfg.Plugins.TrustedKeys[0] != pub {
		t.Fatalf("trusted_keys = %v, want [%s]", cfg.Plugins.TrustedKeys, pub)
	}
	if !cfg.Plugins.RequireSigned {
		t.Fatal("require_signed override ignored")
	}
	if cfg.Plugins.MaxConcurrent != 7 {
		t.Fatalf("max_concurrent = %d, want 7", cfg.Plugins.MaxConcurrent)
	}
	if cfg.Plugins.MaxCPUSeconds != 45 {
		t.Fatalf("max_cpu_seconds = %d, want 45", cfg.Plugins.MaxCPUSeconds)
	}
	if cfg.Plugins.MaxWriteMB != 512 {
		t.Fatalf("max_write_mb = %d, want 512", cfg.Plugins.MaxWriteMB)
	}

	// Bad values are ignored, so the config falls back to its safe defaults
	// instead of being poisoned by a typo.
	t.Setenv("FORGEC2_PLUGINS_REQUIRE_SIGNED", "maybe")
	t.Setenv("FORGEC2_PLUGINS_MAX_CONCURRENT", "-3")
	t.Setenv("FORGEC2_PLUGINS_MAX_CPU_SECONDS", "-3")
	t.Setenv("FORGEC2_PLUGINS_MAX_WRITE_MB", "-3")
	cfg2, err := Load(path)
	if err != nil {
		t.Fatalf("load 2: %v", err)
	}
	if cfg2.Plugins.RequireSigned {
		t.Fatal("unparsable require_signed must be ignored")
	}
	if cfg2.Plugins.MaxConcurrent != 0 {
		t.Fatalf("negative max_concurrent must be ignored, got %d", cfg2.Plugins.MaxConcurrent)
	}
	if cfg2.Plugins.MaxCPUSeconds != 0 {
		t.Fatalf("negative max_cpu_seconds must be ignored, got %d", cfg2.Plugins.MaxCPUSeconds)
	}
	if cfg2.Plugins.MaxWriteMB != 0 {
		t.Fatalf("negative max_write_mb must be ignored, got %d", cfg2.Plugins.MaxWriteMB)
	}
}

// TestPluginQuotasFromConfigYAML proves the CPU and write budgets can be set
// from the config file, not only from the environment.
func TestPluginQuotasFromConfigYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("plugins:\n  max_cpu_seconds: 30\n  max_write_mb: 256\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Plugins.MaxCPUSeconds != 30 {
		t.Fatalf("max_cpu_seconds = %d, want 30", cfg.Plugins.MaxCPUSeconds)
	}
	if cfg.Plugins.MaxWriteMB != 256 {
		t.Fatalf("max_write_mb = %d, want 256", cfg.Plugins.MaxWriteMB)
	}
}

// TestPluginTrustConfigDefaults documents the shipped defaults: permissive, so
// the bundled unsigned plugins keep loading out of the box.
func TestPluginTrustConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Plugins.RequireSigned {
		t.Fatal("plugins.require_signed must default to false")
	}
	if len(cfg.Plugins.TrustedKeys) != 0 {
		t.Fatalf("plugins.trusted_keys must default to empty, got %v", cfg.Plugins.TrustedKeys)
	}
	if cfg.Plugins.MaxConcurrent != 0 {
		t.Fatalf("plugins.max_concurrent must default to 0 (built-in default), got %d", cfg.Plugins.MaxConcurrent)
	}
	if cfg.Plugins.MaxCPUSeconds != 0 {
		t.Fatalf("plugins.max_cpu_seconds must default to 0 (built-in default), got %d", cfg.Plugins.MaxCPUSeconds)
	}
	if cfg.Plugins.MaxWriteMB != 0 {
		t.Fatalf("plugins.max_write_mb must default to 0 (built-in default), got %d", cfg.Plugins.MaxWriteMB)
	}
}
