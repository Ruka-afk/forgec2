//go:build windows
// +build windows

package plugin

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// jobObjectBasicAndIOAccountingInformation mirrors the Win32
// JOBOBJECT_BASIC_AND_IO_ACCOUNTING_INFORMATION struct: the basic accounting
// block followed by the I/O counters. x/sys declares the information class
// and IO_COUNTERS, but not the combined struct.
type jobObjectBasicAndIOAccountingInformation struct {
	BasicInfo jobObjectBasicAccountingInformation
	IOInfo    windows.IO_COUNTERS
}

// writeBytes returns the bytes every process the run's job object has hosted
// so far has written — files, pipes and console alike. The job is created per
// run, so the counter starts at zero and includes children that already
// exited (a writer that filled the disk and then died still counts).
func (g *processGuard) writeBytes() (int64, error) {
	if g == nil {
		return 0, errGuardGone
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job == nil {
		return 0, errGuardGone
	}
	var info jobObjectBasicAndIOAccountingInformation
	if err := windows.QueryInformationJobObject(
		g.job.handle,
		windows.JobObjectBasicAndIoAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return 0, err
	}
	return int64(info.IOInfo.WriteTransferCount), nil
}
