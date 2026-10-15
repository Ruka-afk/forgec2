package plugin

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// defaultPluginCPUQuota bounds how much CPU time one native plugin run may
// burn across its whole process tree before the tree is killed. The wall-clock
// ceiling (maxPluginTimeoutSecs) still applies; the budget exists so a
// spinning plugin cannot saturate a core for the entire wall-clock window.
const defaultPluginCPUQuota = 120 * time.Second

// cpuQuotaPollInterval is how often the CPU watchdog samples the plugin's
// process tree. Sampling is cheap but not free (on Linux one sample walks
// /proc), so the interval is deliberately coarse. It is a variable so tests
// can shrink it.
var cpuQuotaPollInterval = time.Second

// errCPUQuotaUnsupported reports platforms without plugin CPU accounting
// (everything but Linux and Windows). The watchdog logs once and degrades to
// wall-clock enforcement instead of failing runs, because the quota is a
// resource guard, not an isolation guarantee.
var errCPUQuotaUnsupported = errors.New("plugin CPU accounting is unsupported on this platform")

// errCPUGuardGone reports that the guard was already released (the run ended)
// before a sample could be taken; the watchdog exits silently.
var errCPUGuardGone = errors.New("plugin process guard already released")

var (
	cpuQuotaUnsupportedOnce sync.Once
	cpuQuotaSampleErrOnce   sync.Once
)

// watchCPUQuota samples the guarded process tree's CPU time and kills the tree
// once the run spends its budget. It is a watchdog, not a hard limit: a sample
// lands at most one cpuQuotaPollInterval after the budget is exhausted.
//
// done is the run's completion channel; closing it stops the watchdog. On
// success (budget exhausted) the watchdog closes exceeded before releasing the
// guard so the run's error path can tell a CPU kill apart from a timeout.
func watchCPUQuota(guard *processGuard, budget time.Duration, done <-chan struct{}, exceeded chan struct{}) {
	ticker := time.NewTicker(cpuQuotaPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			used, err := guard.cpuTime()
			if err != nil {
				switch {
				case errors.Is(err, errCPUGuardGone):
					return
				case errors.Is(err, errCPUQuotaUnsupported):
					cpuQuotaUnsupportedOnce.Do(func() {
						slog.Warn("plugin CPU accounting unavailable on this platform; plugin runs stay bounded by the wall-clock timeout only")
					})
				default:
					cpuQuotaSampleErrOnce.Do(func() {
						slog.Warn("plugin CPU accounting failed; plugin runs stay bounded by the wall-clock timeout only", "error", err)
					})
				}
				return
			}
			if used >= budget {
				close(exceeded)
				guard.release()
				return
			}
		}
	}
}
