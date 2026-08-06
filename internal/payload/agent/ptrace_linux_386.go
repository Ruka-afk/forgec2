//go:build linux && 386

package main

import "syscall"

func ptraceGetIP(regs *syscall.PtraceRegs) uint64 {
	return uint64(regs.Eip)
}

func ptraceSetIP(regs *syscall.PtraceRegs, ip uint64) {
	regs.Eip = uint32(ip)
}