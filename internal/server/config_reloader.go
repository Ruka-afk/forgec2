package server

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/fsnotify/fsnotify"
)

type ConfigReloader struct {
	cfg      *config.Config
	path     string
	watcher  *fsnotify.Watcher
	mu       sync.Mutex
	running  bool
	onReload func(*config.Config)
}

func NewConfigReloader(cfg *config.Config, path string, onReload func(*config.Config)) *ConfigReloader {
	return &ConfigReloader{
		cfg:      cfg,
		path:     path,
		onReload: onReload,
	}
}

func (r *ConfigReloader) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.mu.Unlock()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	r.watcher = watcher

	if err := watcher.Add(r.path); err != nil {
		watcher.Close()
		return err
	}

	slog.Info("Config reloader started", "path", r.path)

	go r.monitor()

	return nil
}

func (r *ConfigReloader) Stop() {
	r.mu.Lock()
	r.running = false
	if r.watcher != nil {
		r.watcher.Close()
		r.watcher = nil
	}
	r.mu.Unlock()
	slog.Info("Config reloader stopped")
}

func (r *ConfigReloader) monitor() {
	var debounce *time.Timer

	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}

			if event.Name != r.path {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			if debounce != nil {
				debounce.Stop()
			}

			debounce = time.AfterFunc(ConfigReloadDebounce, func() {
				r.reload()
			})

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("Config watcher error", "error", err)
		}
	}
}

func (r *ConfigReloader) reload() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	slog.Info("Detected config change, reloading", "path", r.path)

	data, err := os.ReadFile(r.path)
	if err != nil {
		slog.Error("Failed to read config file", "error", err)
		return
	}

	newCfg := config.DefaultConfig()
	if err := newCfg.LoadFromData(data); err != nil {
		slog.Error("Failed to parse config file", "error", err)
		return
	}

	if err := newCfg.Validate(); err != nil {
		slog.Error("New config failed validation, keeping current config", "error", err)
		return
	}

	changed := diffConfig(r.cfg, newCfg)
	if len(changed) == 0 {
		slog.Info("Config file changed but no values differ, skipping reload")
		return
	}
	slog.Info("Config fields changed", "fields", strings.Join(changed, ", "))

	r.cfg.CopyFrom(newCfg)

	if r.onReload != nil {
		r.onReload(r.cfg)
	}

	slog.Info("Config reloaded successfully", "changed_fields", len(changed))
}

// Reload manually triggers a config reload from disk. Returns an error if the
// reload fails (e.g., file not found, parse error). Safe for concurrent use.
func (r *ConfigReloader) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Info("Manual config reload triggered", "path", r.path)

	data, err := os.ReadFile(r.path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	newCfg := config.DefaultConfig()
	if err := newCfg.LoadFromData(data); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	r.cfg.CopyFrom(newCfg)

	if r.onReload != nil {
		r.onReload(r.cfg)
	}

	slog.Info("Manual config reload completed")
	return nil
}

func diffConfig(old, new *config.Config) []string {
	var changed []string
	if old.Server.Port != new.Server.Port {
		changed = append(changed, "server.port")
	}
	if old.Server.BeaconKey != new.Server.BeaconKey {
		changed = append(changed, "server.beacon_key")
	}
	if old.Crypto.Key != new.Crypto.Key {
		changed = append(changed, "crypto.key")
	}
	if old.Server.JWTSecret != new.Server.JWTSecret {
		changed = append(changed, "server.jwt_secret")
	}
	if old.Database.Driver != new.Database.Driver {
		changed = append(changed, "database.driver")
	}
	if old.Database.Path != new.Database.Path {
		changed = append(changed, "database.path")
	}
	if old.Logging.Level != new.Logging.Level {
		changed = append(changed, "logging.level")
	}
	if old.Server.TLSEnabled != new.Server.TLSEnabled || old.Server.CertFile != new.Server.CertFile || old.Server.KeyFile != new.Server.KeyFile {
		changed = append(changed, "server.tls")
	}
	if old.SIEM.Enabled != new.SIEM.Enabled || old.SIEM.URL != new.SIEM.URL {
		changed = append(changed, "siem")
	}
	if old.PasswordPolicy.MinLength != new.PasswordPolicy.MinLength {
		changed = append(changed, "password_policy")
	}
	if old.RateLimit.Login.MaxAttempts != new.RateLimit.Login.MaxAttempts {
		changed = append(changed, "rate_limit.login")
	}
	if old.Server.DNSEnabled != new.Server.DNSEnabled || old.Server.DNSDomain != new.Server.DNSDomain {
		changed = append(changed, "server.dns")
	}
	if old.Server.GRPCEnabled != new.Server.GRPCEnabled {
		changed = append(changed, "server.grpc")
	}
	return changed
}
