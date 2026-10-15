//go:build !windows && !linux
// +build !windows,!linux

package plugin

import "time"

// cpuTime is unavailable on this platform (no job-object accounting, no
// procfs). The watchdog reports errCPUQuotaUnsupported, logs once and
// degrades to the wall-clock timeout.
func (g *processGuard) cpuTime() (time.Duration, error) {
	return 0, errCPUQuotaUnsupported
}
