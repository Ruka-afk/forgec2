//go:build !windows

package server

import (
	"sync"
	"syscall"
	"time"
)

var (
	prevCPUTime    = time.Time{}
	prevUserTime   int64
	prevSystemTime int64
	cpuMu          sync.Mutex
)

func (m *MonitorCollector) getCPULoad() float64 {
	cpuMu.Lock()
	defer cpuMu.Unlock()
	now := time.Now()
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err != nil {
		return 0
	}

	ut := rusage.Utime.Nano()
	st := rusage.Stime.Nano()

	if prevCPUTime.IsZero() {
		prevCPUTime = now
		prevUserTime = ut
		prevSystemTime = st
		return 0
	}

	elapsed := now.Sub(prevCPUTime).Nanoseconds()
	cpuDelta := (ut - prevUserTime) + (st - prevSystemTime)

	prevCPUTime = now
	prevUserTime = ut
	prevSystemTime = st

	if elapsed <= 0 {
		return 0
	}

	load := float64(cpuDelta) / float64(elapsed) * 100
	if load > 100 {
		load = 100
	}
	return load
}

func (m *MonitorCollector) getDiskStats() struct{ used, total float64 } {
	dataDir := m.server.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		return struct{ used, total float64 }{0, 1}
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free

	if total <= 0 {
		return struct{ used, total float64 }{0, 1}
	}

	return struct{ used, total float64 }{float64(used), float64(total)}
}
