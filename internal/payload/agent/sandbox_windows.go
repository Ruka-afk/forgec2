//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// checkMemorySize checks if memory is too low on Windows using GlobalMemoryStatusEx.
func (sd *SandboxDetector) checkMemorySize() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx := k32.NewProc("GlobalMemoryStatusEx")
	var memInfo struct {
		length        uint32
		memoryLoad    uint32
		totalPhys     uint64
		availPhys     uint64
		totalPageFile uint64
		availPageFile uint64
		totalVirtual  uint64
		availVirtual  uint64
		reserved      [8]uint64
	}
	memInfo.length = uint32(unsafe.Sizeof(memInfo))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if ret == 0 {
		return false
	}
	return memInfo.totalPhys < 4*1024*1024*1024
}

// checkDiskSize checks if disk is too small using GetDiskFreeSpaceExW.
func (sd *SandboxDetector) checkDiskSize() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx := k32.NewProc("GetDiskFreeSpaceExW")
	root, _ := syscall.UTF16PtrFromString("C:\\")
	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	ret, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return false
	}
	return totalBytes < 60*1024*1024*1024
}

// checkVMProcesses checks for VM-related processes
func (sd *SandboxDetector) checkVMProcesses() bool {
	vmProcesses := []string{
		"vboxservice.exe",
		"vmtoolsd.exe",
		"vmwaretray.exe",
		"vmacthlp.exe",
		"VGAuthService.exe",
	}

	// Run tasklist
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}

	output := strings.ToLower(string(out))
	for _, proc := range vmProcesses {
		if strings.Contains(output, strings.ToLower(proc)) {
			return true
		}
	}

	return false
}

// checkVMMAC retrieves MAC address via GetAdaptersInfo and checks against VM prefixes.
func (sd *SandboxDetector) checkVMMAC() bool {
	vmMACPrefixes := []string{
		"00:05:69", // VMware
		"00:0C:29", // VMware
		"00:1C:14", // VMware
		"00:50:56", // VMware
		"08:00:27", // VirtualBox
		"0A:00:27", // VirtualBox
		"00:1C:42", // Parallels
		"00:16:3E", // Xen
		"00:15:5D", // Hyper-V
	}
	iphlpapi := syscall.NewLazyDLL("iphlpapi.dll")
	procGetAdaptersInfo := iphlpapi.NewProc("GetAdaptersInfo")
	var bufSize uint32
	procGetAdaptersInfo.Call(0, uintptr(unsafe.Pointer(&bufSize)))
	if bufSize == 0 {
		return false
	}
	buf := make([]byte, bufSize)
	ret, _, _ := procGetAdaptersInfo.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufSize)))
	if ret != 0 {
		return false
	}
	type ipAddressList struct {
		next      uintptr
		ipAddress [16]byte
		ipMask    [16]byte
		context   uint32
	}
	type adapterInfo struct {
		next                uintptr
		comboIndex          uint32
		name                [260 + 4]byte
		description         [132 + 4]byte
		addressLength       uint32
		address             [8]byte
		index               uint32
		_type               uint32
		dhcpEnabled         uint32
		currentIpAddress    uintptr
		ipAddressList       ipAddressList
		gatewayList         ipAddressList
		dhcpServer          ipAddressList
		haveWins            bool
		primaryWinsServer   ipAddressList
		secondaryWinsServer ipAddressList
		leaseObtained       int64
		leaseExpires        int64
	}
	ai := (*adapterInfo)(unsafe.Pointer(&buf[0]))
	for ai != nil {
		mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			ai.address[0], ai.address[1], ai.address[2],
			ai.address[3], ai.address[4], ai.address[5])
		for _, prefix := range vmMACPrefixes {
			if strings.HasPrefix(strings.ToUpper(mac), prefix) {
				return true
			}
		}
		if ai.next != 0 {
			ai = (*adapterInfo)(unsafe.Pointer(ai.next))
		} else {
			break
		}
	}
	return false
}

// checkMouseMovement checks for lack of mouse movement via GetCursorPos.
func (sd *SandboxDetector) checkMouseMovement() bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	procGetCursorPos := user32.NewProc("GetCursorPos")
	// Sample cursor position twice with a small delay
	var pos1 struct{ x, y int32 }
	ret1, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pos1)))
	time.Sleep(50 * time.Millisecond)
	var pos2 struct{ x, y int32 }
	ret2, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pos2)))
	if ret1 == 0 || ret2 == 0 {
		return false
	}
	// If cursor hasn't moved = sandbox indicator
	return pos1.x == pos2.x && pos1.y == pos2.y
}

// IsDebuggerPresent checks if a debugger is attached using NtQueryInformationProcess.
func (ad *AntiDebug) IsDebuggerPresent() bool {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInformationProcess := ntdll.NewProc("NtQueryInformationProcess")
	// ProcessDebugPort = 7
	const processDebugPort = 7
	var debugPort uintptr
	var retLen uint32
	ret, _, _ := procNtQueryInformationProcess.Call(
		^uintptr(0), // NtCurrentProcess = -1
		processDebugPort,
		uintptr(unsafe.Pointer(&debugPort)),
		unsafe.Sizeof(debugPort),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if ret != 0 {
		return false
	}
	return debugPort != 0
}

// CheckRemoteDebuggerPresent checks for remote debugger.
func (ad *AntiDebug) CheckRemoteDebuggerPresent() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procCheckRemoteDebuggerPresent := k32.NewProc("CheckRemoteDebuggerPresent")
	var isDebugger bool
	ret, _, _ := procCheckRemoteDebuggerPresent.Call(
		^uintptr(0), // GetCurrentProcess = -1
		uintptr(unsafe.Pointer(&isDebugger)),
	)
	if ret == 0 {
		return false
	}
	return isDebugger
}

// checkVMMACEnhanced checks for VM MAC addresses with expanded vendor list.
func (sd *SandboxDetector) checkVMMACEnhanced() bool {
	vmMACPrefixes := []string{
		"00:05:69", // VMware
		"00:0C:29", // VMware
		"00:1C:14", // VMware
		"00:50:56", // VMware
		"00:50:56", // VMware
		"08:00:27", // VirtualBox
		"0A:00:27", // VirtualBox
		"00:1C:42", // Parallels
		"00:16:3E", // Xen
		"00:15:5D", // Hyper-V
		"00:03:FF", // Microsoft Hyper-V
		"00:1B:4D", // VMware
		"00:0F:4B", // VMware
		"00:21:F6", // VMware
		"3C:D9:2B", // Hyper-V (older)
		"00:25:90", // Hyper-V
	}
	iphlpapi := syscall.NewLazyDLL("iphlpapi.dll")
	procGetAdaptersInfo := iphlpapi.NewProc("GetAdaptersInfo")
	var bufSize uint32
	procGetAdaptersInfo.Call(0, uintptr(unsafe.Pointer(&bufSize)))
	if bufSize == 0 {
		return false
	}
	buf := make([]byte, bufSize)
	ret, _, _ := procGetAdaptersInfo.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufSize)))
	if ret != 0 {
		return false
	}
	type ipAddressList struct {
		next      uintptr
		ipAddress [16]byte
		ipMask    [16]byte
		context   uint32
	}
	type adapterInfo struct {
		next                uintptr
		comboIndex          uint32
		name                [260 + 4]byte
		description         [132 + 4]byte
		addressLength       uint32
		address             [8]byte
		index               uint32
		_type               uint32
		dhcpEnabled         uint32
		currentIpAddress    uintptr
		ipAddressList       ipAddressList
		gatewayList         ipAddressList
		dhcpServer          ipAddressList
		haveWins            bool
		primaryWinsServer   ipAddressList
		secondaryWinsServer ipAddressList
		leaseObtained       int64
		leaseExpires        int64
	}
	ai := (*adapterInfo)(unsafe.Pointer(&buf[0]))
	for ai != nil {
		mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			ai.address[0], ai.address[1], ai.address[2],
			ai.address[3], ai.address[4], ai.address[5])
		for _, prefix := range vmMACPrefixes {
			if strings.HasPrefix(strings.ToUpper(mac), prefix) {
				return true
			}
		}
		if ai.next != 0 {
			ai = (*adapterInfo)(unsafe.Pointer(ai.next))
		} else {
			break
		}
	}
	return false
}

// checkRDTSC detects debuggers via RDTSC/RDTSCP instruction timing variance.
func (sd *SandboxDetector) checkRDTSC() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTime := k32.NewProc("GetSystemTimeAsFileTime")
	var t1, t2, t3 uint64
	procGetSystemTime.Call(uintptr(unsafe.Pointer(&t1)))
	// Perform minimal work
	var sum uint64
	for i := 0; i < 100; i++ {
		sum += uint64(i)
	}
	procGetSystemTime.Call(uintptr(unsafe.Pointer(&t2)))
	for i := 0; i < 100; i++ {
		sum += uint64(i)
	}
	procGetSystemTime.Call(uintptr(unsafe.Pointer(&t3)))
	_ = sum
	// First delta should be similar to second if no debugger
	delta1 := t2 - t1
	delta2 := t3 - t2
	// Debuggers cause wildly inconsistent timing; if variance > 5x, suspect
	if delta1 == 0 || delta2 == 0 {
		return false
	}
	if delta1 > delta2*5 || delta2 > delta1*5 {
		return true
	}
	return false
}

// checkHumanPresence checks if active window title changes (human interaction).
func (sd *SandboxDetector) checkHumanPresence() bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow := user32.NewProc("GetForegroundWindow")
	procGetWindowTextW := user32.NewProc("GetWindowTextW")
	sampleWindow := func() string {
		hwnd, _, _ := procGetForegroundWindow.Call()
		if hwnd == 0 {
			return ""
		}
		buf := make([]uint16, 256)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		return syscall.UTF16ToString(buf)
	}
	// Take three samples with delays to check for changes
	title1 := sampleWindow()
	time.Sleep(500 * time.Millisecond)
	title2 := sampleWindow()
	time.Sleep(500 * time.Millisecond)
	title3 := sampleWindow()
	// If any title changed, human is actively using the system
	if title1 != title2 || title2 != title3 {
		return false // human present = NOT sandbox
	}
	// A stable foreground window title is normal human behavior (reading,
	// watching video, momentarily idle). The only weak signal here is the
	// complete absence of any foreground window, which suggests automation.
	return title1 == "" // no foreground window at all -> suspect sandbox
}

// checkHardwareBreakpoints detects hardware breakpoints via DR register check.
func (sd *SandboxDetector) checkHardwareBreakpoints() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThread := k32.NewProc("GetCurrentThread")
	procGetThreadContext := k32.NewProc("GetThreadContext")
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInformationThread := ntdll.NewProc("NtQueryInformationThread")

	// CONTEXT_DEBUG_REGISTERS = 0x00000010 for x64
	const contextDebugRegisters = 0x00000010

	type context struct {
		p1Home   uint64
		p2Home   uint64
		p3Home   uint64
		p4Home   uint64
		p5Home   uint64
		p6Home   uint64
		ctxFlags uint32
		mxCsr    uint32
		segCs    uint16
		segDs    uint16
		segEs    uint16
		segFs    uint16
		segGs    uint16
		segSs    uint16
		eFlags   uint32
		dr0      uint64
		dr1      uint64
		dr2      uint64
		dr3      uint64
		dr6      uint64
		dr7      uint64
	}

	ctx := &context{}
	ctx.ctxFlags = contextDebugRegisters
	hThread, _, _ := procGetCurrentThread.Call()

	// Suspend the thread to safely read context, then resume
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ret, _, _ := procGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(ctx)))
	if ret == 0 {
		return false
	}

	// If any DR0-DR3 are non-zero, breakpoints are set
	if ctx.dr0 != 0 || ctx.dr1 != 0 || ctx.dr2 != 0 || ctx.dr3 != 0 || ctx.dr7 != 0 {
		return true
	}

	// Also check via NtQueryInformationThread for ThreadHideFromDebugger (0x11)
	const threadHideFromDebugger = 0x11
	var hideFromDebugger byte
	retNt, _, _ := procNtQueryInformationThread.Call(
		hThread,
		threadHideFromDebugger,
		uintptr(unsafe.Pointer(&hideFromDebugger)),
		unsafe.Sizeof(hideFromDebugger),
		0,
	)
	if retNt == 0 && hideFromDebugger != 0 {
		return true
	}

	return false
}

// checkDiskSizeSmall checks if disk < 100GB (VM indicator).
func (sd *SandboxDetector) checkDiskSizeSmall() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx := k32.NewProc("GetDiskFreeSpaceExW")
	root, _ := syscall.UTF16PtrFromString("C:\\")
	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	ret, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return false
	}
	return totalBytes < 100*1024*1024*1024
}

// checkRAMSizeSmall checks if RAM < 4GB (VM indicator).
func (sd *SandboxDetector) checkRAMSizeSmall() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatusEx := k32.NewProc("GlobalMemoryStatusEx")
	var memInfo struct {
		length        uint32
		memoryLoad    uint32
		totalPhys     uint64
		availPhys     uint64
		totalPageFile uint64
		availPageFile uint64
		totalVirtual  uint64
		availVirtual  uint64
		reserved      [8]uint64
	}
	memInfo.length = uint32(unsafe.Sizeof(memInfo))
	ret, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if ret == 0 {
		return false
	}
	return memInfo.totalPhys < 4*1024*1024*1024
}
