package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/forgec2/forgec2/internal/db"
	"gorm.io/gorm"
)

// Manager loads, stores, and executes ForgeC2 plugins.
type Manager struct {
	db          *gorm.DB
	marketplace *Marketplace
	plugins     map[string]Plugin
	mu          sync.RWMutex
	pluginDir   string
	exec        *executor
}

// NewManager creates a new plugin manager backed by the given database.
func NewManager(database *gorm.DB) *Manager {
	return &Manager{
		db:        database,
		plugins:   make(map[string]Plugin),
		pluginDir: "plugins",
		exec:      &executor{},
	}
}

// SetMarketplace links the manager to an existing marketplace instance.
func (m *Manager) SetMarketplace(mp *Marketplace) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.marketplace = mp
}

// LoadFromDisk walks a directory of plugin packages and registers each one.
func (m *Manager) LoadFromDisk(dir string) error {
	m.mu.Lock()
	m.pluginDir = dir
	m.mu.Unlock()

	return m.loadFromDir(dir)
}

func (m *Manager) loadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "manifest.yaml")
		if _, err := os.Stat(manifestPath); err == nil {
			manifest, err := LoadManifest(manifestPath)
			if err != nil {
				slog.Warn("Failed to load plugin manifest", "path", manifestPath, "err", err)
				continue
			}
			if err := m.registerAtDir(manifest, pluginDir); err != nil {
				slog.Warn("Failed to register plugin", "name", manifest.Name, "err", err)
			}
			continue
		}
		// Allow nested directories (e.g. plugins/example/portscan).
		if err := m.loadFromDir(pluginDir); err != nil {
			slog.Warn("Failed to load plugins from subdirectory", "dir", pluginDir, "err", err)
		}
	}
	return nil
}

// Register persists a plugin manifest and loads it into memory.
func (m *Manager) Register(manifest *Manifest) error {
	return m.registerAtDir(manifest, m.pluginDirFor(manifest.Name))
}

func (m *Manager) registerAtDir(manifest *Manifest, pluginDir string) error {
	if err := manifest.Validate(); err != nil {
		return err
	}

	if err := manifest.Save(pluginDir); err != nil {
		return err
	}

	// Persist to DB.
	var record db.Plugin
	result := m.db.Where("name = ?", manifest.Name).First(&record)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}

	record.Name = manifest.Name
	record.Version = manifest.Version
	record.Description = manifest.Description
	record.Author = manifest.Author
	record.Type = manifest.Type
	record.Category = manifest.Category
	record.Config = manifest.ConfigJSON()
	if record.ID == 0 {
		record.Enabled = true
	}

	if err := m.db.Save(&record).Error; err != nil {
		return fmt.Errorf("failed to save plugin record: %w", err)
	}

	config := manifest.Config
	if record.Config != "" {
		var dbConfig map[string]interface{}
		if err := json.Unmarshal([]byte(record.Config), &dbConfig); err == nil {
			config = mergeMaps(config, dbConfig)
		}
	}

	p := &externalPlugin{
		manifest: manifest,
		dir:      pluginDir,
		enabled:  record.Enabled,
		exec:     m.exec,
	}
	if err := p.Init(context.Background(), config); err != nil {
		return fmt.Errorf("failed to initialise plugin %q: %w", manifest.Name, err)
	}

	m.mu.Lock()
	m.plugins[manifest.Name] = p
	m.mu.Unlock()

	slog.Info("Plugin registered", "name", manifest.Name, "type", manifest.Type, "version", manifest.Version, "dir", pluginDir)
	return nil
}

// Unregister removes a plugin from memory and the database.
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	pluginDir := ""
	if p, ok := m.plugins[name]; ok {
		if ep, ok := p.(*externalPlugin); ok {
			pluginDir = ep.dir
		}
	}
	delete(m.plugins, name)
	m.mu.Unlock()

	if err := m.db.Where("name = ?", name).Delete(&db.Plugin{}).Error; err != nil {
		return err
	}

	if pluginDir == "" {
		pluginDir = m.pluginDirFor(name)
	}
	if err := os.RemoveAll(pluginDir); err != nil {
		slog.Warn("Failed to remove plugin directory", "dir", pluginDir, "err", err)
	}
	return nil
}

// Get returns a loaded plugin by name.
func (m *Manager) Get(name string) (Plugin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}
	return p, nil
}

// List returns all loaded plugins.
func (m *Manager) List() []Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	return list
}

// SetEnabled updates whether a plugin is allowed to execute.
func (m *Manager) SetEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var record db.Plugin
	if err := m.db.Where("name = ?", name).First(&record).Error; err != nil {
		return err
	}
	record.Enabled = enabled
	if err := m.db.Save(&record).Error; err != nil {
		return err
	}

	if p, ok := m.plugins[name]; ok {
		if ep, ok := p.(*externalPlugin); ok {
			ep.enabled = enabled
		}
	}
	return nil
}

// ExecuteCommand runs a command plugin with the given agent and parameters.
func (m *Manager) ExecuteCommand(ctx context.Context, name, agentID string, params map[string]interface{}) (*Result, error) {
	p, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	if !p.IsEnabled() {
		return nil, fmt.Errorf("plugin %q is disabled", name)
	}
	cp, ok := p.(CommandPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not a command plugin", name)
	}
	return cp.Execute(ctx, agentID, params)
}

// ExecuteHook runs all hook plugins subscribed to the given event.
func (m *Manager) ExecuteHook(ctx context.Context, event Event) error {
	m.mu.RLock()
	plugins := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	errCh := make(chan error, len(plugins))
	for _, p := range plugins {
		hp, ok := p.(HookPlugin)
		if !ok || !p.IsEnabled() {
			continue
		}
		if !contains(hp.SubscribedEvents(), event.Type) {
			continue
		}
		wg.Add(1)
		go func(h HookPlugin) {
			defer wg.Done()
			if err := h.OnEvent(ctx, event); err != nil {
				slog.Warn("Hook plugin execution failed", "plugin", h.Name(), "event", event.Type, "err", err)
				errCh <- err
			}
		}(hp)
	}
	wg.Wait()
	close(errCh)

	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("hook execution errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GenerateReport runs a report plugin with the given parameters.
func (m *Manager) GenerateReport(ctx context.Context, name string, params map[string]interface{}) (*Report, error) {
	p, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	if !p.IsEnabled() {
		return nil, fmt.Errorf("plugin %q is disabled", name)
	}
	rp, ok := p.(ReportPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not a report plugin", name)
	}
	return rp.Generate(ctx, params)
}

// PluginDir returns the on-disk directory used for the named plugin.
func (m *Manager) PluginDir(name string) string {
	return m.pluginDirFor(name)
}

func (m *Manager) pluginDirFor(name string) string {
	safe := strings.ReplaceAll(name, "..", "_")
	safe = strings.ReplaceAll(safe, string(filepath.Separator), "_")
	safe = strings.ReplaceAll(safe, "/", "_")
	return filepath.Join(m.pluginDir, safe)
}

// externalPlugin adapts a Manifest into the Plugin interfaces.
type externalPlugin struct {
	manifest *Manifest
	dir      string
	enabled  bool
	config   map[string]interface{}
	exec     *executor
}

func (p *externalPlugin) Name() string        { return p.manifest.Name }
func (p *externalPlugin) Version() string     { return p.manifest.Version }
func (p *externalPlugin) Type() string        { return p.manifest.Type }
func (p *externalPlugin) Description() string { return p.manifest.Description }
func (p *externalPlugin) IsEnabled() bool     { return p.enabled }
func (p *externalPlugin) Manifest() *Manifest { return p.manifest }

func (p *externalPlugin) Init(ctx context.Context, config map[string]interface{}) error {
	p.config = config
	if p.config == nil {
		p.config = make(map[string]interface{})
	}
	return nil
}

func (p *externalPlugin) Shutdown() error { return nil }

func (p *externalPlugin) Execute(ctx context.Context, agentID string, params map[string]interface{}) (*Result, error) {
	input := map[string]interface{}{
		"agent_id": agentID,
		"params":   params,
		"config":   p.config,
	}
	res, err := p.exec.run(ctx, p.dir, p.manifest, input, p.manifest.Timeout)
	if err != nil {
		return nil, err
	}
	r, parseErr := parseResult(res.Stdout)
	if parseErr != nil {
		return &Result{
			Success: false,
			Output:  string(res.Stdout),
			Error:   parseErr.Error(),
		}, nil
	}
	return r, nil
}

func (p *externalPlugin) OnEvent(ctx context.Context, event Event) error {
	input := map[string]interface{}{
		"event":  event,
		"config": p.config,
	}
	res, err := p.exec.run(ctx, p.dir, p.manifest, input, p.manifest.Timeout)
	if err != nil {
		return err
	}
	if len(res.Stdout) == 0 {
		return nil
	}
	_, err = parseResult(res.Stdout)
	return err
}

func (p *externalPlugin) SubscribedEvents() []string { return p.manifest.Events }

func (p *externalPlugin) Generate(ctx context.Context, params map[string]interface{}) (*Report, error) {
	input := map[string]interface{}{
		"params": params,
		"config": p.config,
	}
	res, err := p.exec.run(ctx, p.dir, p.manifest, input, p.manifest.Timeout)
	if err != nil {
		return nil, err
	}
	return parseReport(res.Stdout)
}

func contains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

func mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
