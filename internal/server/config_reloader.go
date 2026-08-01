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
	onReload func(*config.Config, []string)
}

func NewConfigReloader(cfg *config.Config, path string, onReload func(*config.Config, []string)) *ConfigReloader {
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
	r.mu.Lock()
	w := r.watcher
	r.mu.Unlock()

	var debounce *time.Timer

	for {
		select {
		case event, ok := <-w.Events:
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

		case err, ok := <-w.Errors:
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

	changed, staticOnly := diffConfig(r.cfg, newCfg)
	if len(changed) == 0 && len(staticOnly) == 0 {
		slog.Info("Config file changed but no values differ, skipping reload")
		return
	}
	if len(staticOnly) > 0 {
		slog.Warn("Config file changed with non-hot-reloadable fields (restart required), rejecting reload", "fields", strings.Join(staticOnly, ", "))
		return
	}
	slog.Info("Config fields changed", "fields", strings.Join(changed, ", "))

	if r.onReload != nil {
		r.onReload(newCfg, changed)
	}
	r.cfg = newCfg

	slog.Info("Config reloaded successfully", "changed_fields", len(changed))
}

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

	changed, staticOnly := diffConfig(r.cfg, newCfg)
	if len(staticOnly) > 0 {
		return fmt.Errorf("config has non-hot-reloadable fields that require restart: %s", strings.Join(staticOnly, ", "))
	}

	if r.onReload != nil {
		r.onReload(newCfg, changed)
	}
	r.cfg = newCfg

	slog.Info("Manual config reload completed", "changed_fields", len(changed))
	return nil
}

var nonHotReloadableFields = map[string]bool{
	"server.port":    true,
	"server.tls":     true,
	"server.dns":     true,
	"server.grpc":    true,
	"database.driver": true,
	"database.path":  true,
}

func diffConfig(old, new *config.Config) (hotReloadable []string, staticOnly []string) {
	if old.Server.Port != new.Server.Port {
		staticOnly = append(staticOnly, "server.port")
	}
	if old.Server.BeaconKey != new.Server.BeaconKey {
		hotReloadable = append(hotReloadable, "server.beacon_key")
	}
	if old.Crypto.Key != new.Crypto.Key {
		hotReloadable = append(hotReloadable, "crypto.key")
	}
	if old.Server.JWTSecret != new.Server.JWTSecret {
		hotReloadable = append(hotReloadable, "server.jwt_secret")
	}
	if old.Database.Driver != new.Database.Driver {
		staticOnly = append(staticOnly, "database.driver")
	}
	if old.Database.Path != new.Database.Path {
		staticOnly = append(staticOnly, "database.path")
	}
	if old.Logging.Level != new.Logging.Level {
		hotReloadable = append(hotReloadable, "logging.level")
	}
	if old.Server.TLSEnabled != new.Server.TLSEnabled || old.Server.CertFile != new.Server.CertFile || old.Server.KeyFile != new.Server.KeyFile {
		staticOnly = append(staticOnly, "server.tls")
	}
	if old.SIEM.Enabled != new.SIEM.Enabled || old.SIEM.URL != new.SIEM.URL {
		hotReloadable = append(hotReloadable, "siem")
	}
	if old.PasswordPolicy.MinLength != new.PasswordPolicy.MinLength {
		hotReloadable = append(hotReloadable, "password_policy")
	}
	if old.RateLimit.Login.MaxAttempts != new.RateLimit.Login.MaxAttempts {
		hotReloadable = append(hotReloadable, "rate_limit.login")
	}
	if old.Server.DNSEnabled != new.Server.DNSEnabled || old.Server.DNSDomain != new.Server.DNSDomain {
		staticOnly = append(staticOnly, "server.dns")
	}
	if old.Server.GRPCEnabled != new.Server.GRPCEnabled {
		staticOnly = append(staticOnly, "server.grpc")
	}
	if len(staticOnly) > 0 {
		slog.Warn("Config file changed with non-hot-reloadable fields (restart required)", "fields", staticOnly)
	}
	return
}
