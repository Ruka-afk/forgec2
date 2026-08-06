//go:build linux && arm64

package main

import "syscall"

func ptraceGetIP(regs *syscall.PtraceRegs) uint64 {
	return regs.Pc
}

func ptraceSetIP(regs *syscall.PtraceRegs, ip uint64) {
	regs.Pc = ip
}