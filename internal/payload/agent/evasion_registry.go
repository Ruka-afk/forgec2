//go:build linux || windows || darwin

package main

import "sync"

type evasionFunc func() string

var (
	evasionRegistry = map[string]evasionFunc{}
	evasionMu       sync.RWMutex
)

func registerEvasion(name string, fn evasionFunc) {
	evasionMu.Lock()
	defer evasionMu.Unlock()
	evasionRegistry[name] = fn
}

func runEvasion(name string) string {
	evasionMu.RLock()
	fn, ok := evasionRegistry[name]
	evasionMu.RUnlock()
	if !ok {
		return ""
	}
	return fn()
}

func listEvasionTechniques() []string {
	evasionMu.RLock()
	defer evasionMu.RUnlock()
	names := make([]string, 0, len(evasionRegistry))
	for name := range evasionRegistry {
		names = append(names, name)
	}
	return names
}
