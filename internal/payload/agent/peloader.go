//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	ntdll                       = syscall.NewLazyDLL("ntdll.dll")
	procVirtualAlloc            = kernel32.NewProc("VirtualAlloc")
	procVirtualFree             = kernel32.NewProc("VirtualFree")
	procVirtualProtect          = kernel32.NewProc("VirtualProtect")
	procLoadLibraryA            = kernel32.NewProc("LoadLibraryA")
	procGetProcAddress          = kernel32.NewProc("GetProcAddress")
	procGetModuleHandleA        = kernel32.NewProc("GetModuleHandleA")
	procFlushInstructionCache   = kernel32.NewProc("FlushInstructionCache")
	procGetCurrentProcess       = kernel32.NewProc("GetCurrentProcess")
	procWaitForSingleObject     = kernel32.NewProc("WaitForSingleObject")
	procCreateThread            = kernel32.NewProc("CreateThread")
	procRtlMoveMemory           = kernel32.NewProc("RtlMoveMemory")
)

// IMAGE_DOS_HEADER
type imageDOSHeader struct {
	eMagic    uint16
	eCblp     uint16
	eCp       uint16
	eCrlc     uint16
	eCparhdr  uint16
	eMinalloc uint16
	eMaxalloc uint16
	eSs       uint16
	eSp       uint16
	eCsum     uint16
	eIp       uint16
	eCs       uint16
	eLfarlc   uint16
	eOvno     uint16
	eRes      [4]uint16
	eOemid    uint16
	eOeminfo  uint16
	eRes2     [10]uint16
	eLfanew   int32
}

type imageNTHeaders64 struct {
	signature      uint32
	fileHeader     imageFileHeader
	optionalHeader imageOptionalHeader64
}

type imageNTHeaders32 struct {
	signature      uint32
	fileHeader     imageFileHeader
	optionalHeader imageOptionalHeader32
}

type imageFileHeader struct {
	machine              uint16
	numberOfSections     uint16
	timeDateStamp        uint32
	pointerToSymbolTable uint32
	numberOfSymbols      uint32
	sizeOfOptionalHeader uint16
	characteristics      uint16
}

type imageOptionalHeader64 struct {
	magic                       uint16
	majorLinkerVersion          uint8
	minorLinkerVersion          uint8
	sizeOfCode                  uint32
	sizeOfInitializedData       uint32
	sizeOfUninitializedData     uint32
	addressOfEntryPoint         uint32
	baseOfCode                  uint32
	imageBase                   uint64
	sectionAlignment            uint32
	fileAlignment               uint32
	majorOperatingSystemVersion uint16
	minorOperatingSystemVersion uint16
	majorImageVersion           uint16
	minorImageVersion           uint16
	majorSubsystemVersion       uint16
	minorSubsystemVersion       uint16
	win32VersionValue           uint32
	sizeOfImage                 uint32
	sizeOfHeaders               uint32
	checkSum                    uint32
	subsystem                   uint16
	dllCharacteristics          uint16
	sizeOfStackReserve          uint64
	sizeOfStackCommit           uint64
	sizeOfHeapReserve           uint64
	sizeOfHeapCommit            uint64
	loaderFlags                 uint32
	numberOfRvaAndSizes         uint32
	dataDirectory               [16]imageDataDirectory
}

type imageOptionalHeader32 struct {
	magic                       uint16
	majorLinkerVersion          uint8
	minorLinkerVersion          uint8
	sizeOfCode                  uint32
	sizeOfInitializedData       uint32
	sizeOfUninitializedData     uint32
	addressOfEntryPoint         uint32
	baseOfCode                  uint32
	imageBase                   uint32
	sectionAlignment            uint32
	fileAlignment               uint32
	majorOperatingSystemVersion uint16
	minorOperatingSystemVersion uint16
	majorImageVersion           uint16
	minorImageVersion           uint16
	majorSubsystemVersion       uint16
	minorSubsystemVersion       uint16
	win32VersionValue           uint32
	sizeOfImage                 uint32
	sizeOfHeaders               uint32
	checkSum                    uint32
	subsystem                   uint16
	dllCharacteristics          uint16
	sizeOfStackReserve          uint32
	sizeOfStackCommit           uint32
	sizeOfHeapReserve           uint32
	sizeOfHeapCommit            uint32
	loaderFlags                 uint32
	numberOfRvaAndSizes         uint32
	dataDirectory               [16]imageDataDirectory
}

type imageDataDirectory struct {
	virtualAddress uint32
	size           uint32
}

type imageSectionHeader struct {
	name                 [8]uint8
	virtualSize          uint32
	virtualAddress       uint32
	sizeOfRawData        uint32
	pointerToRawData     uint32
	pointerToRelocations uint32
	pointerToLinenumbers uint32
	numberOfRelocations  uint16
	numberOfLinenumbers  uint16
	characteristics      uint32
}

type imageBaseRelocation struct {
	virtualAddress uint32
	sizeOfBlock    uint32
}

type imageImportDescriptor struct {
	originalFirstThunk uint32
	timeDateStamp      uint32
	forwarderChain     uint32
	name               uint32
	firstThunk         uint32
}

type imageThunkData64 struct {
	addressOfData uint64
}

type imageThunkData32 struct {
	addressOfData uint32
}

type imageExportDirectory struct {
	characteristics       uint32
	timeDateStamp         uint32
	majorVersion          uint16
	minorVersion          uint16
	name                  uint32
	base                  uint32
	numberOfFunctions     uint32
	numberOfNames         uint32
	addressOfFunctions    uint32
	addressOfNames        uint32
	addressOfNameOrdinals uint32
}

const (
	imageDllcharacteristicsDynamicBase uint16 = 0x0040
	imageFileRelocsStripped            uint16 = 0x0001
	imageScnMemExecute                 uint32 = 0x20000000
	imageScnMemRead                    uint32 = 0x40000000
	imageScnMemWrite                   uint32 = 0x80000000
	imageScnMemDiscardable             uint32 = 0x02000000
	relBasedDir64                      uint8  = 0x0A
	relBasedAbs                        uint8  = 0x00
	relBasedHighlow                    uint8  = 0x03
	imageSnpByOrdinal                  uint32 = 0x80000000
	imageOrdFlag32                     uint32 = 0x0000FFFF
	dllProcessAttach                   uint32 = 1
	dllProcessDetach                   uint32 = 0
	pageExecuteRead                    uint32 = 0x20
	pageReadwrite                      uint32 = 0x04
	pageExecuteReadwrite               uint32 = 0x40
	memCommit                          uint32 = 0x1000
	memReserve                         uint32 = 0x2000
	infinity                           uint32 = 0xFFFFFFFF
)

func peAligment(value, alignment uint32) uint32 {
	return (value + alignment - 1) & ^(alignment - 1)
}

func reflectDLL(dllData []byte) (uintptr, error) {
	if len(dllData) < 64 {
		return 0, fmt.Errorf("PE too small")
	}

	dos := (*imageDOSHeader)(unsafe.Pointer(&dllData[0]))
	if dos.eMagic != 0x5A4D {
		return 0, fmt.Errorf("invalid DOS header magic")
	}
	if int(dos.eLfanew) >= len(dllData)-4 {
		return 0, fmt.Errorf("invalid e_lfanew")
	}

	ntOffset := uintptr(dos.eLfanew)
	nt32 := (*imageNTHeaders32)(unsafe.Pointer(&dllData[ntOffset]))
	if nt32.signature != 0x00004550 {
		return 0, fmt.Errorf("invalid NT signature")
	}

	var is64Bit bool
	var imageBaseDelta uint64
	var sizeOfImage uint32
	var sizeOfHeaders uint32
	var entryPoint uint32
	var imageBase uint64
	var numberOfSections uint16
	var sectionOffset uintptr

	if nt32.optionalHeader.magic == 0x10B {
		is64Bit = false
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		imageBase = uint64(oh.imageBase)
		sizeOfImage = oh.sizeOfImage
		sizeOfHeaders = oh.sizeOfHeaders
		entryPoint = oh.addressOfEntryPoint
		numberOfSections = nt32.fileHeader.numberOfSections
		sectionOffset = ntOffset + 24 + uintptr(nt32.fileHeader.sizeOfOptionalHeader)
	} else if nt32.optionalHeader.magic == 0x20B {
		is64Bit = true
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		imageBase = oh.imageBase
		sizeOfImage = oh.sizeOfImage
		sizeOfHeaders = oh.sizeOfHeaders
		entryPoint = oh.addressOfEntryPoint
		numberOfSections = nt32.fileHeader.numberOfSections
		sectionOffset = ntOffset + 24 + uintptr(nt32.fileHeader.sizeOfOptionalHeader)
	} else {
		return 0, fmt.Errorf("unsupported PE magic 0x%04X", nt32.optionalHeader.magic)
	}

	if sizeOfImage == 0 || sizeOfImage > 100*1024*1024 {
		return 0, fmt.Errorf("invalid image size: %d", sizeOfImage)
	}

	allocAddr, _, _ := procVirtualAlloc.Call(
		0,
		uintptr(sizeOfImage),
		uintptr(memCommit|memReserve),
		uintptr(pageExecuteReadwrite),
	)
	if allocAddr == 0 {
		return 0, fmt.Errorf("VirtualAlloc(%d) failed", sizeOfImage)
	}

	imageBaseDelta = uint64(allocAddr) - imageBase

	procRtlMoveMemory.Call(allocAddr, uintptr(unsafe.Pointer(&dllData[0])), uintptr(sizeOfHeaders))

	for i := uint16(0); i < numberOfSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(&dllData[sectionOffset+uintptr(i)*uintptr(unsafe.Sizeof(imageSectionHeader{}))]))
		if sec.sizeOfRawData == 0 {
			continue
		}
		if sec.pointerToRawData == 0 || sec.pointerToRawData >= uint32(len(dllData)) {
			continue
		}
		if sec.virtualAddress == 0 {
			continue
		}
		dst := allocAddr + uintptr(sec.virtualAddress)

		rawEnd := sec.pointerToRawData + sec.sizeOfRawData
		if rawEnd > uint32(len(dllData)) {
			rawEnd = uint32(len(dllData))
		}
		if rawEnd > sec.pointerToRawData {
			srcData := dllData[sec.pointerToRawData:rawEnd]
			procRtlMoveMemory.Call(dst, uintptr(unsafe.Pointer(&srcData[0])), uintptr(len(srcData)))
		}
	}

	var dataDirAddr uint32
	if is64Bit {
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[0].virtualAddress
	} else {
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[0].virtualAddress
	}

	if dataDirAddr != 0 {
		relocAddr := allocAddr + uintptr(dataDirAddr)
		sizeOfBlock := ^uint32(0)
		for sizeOfBlock > 0 {
			reloc := (*imageBaseRelocation)(unsafe.Pointer(relocAddr))
			if reloc.virtualAddress == 0 {
				break
			}
			sizeOfBlock = reloc.sizeOfBlock
			if sizeOfBlock < 8 {
				break
			}

			numEntries := (sizeOfBlock - 8) / 2
			entriesStart := relocAddr + 8

			for j := uint32(0); j < numEntries; j++ {
				entry := *(*uint16)(unsafe.Pointer(entriesStart + uintptr(j)*2))
				typ := uint8((entry >> 12) & 0x0F)
				offset := uint32(entry & 0x0FFF)

				if typ == relBasedAbs {
					continue
				}

				relocPtr := allocAddr + uintptr(reloc.virtualAddress) + uintptr(offset)

				if is64Bit && typ == relBasedDir64 {
					oldVal := *(*uint64)(unsafe.Pointer(relocPtr))
					newVal := oldVal + imageBaseDelta
					*(*uint64)(unsafe.Pointer(relocPtr)) = newVal
				} else if !is64Bit && typ == relBasedHighlow {
					oldVal := *(*uint32)(unsafe.Pointer(relocPtr))
					newVal := uint32(uint64(oldVal) + imageBaseDelta)
					*(*uint32)(unsafe.Pointer(relocPtr)) = newVal
				}
			}
			relocAddr = relocAddr + uintptr(sizeOfBlock)
		}
	}

	if is64Bit {
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		for i := 0; i < int(oh.numberOfRvaAndSizes) && i < 16; i++ {
			dd := oh.dataDirectory[i]
			if dd.virtualAddress == 0 {
				continue
			}
			if i == 1 {
				resolveImports(allocAddr, dd)
			}
		}
	} else {
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		for i := 0; i < int(oh.numberOfRvaAndSizes) && i < 16; i++ {
			dd := oh.dataDirectory[i]
			if dd.virtualAddress == 0 {
				continue
			}
			if i == 1 {
				resolveImports(allocAddr, dd)
			}
		}
	}

	protectMemory(allocAddr, sizeOfHeaders, pageReadwrite)
	setExecutableSections(allocAddr, dllData[ntOffset+24+uintptr(nt32.fileHeader.sizeOfOptionalHeader):], numberOfSections, is64Bit)

	if !is64Bit {
		hProcess, _, _ := procGetCurrentProcess.Call()
		procFlushInstructionCache.Call(hProcess, allocAddr, uintptr(sizeOfImage))
	}

	if entryPoint != 0 {
		entryAddr := allocAddr + uintptr(entryPoint)

		procRtlMoveMemory.Call(allocAddr+uintptr(sizeOfHeaders-8), uintptr(unsafe.Pointer(&imageBaseDelta)), 8)

		dllMain := func(hinstDLL, fdwReason, lpvReserved uintptr) uintptr {
			ret, _, _ := syscall.SyscallN(entryAddr, hinstDLL, fdwReason, lpvReserved)
			return ret
		}

		go func() {
			thread, _, _ := procCreateThread.Call(
				0,
				0,
				syscall.NewCallback(func(hinstDLL, fdwReason, lpvReserved uintptr) uintptr {
					ret, _, _ := syscall.SyscallN(entryAddr, hinstDLL, fdwReason, lpvReserved)
					return ret
				}),
				allocAddr,
				uintptr(dllProcessAttach),
				0,
			)
			if thread != 0 {
				procWaitForSingleObject.Call(thread, 5000)
				procCloseHandle.Call(thread)
			}
		}()

		_ = dllMain
	}

	return allocAddr, nil
}

func resolveImports(imageBase uintptr, dd imageDataDirectory) {
	importStart := imageBase + uintptr(dd.virtualAddress)
	idx := uint32(0)
	for {
		impDesc := (*imageImportDescriptor)(unsafe.Pointer(importStart + uintptr(idx)*uintptr(unsafe.Sizeof(imageImportDescriptor{}))))
		if impDesc.name == 0 {
			break
		}
		if impDesc.originalFirstThunk == 0 && impDesc.firstThunk == 0 {
			idx++
			continue
		}

		dllName := goStringFromPtr(imageBase + uintptr(impDesc.name))
		if dllName == "" {
			idx++
			continue
		}

		dllHandle, _, _ := procLoadLibraryA.Call(uintptr(unsafe.Pointer(unsafe.StringData(dllName))))
		if dllHandle == 0 {
			idx++
			continue
		}

		thunk := impDesc.originalFirstThunk
		if thunk == 0 {
			thunk = impDesc.firstThunk
		}
		thunkAddr := imageBase + uintptr(thunk)

		firstThunkAddr := imageBase + uintptr(impDesc.firstThunk)

		i := uint32(0)
		for {
			thunkVal := *(*uint64)(unsafe.Pointer(thunkAddr + uintptr(i)*8))
			if thunkVal == 0 {
				break
			}

			var funcAddr uintptr
			if thunkVal&uint64(imageSnpByOrdinal) != 0 {
				ordinal := thunkVal & uint64(imageOrdFlag32)
				funcAddr, _, _ = procGetProcAddress.Call(dllHandle, uintptr(ordinal))
			} else {
				hintNameAddr := imageBase + uintptr(uint32(thunkVal))
				nameOff := hintNameAddr + 2
				funcName := goStringFromPtr(nameOff)
				if funcName != "" {
					funcAddr, _, _ = procGetProcAddress.Call(dllHandle, uintptr(unsafe.Pointer(unsafe.StringData(funcName))))
				}
			}

			if funcAddr != 0 {
				*(*uint64)(unsafe.Pointer(firstThunkAddr + uintptr(i)*8)) = uint64(funcAddr)
			}
			i++
		}
		idx++
	}
}

func goStringFromPtr(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}
	var buf []byte
	for i := 0; ; i++ {
		b := *(*byte)(unsafe.Pointer(ptr + uintptr(i)))
		if b == 0 {
			break
		}
		buf = append(buf, b)
	}
	return string(buf)
}

func protectMemory(addr uintptr, size uint32, prot uint32) {
	var oldProtect uint32
	procVirtualProtect.Call(addr, uintptr(size), uintptr(prot), uintptr(unsafe.Pointer(&oldProtect)))
}

func setExecutableSections(imageBase uintptr, sectionData []byte, numSections uint16, is64Bit bool) {
	for i := uint16(0); i < numSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(&sectionData[uintptr(i)*uintptr(unsafe.Sizeof(imageSectionHeader{}))]))
		if sec.virtualAddress == 0 || sec.virtualSize == 0 {
			continue
		}

		prot := pageReadwrite
		if sec.characteristics&imageScnMemExecute != 0 {
			if sec.characteristics&imageScnMemWrite != 0 {
				prot = pageExecuteReadwrite
			} else {
				prot = pageExecuteRead
			}
		} else if sec.characteristics&imageScnMemRead != 0 {
			if sec.characteristics&imageScnMemWrite != 0 {
				prot = pageReadwrite
			}
		}

		if sec.characteristics&imageScnMemDiscardable != 0 {
			continue
		}

		protectMemory(imageBase+uintptr(sec.virtualAddress), sec.virtualSize, prot)
	}
}

// loadDLLReflectively loads a DLL from memory into the current process.
// Returns the base address (HMODULE) of the loaded DLL.
func loadDLLReflectively(data []byte) (uintptr, error) {
	return reflectDLL(data)
}
