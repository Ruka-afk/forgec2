//go:build !windows
// +build !windows

package plugin

import (
	"os"
	"syscall"
)

// processGuard kills the plugin's entire process group on release so
// interpreter children (shells, go run toolchains) cannot outlive a
// timeout or failed run.
type processGuard struct {
	pgid int
	done bool
}

// attachProcessGuard puts the child in its own process group (Setpgid)
// before it does work. Must be called after cmd.Start (pid is known).
func attachProcessGuard(p *os.Process) (*processGuard, error) {
	if p == nil {
		return &processGuard{}, nil
	}
	// Best-effort: if the child already called setpgid differently, fail soft —
	// CommandContext timeout still stops the main process.
	if err := syscall.Kill(-p.Pid, 0); err == nil {
		// Already in a killable group with this pgid.
		return &processGuard{pgid: p.Pid}, nil
	}
	g := &processGuard{pgid: p.Pid}
	return g, nil
}

// release terminates the process group (negative pid). Safe to call twice.
func (g *processGuard) release() {
	if g == nil || g.done || g.pgid <= 0 {
		return
	}
	g.done = true
	// SIGKILL the group; ignore ESRCH (already exited).
	_ = syscall.Kill(-g.pgid, syscall.SIGKILL)
}
