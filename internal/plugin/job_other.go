//go:build !windows
// +build !windows

package plugin

import "os"

// processGuard is a no-op on non-Windows platforms. Plugin processes there
// rely on exec.CommandContext's timeout; process-tree kill is not enforced.
type processGuard struct{}

func attachProcessGuard(p *os.Process) (*processGuard, error) {
	return &processGuard{}, nil
}

func (g *processGuard) release() {}
