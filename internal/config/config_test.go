package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Agent.DefaultInterval != 10 {
		t.Errorf("expected interval 10, got %d", cfg.Agent.DefaultInterval)
	}
	if cfg.Agent.DefaultJitter != 20 {
		t.Errorf("expected jitter 20, got %d", cfg.Agent.DefaultJitter)
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
	if cfg.Server.JWTSecret == "" {
		t.Error("JWTSecret should be auto-generated")
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
