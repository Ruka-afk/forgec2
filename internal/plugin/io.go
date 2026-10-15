package plugin

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// defaultPluginWriteQuota bounds how many bytes one native plugin run may
// write before its process tree is killed. The wall-clock ceiling and the CPU
// budget still apply; the write budget exists so a plugin that streams output
// to disk (a runaway archive, a log flood, a copy loop) cannot fill the volume
// it runs on while its wall-clock window lasts.
//
// The budget covers every write the kernel attributes to the tree — files,
// pipes and console alike. That is deliberate: bytes the plugin throws away
// are capped by the output buffers anyway, so the only writes this quota can
// cut short are ones already going nowhere useful.
const defaultPluginWriteQuota int64 = 1 << 30 // 1 GiB

// defaultQuotaPollInterval is how often the run watchdogs sample the plugin's
// process tree (CPU time and written bytes), when the executor does not
// request a different interval (tests shrink it). Sampling is cheap but not
// free (on Linux each sample walks /proc), so the interval is deliberately
// coarse: one second turns a runaway plugin into a bounded incident.
const defaultQuotaPollInterval = time.Second

// errWriteQuotaUnsupported reports platforms without plugin write accounting
// (everything but Linux and Windows). The watchdog logs once and degrades to
// the wall-clock timeout and CPU budget instead of failing runs, because the
// quota is a resource guard, not an isolation guarantee.
var errWriteQuotaUnsupported = errors.New("plugin write accounting is unsupported on this platform")

var (
	writeQuotaUnsupportedOnce sync.Once
	writeQuotaSampleErrOnce   sync.Once
)

// watchWriteQuota samples the guarded process tree's written bytes and kills
// the tree once the run spends its budget. It is a watchdog, not a hard limit:
// a sample lands at most one interval after the budget is exhausted.
//
// done is the run's completion channel; closing it stops the watchdog. On
// success (budget exhausted) the watchdog closes exceeded before releasing the
// guard so the run's error path can tell a write kill apart from a timeout.
func watchWriteQuota(guard *processGuard, budget int64, interval time.Duration, done <-chan struct{}, exceeded chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			written, err := guard.writeBytes()
			if err != nil {
				switch {
				case errors.Is(err, errGuardGone):
					return
				case errors.Is(err, errWriteQuotaUnsupported):
					writeQuotaUnsupportedOnce.Do(func() {
						slog.Warn("plugin write accounting unavailable on this platform; plugin runs stay bounded by the wall-clock timeout and CPU budget only")
					})
				default:
					writeQuotaSampleErrOnce.Do(func() {
						slog.Warn("plugin write accounting failed; plugin runs stay bounded by the wall-clock timeout and CPU budget only", "error", err)
					})
				}
				return
			}
			if written >= budget {
				close(exceeded)
				guard.release()
				return
			}
		}
	}
}
