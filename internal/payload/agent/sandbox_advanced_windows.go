//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func getWindowsRAMGB() float64 {
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
		return 0
	}
	return float64(memInfo.totalPhys) / (1024 * 1024 * 1024)
}

func getWindowsDiskGB(drive string) float64 {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx := k32.NewProc("GetDiskFreeSpaceExW")
	root, _ := syscall.UTF16PtrFromString(drive)
	var freeBytesAvailable, totalBytes, totalFreeBytes int64
	ret, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return 0
	}
	return float64(totalBytes) / (1024 * 1024 * 1024)
}

func getWindowsProcessCount() int {
	type processEntry32 struct {
		Size            uint32
		CntUsage        uint32
		ProcessID       uint32
		DefaultHeapID   uintptr
		ModuleID        uint32
		CntThreads      uint32
		ParentProcessID uint32
		PriClassBase    int32
		Flags           uint32
		ExeFile         [260]uint16
	}
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procCreateToolhelp32Snapshot := k32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW := k32.NewProc("Process32FirstW")
	procProcess32NextW := k32.NewProc("Process32NextW")
	procCloseHandle := k32.NewProc("CloseHandle")

	const th32csProcess = 0x00000002
	snapshot, _, _ := procCreateToolhelp32Snapshot.Call(th32csProcess, 0)
	if snapshot == 0 || snapshot == ^uintptr(0) {
		return 0
	}
	defer procCloseHandle.Call(snapshot)

	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	count := 0

	ret, _, _ := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return 0
	}
	count++

	for {
		ret, _, _ = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
		count++
	}
	return count
}

func getVMVendorMACs() []string {
	vmMACPrefixes := []string{
		"00:05:69", "00:0C:29", "00:1C:14", "00:50:56",
		"08:00:27", "0A:00:27", "00:1C:42", "00:16:3E",
		"00:15:5D", "00:03:FF", "00:1B:4D", "00:0F:4B",
		"00:21:F6", "3C:D9:2B", "00:25:90",
	}
	iphlpapi := syscall.NewLazyDLL("iphlpapi.dll")
	procGetAdaptersInfo := iphlpapi.NewProc("GetAdaptersInfo")
	var bufSize uint32
	procGetAdaptersInfo.Call(0, uintptr(unsafe.Pointer(&bufSize)))
	if bufSize == 0 {
		return nil
	}
	buf := make([]byte, bufSize)
	ret, _, _ := procGetAdaptersInfo.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufSize)))
	if ret != 0 {
		return nil
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
	var found []string
	ai := (*adapterInfo)(unsafe.Pointer(&buf[0]))
	for ai != nil {
		mac := fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
			ai.address[0], ai.address[1], ai.address[2],
			ai.address[3], ai.address[4], ai.address[5])
		for _, prefix := range vmMACPrefixes {
			if strings.HasPrefix(strings.ToUpper(mac), prefix) {
				found = append(found, mac)
				break
			}
		}
		if ai.next != 0 {
			ai = (*adapterInfo)(unsafe.Pointer(ai.next))
		} else {
			break
		}
	}
	return found
}

func checkVMRegistryKeys() []string {
	vmKeys := []string{
		`SOFTWARE\VMware, Inc.\VMware Tools`,
		`SOFTWARE\Oracle\VirtualBox Guest Additions`,
		`SYSTEM\CurrentControlSet\Services\vmci`,
		`SYSTEM\CurrentControlSet\Services\vmhgfs`,
		`SYSTEM\CurrentControlSet\Services\VBoxGuest`,
		`SYSTEM\CurrentControlSet\Services\VBoxMouse`,
	}
	var found []string
	for _, keyPath := range vmKeys {
		if checkRegistryKey(keyPath) {
			found = append(found, keyPath)
		}
	}
	return found
}

func checkRegistryKey(path string) bool {
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW := advapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey := advapi32.NewProc("RegCloseKey")

	keyPathPtr, _ := syscall.UTF16PtrFromString(path)
	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		0x80000002, // HKEY_LOCAL_MACHINE
		uintptr(unsafe.Pointer(keyPathPtr)),
		0,
		0x0010, // KEY_READ
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret == 0 && hKey != 0 {
		procRegCloseKey.Call(hKey)
		return true
	}
	return false
}

func getWindowsUptimeMinutes() float64 {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetTickCount64 := k32.NewProc("GetTickCount64")
	ret, _, _ := procGetTickCount64.Call()
	if ret == 0 {
		return 0
	}
	return float64(ret) / 60000.0
}

func getDesktopResolution() (int, int) {
	user32 := syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics := user32.NewProc("GetSystemMetrics")
	const smCxscreen = 0
	const smCyscreen = 1
	w, _, _ := procGetSystemMetrics.Call(smCxscreen)
	h, _, _ := procGetSystemMetrics.Call(smCyscreen)
	return int(w), int(h)
}

func checkMouseMoved() bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	procGetCursorPos := user32.NewProc("GetCursorPos")
	var pos1 struct{ x, y int32 }
	ret1, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pos1)))
	time.Sleep(50 * time.Millisecond)
	var pos2 struct{ x, y int32 }
	ret2, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pos2)))
	if ret1 == 0 || ret2 == 0 {
		return false
	}
	return pos1.x != pos2.x || pos1.y != pos2.y
}

func checkRDTSCVariance() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procQueryPerformanceCounter := k32.NewProc("QueryPerformanceCounter")
	procQueryPerformanceFrequency := k32.NewProc("QueryPerformanceFrequency")
	var freq int64
	procQueryPerformanceFrequency.Call(uintptr(unsafe.Pointer(&freq)))
	if freq == 0 {
		return false
	}
	var t1, t2, t3 int64
	procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&t1)))
	var sum int64
	for i := 0; i < 100; i++ {
		sum += int64(i)
	}
	procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&t2)))
	for i := 0; i < 100; i++ {
		sum += int64(i)
	}
	procQueryPerformanceCounter.Call(uintptr(unsafe.Pointer(&t3)))
	_ = sum
	delta1 := t2 - t1
	delta2 := t3 - t2
	if delta1 == 0 || delta2 == 0 {
		return false
	}
	if delta1 > delta2*5 || delta2 > delta1*5 {
		return true
	}
	return false
}

func checkDRRegisters() bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	procGetCurrentThread := k32.NewProc("GetCurrentThread")
	procGetThreadContext := k32.NewProc("GetThreadContext")

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
	ret, _, _ := procGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(ctx)))
	if ret == 0 {
		return false
	}
	return ctx.dr0 != 0 || ctx.dr1 != 0 || ctx.dr2 != 0 || ctx.dr3 != 0 || ctx.dr7 != 0
}
