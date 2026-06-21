//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// SelfDelete securely deletes the agent binary by overwriting and removing it.
func selfDelete() string {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("self_delete: get exe path failed: %v", err)
	}
	stat, err := os.Stat(exe)
	if err != nil {
		return fmt.Sprintf("self_delete: stat failed: %v", err)
	}
	size := stat.Size()

	// Overwrite with zeros in chunks
	chunkSize := int64(4096)
	zero := make([]byte, chunkSize)
	f, err := os.OpenFile(exe, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Sprintf("self_delete: open failed: %v", err)
	}
	defer f.Close()

	for offset := int64(0); offset < size; offset += chunkSize {
		writeLen := chunkSize
		if offset+writeLen > size {
			writeLen = size - offset
		}
		f.WriteAt(zero[:writeLen], offset)
	}
	f.Close()

	// Delete via move-on-reboot for running binary
	k32 := syscall.NewLazyDLL("kernel32.dll")
	moveFileEx := k32.NewProc("MoveFileExW")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	moveFileEx.Call(uintptr(unsafe.Pointer(exePtr)), 0, 4) // MOVEFILE_DELAY_UNTIL_REBOOT

	os.Remove(exe)
	return fmt.Sprintf("self_delete: overwritten and removed %s (%d bytes)", exe, size)
}
