package server

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/fsnotify/fsnotify"
)

type ConfigReloader struct {
	cfg        *config.Config
	path       string
	watcher    *fsnotify.Watcher
	mu         sync.Mutex
	running    bool
	onReload   func(*config.Config)
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

	dir := filepath.Dir(r.path)
	if err := watcher.Add(dir); err != nil {
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

			if filepath.Base(event.Name) != filepath.Base(r.path) {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			if debounce != nil {
				debounce.Stop()
			}

			debounce = time.AfterFunc(500*time.Millisecond, func() {
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

	*r.cfg = *newCfg

	if r.onReload != nil {
		r.onReload(r.cfg)
	}

	slog.Info("Config reloaded successfully")
}