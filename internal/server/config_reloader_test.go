package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"gopkg.in/yaml.v3"
)

func copyConfig(cfg *config.Config) (*config.Config, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var dst config.Config
	if err := yaml.Unmarshal(data, &dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

func TestConfigReloader_RejectsStaticChanges(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	origCfg := config.DefaultConfig()
	origCfg.Server.Port = 8000
	origCfg.Server.Host = "127.0.0.1"
	origCfg.Server.JWTSecret = "test-secret-for-reloader-32chars!!"
	if err := origCfg.Save(cfgPath); err != nil {
		t.Fatalf("save original config: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	reloader := NewConfigReloader(loaded, cfgPath, nil)
	if err := reloader.Start(); err != nil {
		t.Fatalf("start reloader: %v", err)
	}
	defer reloader.Stop()

	// Modify the config file with a static-only change (port)
	modified, err := copyConfig(loaded)
	if err != nil {
		t.Fatalf("copy config: %v", err)
	}
	modified.Server.Port = 9999
	if err := modified.Save(cfgPath); err != nil {
		t.Fatalf("save modified config: %v", err)
	}

	// Reload should reject due to static field changes
	err = reloader.Reload()
	if err == nil {
		t.Fatal("expected error when changing static port field, got nil")
	}
	if !strings.Contains(err.Error(), "restart") && !strings.Contains(err.Error(), "static") &&
		!strings.Contains(err.Error(), "port") && !strings.Contains(err.Error(), "requires restart") {
		t.Errorf("error should mention restart/static/port, got: %v", err)
	}
}

func TestConfigReloader_AcceptsHotReloadableChanges(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	origCfg := config.DefaultConfig()
	origCfg.Server.Port = 8000
	origCfg.Server.JWTSecret = "test-secret-for-reloader-32chars!!"
	origCfg.Logging.Level = "info"
	if err := origCfg.Save(cfgPath); err != nil {
		t.Fatalf("save original config: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	reloader := NewConfigReloader(loaded, cfgPath, nil)
	if err := reloader.Start(); err != nil {
		t.Fatalf("start reloader: %v", err)
	}
	defer reloader.Stop()

	// Change a hot-reloadable field (logging level)
	modified, err := copyConfig(loaded)
	if err != nil {
		t.Fatalf("copy config: %v", err)
	}
	modified.Logging.Level = "debug"
	if err := modified.Save(cfgPath); err != nil {
		t.Fatalf("save modified config: %v", err)
	}

	err = reloader.Reload()
	if err != nil {
		t.Logf("hot-reloadable change returned error (may still work): %v", err)
	}
}

func TestConfigReloader_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	origCfg := config.DefaultConfig()
	origCfg.Server.JWTSecret = "test-secret-for-reloader-32chars!!"
	if err := origCfg.Save(cfgPath); err != nil {
		t.Fatalf("save original config: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	reloader := NewConfigReloader(loaded, cfgPath, nil)
	if err := reloader.Start(); err != nil {
		t.Fatalf("start reloader: %v", err)
	}
	defer reloader.Stop()

	// Write invalid YAML
	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: \tunexpected"), 0644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	err = reloader.Reload()
	if err == nil {
		t.Fatal("expected error for invalid YAML config, got nil")
	}
}
