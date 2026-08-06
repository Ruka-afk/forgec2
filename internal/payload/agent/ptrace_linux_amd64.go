//go:build linux && amd64

package main

import "syscall"

func ptraceGetIP(regs *syscall.PtraceRegs) uint64 {
	return regs.Rip
}

func ptraceSetIP(regs *syscall.PtraceRegs, ip uint64) {
	regs.Rip = ip
}