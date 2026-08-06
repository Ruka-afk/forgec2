//go:build windows

package main

import "syscall"

// detachedSysProcAttr launches the migrated copy with no console window on the
// current desktop session (no new console flash during migration).
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000 | 0x00000004, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	}
}