//go:build !windows
// +build !windows

package plugin

import (
	"errors"
	"os"
	"sync"
	"syscall"
)

// processGuard kills the plugin's entire process group on release so
// interpreter children (shells, go run toolchains) cannot outlive a
// timeout or failed run. release is safe to call concurrently: the timeout
// watchdog and the main wait path both call it.
type processGuard struct {
	pgid int
	once sync.Once
}

// attachProcessGuard verifies the child leads its own process group (set via
// Setpgid before Start). Returns an error when no guard can be established —
// the caller then refuses the run rather than executing without tree-kill.
func attachProcessGuard(p *os.Process) (*processGuard, error) {
	if p == nil || p.Pid <= 0 {
		return nil, errors.New("no process to guard")
	}
	// The child was started with Setpgid, so its pid is also the pgid. Probe
	// the group; ESRCH here normally means the process already exited, which
	// is not a containment failure (there is nothing left to contain).
	if err := syscall.Kill(-p.Pid, 0); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return &processGuard{pgid: p.Pid}, nil
		}
		return nil, err
	}
	return &processGuard{pgid: p.Pid}, nil
}

// release terminates the process group (negative pid). Safe to call twice or
// concurrently.
func (g *processGuard) release() {
	if g == nil {
		return
	}
	g.once.Do(func() {
		if g.pgid <= 0 {
			return
		}
		// SIGKILL the group; ignore ESRCH (already exited).
		_ = syscall.Kill(-g.pgid, syscall.SIGKILL)
	})
}
