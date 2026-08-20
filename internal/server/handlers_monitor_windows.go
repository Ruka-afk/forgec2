//go:build windows
// +build windows

package server

import (
	"math"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessTimes           = modkernel32.NewProc("GetProcessTimes")
	procGetDiskFreeSpaceExW       = modkernel32.NewProc("GetDiskFreeSpaceExW")
	prevCPUKernelTime       int64 = 0
	prevCPUUserTime         int64 = 0
	prevCPUTime                   = time.Time{}
	cpuMu                   sync.Mutex
)

func (m *MonitorCollector) getCPULoad() (float64, bool) {
	cpuMu.Lock()
	defer cpuMu.Unlock()
	now := time.Now()
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	ret, _, _ := procGetProcessTimes.Call(
		uintptr(windows.CurrentProcess()),
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		// A failed read must never masquerade as a flat 0% (healthy) reading.
		return math.NaN(), false
	}

	kt := kernelTime.Nanoseconds()
	ut := userTime.Nanoseconds()

	if prevCPUTime.IsZero() {
		prevCPUTime = now
		prevCPUKernelTime = kt
		prevCPUUserTime = ut
		return 0, true
	}

	elapsed := now.Sub(prevCPUTime).Nanoseconds()
	cpuDelta := (kt - prevCPUKernelTime) + (ut - prevCPUUserTime)

	prevCPUTime = now
	prevCPUKernelTime = kt
	prevCPUUserTime = ut

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
	path, err := windows.UTF16PtrFromString(dataDir)
	if err != nil {
		return 0, 0, false
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	ret, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 || totalBytes <= 0 {
		return 0, 0, false
	}

	usedBytes := totalBytes - totalFreeBytes
	return float64(usedBytes), float64(totalBytes), true
}
