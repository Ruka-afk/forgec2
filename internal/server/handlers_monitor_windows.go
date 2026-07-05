//go:build windows
// +build windows

package server

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGetProcessTimes      = modkernel32.NewProc("GetProcessTimes")
	procGetDiskFreeSpaceExW  = modkernel32.NewProc("GetDiskFreeSpaceExW")
	prevCPUKernelTime  int64 = 0
	prevCPUUserTime    int64 = 0
	prevCPUTime              = time.Time{}
	cpuMu              sync.Mutex
)

func (m *MonitorCollector) getCPULoad() float64 {
	cpuMu.Lock()
	defer cpuMu.Unlock()
	now := time.Now()
	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	procGetProcessTimes.Call(
		uintptr(windows.CurrentProcess()),
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)

	kt := kernelTime.Nanoseconds()
	ut := userTime.Nanoseconds()

	if prevCPUTime.IsZero() {
		prevCPUTime = now
		prevCPUKernelTime = kt
		prevCPUUserTime = ut
		return 0
	}

	elapsed := now.Sub(prevCPUTime).Nanoseconds()
	cpuDelta := (kt - prevCPUKernelTime) + (ut - prevCPUUserTime)

	prevCPUTime = now
	prevCPUKernelTime = kt
	prevCPUUserTime = ut

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
	path, err := windows.UTF16PtrFromString(dataDir)
	if err != nil {
		return struct{ used, total float64 }{0, 1}
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	ret, _, _ := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 || totalBytes <= 0 {
		return struct{ used, total float64 }{0, 1}
	}

	used := totalBytes - totalFreeBytes
	return struct{ used, total float64 }{float64(used), float64(totalBytes)}
}
