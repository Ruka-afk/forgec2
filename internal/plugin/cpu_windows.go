//go:build windows
// +build windows

package plugin

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobObjectBasicAccountingInformation mirrors the Win32
// JOBOBJECT_BASIC_ACCOUNTING_INFORMATION struct: four LARGE_INTEGER counts
// followed by four DWORD counters. x/sys declares the information class but
// not the struct.
type jobObjectBasicAccountingInformation struct {
	TotalUserTime            int64
	TotalKernelTime          int64
	ThisPeriodUserTime       int64
	ThisPeriodKernelTime     int64
	TotalPageFaultCount      uint32
	TotalProcesses           uint32
	ActiveProcesses          uint32
	TotalTerminatedProcesses uint32
}

// cpuTime returns the CPU time consumed by every process the run's job object
// has hosted so far — kernel plus user time, in 100-ns units. The job is
// created per run, so the counters start at zero and include children that
// already exited.
func (g *processGuard) cpuTime() (time.Duration, error) {
	if g == nil {
		return 0, errCPUGuardGone
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job == nil {
		return 0, errCPUGuardGone
	}
	var info jobObjectBasicAccountingInformation
	if err := windows.QueryInformationJobObject(
		g.job.handle,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return 0, err
	}
	return time.Duration(info.TotalUserTime+info.TotalKernelTime) * 100 * time.Nanosecond, nil
}
