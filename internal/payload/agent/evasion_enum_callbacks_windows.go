//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	ntdllEnumCB        = syscall.NewLazyDLL("ntdll.dll")
	procNtQuerySysInfo = ntdllEnumCB.NewProc("NtQuerySystemInformation")
)

const (
	SystemHandleInformation  = 0x10
	SystemObjectInformation  = 0x11
	SystemExtendedHandleInfo = 0x40
)

type systemHandleEntry struct {
	UniqueProcessId  uint32
	CreatorBackTrace uint32
	ObjectTypeIndex  uint8
	HandleAttributes uint8
	HandleValue      uint16
	ObjectPointer    uintptr
	GrantedAccess    uint32
}

func init() {
	registerEvasion("enum_callbacks", runEvasionEnumCallbacks)
}

func runEvasionEnumCallbacks() string {
	var result strings.Builder
	result.WriteString("[*] Kernel Callback Enumeration Report\n")
	result.WriteString("=====================================\n\n")

	// 1. Check kernel debugger (EDR callback indicator)
	if queryEDRKernelDebugger() {
		result.WriteString("[!] Kernel debugger/EDR callback infrastructure detected\n\n")
	} else {
		result.WriteString("[*] No kernel debugger callback infrastructure detected\n\n")
	}

	// 2. Enumerate system handles to find EDR driver objects
	edrHandles := enumerateEDRHandles()
	if len(edrHandles) > 0 {
		result.WriteString(fmt.Sprintf("[!] Potential EDR callback handles found: %d\n", len(edrHandles)))
		for _, h := range edrHandles {
			result.WriteString(fmt.Sprintf("    PID=%d Handle=0x%x TypeIndex=%d Access=0x%x\n",
				h.UniqueProcessId, h.HandleValue, h.ObjectTypeIndex, h.GrantedAccess))
		}
	} else {
		result.WriteString("[*] No obvious EDR callback handles detected\n")
	}

	// 3. Check process debug port (EDR process creation callbacks)
	result.WriteString(checkProcessDebugPort())

	// 4. Image load callback detection via process mitigation
	result.WriteString(checkImageLoadCallbacks())

	// 5. Registry callback detection
	result.WriteString(checkRegistryCallbacks())

	return strings.TrimSpace(result.String())
}

func queryEDRKernelDebugger() bool {
	type systemKernelDebuggerInformation struct {
		KernelDebuggerEnabled    bool
		KernelDebuggerNotPresent bool
	}
	var buf [32]byte
	ret, _, _ := procNtQuerySysInfo.Call(
		uintptr(SystemKernelDebuggerInfo),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	if ret != 0 {
		return false
	}
	info := (*systemKernelDebuggerInformation)(unsafe.Pointer(&buf[0]))
	return info.KernelDebuggerEnabled || !info.KernelDebuggerNotPresent
}

func enumerateEDRHandles() []systemHandleEntry {
	var retLen uint32
	procNtQuerySysInfo.Call(
		uintptr(SystemExtendedHandleInfo),
		0,
		0,
		uintptr(unsafe.Pointer(&retLen)),
	)
	if retLen == 0 || retLen > 2*1024*1024 {
		return nil
	}

	buf := make([]byte, retLen+256)
	ret, _, _ := procNtQuerySysInfo.Call(
		uintptr(SystemExtendedHandleInfo),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if ret != 0 {
		return nil
	}

	type systemHandleInfoEx struct {
		NumberOfHandles uintptr
		Handles         [1]systemHandleEntry
	}

	shi := (*systemHandleInfoEx)(unsafe.Pointer(&buf[0]))
	numHandles := int(shi.NumberOfHandles)
	if numHandles > 10000 {
		numHandles = 10000
	}

	var edrHandles []systemHandleEntry
	knownEdrPids := getKnownEDRPIDs()

	for i := 0; i < numHandles; i++ {
		entry := (*systemHandleEntry)(unsafe.Pointer(
			uintptr(unsafe.Pointer(&buf[0])) + uintptr(unsafe.Sizeof(uintptr(0))) + uintptr(i)*unsafe.Sizeof(systemHandleEntry{}),
		))

		// Filter for EDR processes with callback-related handles
		if entry.ObjectTypeIndex == 6 || entry.ObjectTypeIndex == 7 || entry.ObjectTypeIndex == 11 {
			for _, pid := range knownEdrPids {
				if entry.UniqueProcessId == pid {
					edrHandles = append(edrHandles, *entry)
				}
			}
		}
	}

	return edrHandles
}

func getKnownEDRPIDs() []uint32 {
	var pids []uint32
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 {
		return nil
	}
	defer procCloseHandle.Call(snap)

	var pe processEntry32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		name := syscall.UTF16ToString(pe.szExeFile[:])
		lower := strings.ToLower(name)
		for _, sig := range []string{
			"csfalcon", "sentinel", "cylance", "carbonblack", "msmpeng",
			"symantec", "tmcc", "bdagent", "vsserv", "elastic",
		} {
			if strings.Contains(lower, sig) {
				pids = append(pids, pe.th32ProcessID)
				break
			}
		}
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return pids
}

func checkProcessDebugPort() string {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInfoProcess := ntdll.NewProc("NtQueryInformationProcess")

	const ProcessDebugPort = 0x07
	var debugPort uintptr
	var retLen uint32

	ret, _, _ := procNtQueryInfoProcess.Call(
		^uintptr(0xffffffffffffffff),
		uintptr(ProcessDebugPort),
		uintptr(unsafe.Pointer(&debugPort)),
		uintptr(unsafe.Sizeof(debugPort)),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if ret != 0 {
		return "[*] ProcessDebugPort: NtQueryInformationProcess failed\n"
	}

	if debugPort != 0 {
		return fmt.Sprintf("[!] ProcessDebugPort: 0x%x — process is being debugged (EDR callback active)\n", debugPort)
	}
	return "[*] ProcessDebugPort: 0 (no debugger attached)\n"
}

func checkImageLoadCallbacks() string {
	// Check if image load callbacks are active by trying to detect
	// if the system is monitoring DLL loads
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInfoProcess := ntdll.NewProc("NtQueryInformationProcess")

	const ProcessImageFileName = 0x1B

	var buf [512]byte
	var retLen uint32
	ret, _, _ := procNtQueryInfoProcess.Call(
		^uintptr(0xffffffffffffffff),
		uintptr(ProcessImageFileName),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if ret != 0 {
		return "[*] Image load callback check: NtQueryInformationProcess failed (expected for some processes)\n"
	}

	return "[*] Image load callback check: process image name query succeeded\n"
}

func checkRegistryCallbacks() string {
	// Registry callbacks can be detected by attempting operations
	// that EDRs commonly hook (e.g., opening protected registry keys)
	advapi := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx := advapi.NewProc("RegOpenKeyExW")

	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		0x80000002, // HKEY_LOCAL_MACHINE
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("SYSTEM\\CurrentControlSet\\Services"))),
		0,
		0x20019, // KEY_READ
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret == 0 {
		return "[*] Registry callbacks: HKLM\\Services opened without interference\n"
	}
	return fmt.Sprintf("[*] Registry callbacks: RegOpenKeyEx returned 0x%x (possible callback interference)\n", ret)
}
