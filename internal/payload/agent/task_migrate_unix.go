//go:build linux || darwin
// +build linux darwin

package main

import "syscall"

// detachedSysProcAttr starts the migrated copy in a new session so it survives
// this process exiting and is fully detached from the caller's process group.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}