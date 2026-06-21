package server

import "github.com/forgec2/forgec2/internal/db"

// PluginSDK defines the interface for forgec2 plugins
type PluginSDK interface {
	Name() string
	Version() string
	Description() string
	Init(db *db.Plugin) error
}

type PluginManager struct {
	plugins map[string]PluginSDK
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: make(map[string]PluginSDK),
	}
}

func (pm *PluginManager) Register(p PluginSDK) {
	pm.plugins[p.Name()] = p
}

func (pm *PluginManager) Get(name string) PluginSDK {
	return pm.plugins[name]
}

func (pm *PluginManager) List() []PluginSDK {
	var list []PluginSDK
	for _, p := range pm.plugins {
		list = append(list, p)
	}
	return list
}
