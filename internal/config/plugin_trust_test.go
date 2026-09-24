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

	// Bad values are ignored, so the config falls back to its safe defaults
	// instead of being poisoned by a typo.
	t.Setenv("FORGEC2_PLUGINS_REQUIRE_SIGNED", "maybe")
	t.Setenv("FORGEC2_PLUGINS_MAX_CONCURRENT", "-3")
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
}
