//go:build !windows

package server

import (
	"math"
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

func (m *MonitorCollector) getCPULoad() (float64, bool) {
	cpuMu.Lock()
	defer cpuMu.Unlock()
	now := time.Now()
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err != nil {
		// A failed read must never masquerade as a flat 0% (healthy) reading.
		return math.NaN(), false
	}

	ut := rusage.Utime.Nano()
	st := rusage.Stime.Nano()

	if prevCPUTime.IsZero() {
		prevCPUTime = now
		prevUserTime = ut
		prevSystemTime = st
		return 0, true
	}

	elapsed := now.Sub(prevCPUTime).Nanoseconds()
	cpuDelta := (ut - prevUserTime) + (st - prevSystemTime)

	prevCPUTime = now
	prevUserTime = ut
	prevSystemTime = st

	if elapsed <= 0 {
		return 0, true
	}

	load := float64(cpuDelta) / float64(elapsed) * 100
	if load > 100 {
		load = 100
	}
	return load, true
}

func (m *MonitorCollector) getDiskStats() (used, total float64, ok bool) {
	dataDir := m.server.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "."
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		return 0, 0, false
	}

	totalBytes := stat.Blocks * uint64(stat.Bsize)
	if totalBytes <= 0 {
		return 0, 0, false
	}

	free := stat.Bfree * uint64(stat.Bsize)
	usedBytes := totalBytes - free
	return float64(usedBytes), float64(totalBytes), true
}
