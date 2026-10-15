//go:build !windows && !linux
// +build !windows,!linux

package plugin

// writeBytes is unavailable on this platform (no job-object accounting, no
// procfs). The watchdog reports errWriteQuotaUnsupported, logs once and
// degrades to the wall-clock timeout and CPU budget.
func (g *processGuard) writeBytes() (int64, error) {
	return 0, errWriteQuotaUnsupported
}
