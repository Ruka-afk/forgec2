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
// NOTE: The hardware-breakpoint VEH handler requires correct post-function
// resume (instruction-length aware RIP advance plus a clean return value).
// It is intentionally not implemented so operators are not misled into
// believing AMSI/ETW is bypassed.

// HardwareBreakpointAMSI sets a hardware execution breakpoint on AmsiScanBuffer.
// When the breakpoint fires, the VEH modifies EAX to return AMSI_RESULT_CLEAN.
func HardwareBreakpointAMSI() string {
	return "HWBP AMSI: not implemented (handler needs assembly trampoline; AMSI NOT bypassed)"
}

// HardwareBreakpointETW sets a hardware execution breakpoint on EtwEventWrite.
// When the breakpoint fires, the VEH modifies the return value to STATUS_SUCCESS.
func HardwareBreakpointETW() string {
	return "HWBP ETW: not implemented (handler needs assembly trampoline; ETW NOT bypassed)"
}