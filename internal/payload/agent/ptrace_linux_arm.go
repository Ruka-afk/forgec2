//go:build linux && arm

package main

import "syscall"

// On 32-bit ARM the kernel's struct pt_regs is exposed by Go as Uregs[18]:
// r0-r15 occupy indices 0-15, CPSR is 16 and ORIG_R0 is 17. The program
// counter is therefore r15.
func ptraceGetIP(regs *syscall.PtraceRegs) uint64 {
	return uint64(regs.Uregs[15])
}

func ptraceSetIP(regs *syscall.PtraceRegs, ip uint64) {
	regs.Uregs[15] = uint32(ip)
}
