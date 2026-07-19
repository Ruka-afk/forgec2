package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Implant.DefaultInterval != 5 {
		t.Errorf("expected interval 5, got %d", cfg.Implant.DefaultInterval)
	}
	if cfg.Implant.DefaultJitter != 20 {
		t.Errorf("expected jitter 20, got %d", cfg.Implant.DefaultJitter)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected level info, got %s", cfg.Logging.Level)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Server.Port != 8080 {
		t.Error("default port should be 8080")
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Server.Port = 9090
	cfg.Server.JWTSecret = "test-secret-12345"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", loaded.Server.Port)
	}
}

func TestJWTAutoGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	if cfg.Server.JWTSecret != "" {
		t.Error("default config should not have JWT secret")
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Server.JWTSecret) < 32 {
		t.Errorf("JWT secret too short: %d chars", len(loaded.Server.JWTSecret))
	}
}

func TestSaveConcurrentSafety(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-concurrent"

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			cfg.Save(path)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file should exist after concurrent saves")
	}
}

func TestIsWeakSecret(t *testing.T) {
	tests := []struct {
		secret string
		weak   bool
	}{
		{"", true},
		{"short", true},
		{"change_me", true},
		{"forgec2_secret_key_change_this_in_production", true},
		{"secret", true},
		{"password", true},
		{"a]32chars-----------------------------------", false},
		{"a]32chars-----------------------------------2", false},
	}
	for _, tt := range tests {
		if got := isWeakSecret(tt.secret); got != tt.weak {
			t.Errorf("isWeakSecret(%q) = %v, want %v", tt.secret, got, tt.weak)
		}
	}
}

func TestAutoGenerateJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	// Write config with empty JWT secret
	cfg := DefaultConfig()
	cfg.Server.JWTSecret = ""
	out, _ := yaml.Marshal(cfg)
	os.WriteFile(path, out, 0644)

	// Load should auto-generate
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server.JWTSecret == "" {
		t.Fatal("expected auto-generated JWT secret, got empty")
	}
	if len(loaded.Server.JWTSecret) != 64 { // 32 bytes hex
		t.Fatalf("expected 64-char hex secret, got %d chars", len(loaded.Server.JWTSecret))
	}
}
