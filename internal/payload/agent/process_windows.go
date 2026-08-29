//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func getPlatformSecurityInfo() (string, bool, string) {
	integrity := "Medium"
	elevated := false

	var hToken uintptr
	currentProc, _ := syscall.GetCurrentProcess()
	ret, _, _ := procOpenProcessToken.Call(
		uintptr(currentProc),
		0x0008,
		uintptr(unsafe.Pointer(&hToken)),
	)
	if ret != 0 && hToken != 0 {
		defer syscall.CloseHandle(syscall.Handle(hToken))

		var needed uint32
		procGetTokenInformation.Call(hToken, 25, 0, 0, uintptr(unsafe.Pointer(&needed)))
		if needed > 0 {
			buf := make([]byte, needed)
			ret2, _, _ := procGetTokenInformation.Call(
				hToken, 25,
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(needed),
				uintptr(unsafe.Pointer(&needed)),
			)
			// Parse TOKEN_INFORMATION_CLASS 25 (TokenIntegrityLevel) buffer:
			// first 8 bytes = pointer to SID, followed by SID sub-authority count and RIDs
			if ret2 != 0 && len(buf) >= 8 {
				// Read SID pointer from token info buffer, then extract sub-authority count
				sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
				if sidPtr != 0 {
					// SID offset 1: SubAuthorityCount; offset 8+: SubAuthority array
					subCount := *(*uint8)(unsafe.Pointer(sidPtr + 1))
					if subCount > 0 {
						// Read last SubAuthority (RID) from SID to determine integrity level
						rid := *(*uint32)(unsafe.Pointer(sidPtr + 8 + uintptr(subCount-1)*4))
						switch {
						case rid >= 16384:
							integrity = "System"
							elevated = true
						case rid >= 12288:
							integrity = "High"
							elevated = true
						case rid >= 8192:
							integrity = "Medium"
						case rid >= 4096:
							integrity = "Low"
						default:
							integrity = "Untrusted"
						}
					}
				}
			}
		}
	}

	domain := os.Getenv("USERDOMAIN")
	if domain == "" {
		domain, _ = os.Hostname()
	}
	return integrity, elevated, domain
}

func listProcessesForTree() ([]procNode, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var pe processEntry32
	pe.dwSize = uint32(unsafe.Sizeof(pe))
	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed")
	}
	var nodes []procNode
	for ret != 0 {
		name := syscall.UTF16ToString(pe.szExeFile[:])
		nodes = append(nodes, procNode{
			PID:  int(pe.th32ProcessID),
			PPID: int(pe.th32ParentProcessID),
			Name: name,
		})
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return nodes, nil
}

func findPIDByName(name string) (uint32, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var pe processEntry32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		exe := syscall.UTF16ToString(pe.szExeFile[:])
		if strings.EqualFold(exe, name) || strings.EqualFold(filepath.Base(exe), name) {
			return pe.th32ProcessID, nil
		}
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return 0, fmt.Errorf("process not found: %s", name)
}

func suspendProcessWindows(target string) (string, error) {
	var pid uint32
	if p, err := strconv.ParseUint(target, 10, 32); err == nil {
		pid = uint32(p)
	} else {
		p, err := findPIDByName(target)
		if err != nil {
			return "", err
		}
		pid = p
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return "", fmt.Errorf("snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))

	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	count := 0
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			h, _, _ := procOpenThread.Call(THREAD_SUSPEND_RESUME, 0, uintptr(te.th32ThreadID))
			if h != 0 {
				procSuspendThread.Call(h)
				procCloseHandle.Call(h)
				count++
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Sprintf("suspended %d threads (pid=%d)", count, pid), nil
}

func resumeProcessWindows(target string) (string, error) {
	var pid uint32
	if p, err := strconv.ParseUint(target, 10, 32); err == nil {
		pid = uint32(p)
	} else {
		p, err := findPIDByName(target)
		if err != nil {
			return "", err
		}
		pid = p
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return "", fmt.Errorf("snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))

	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	count := 0
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			h, _, _ := procOpenThread.Call(THREAD_SUSPEND_RESUME, 0, uintptr(te.th32ThreadID))
			if h != 0 {
				procResumeThread.Call(h)
				procCloseHandle.Call(h)
				count++
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Sprintf("resumed %d threads (pid=%d)", count, pid), nil
}

func killProcessWindows(target string) (string, error) {
	var pid uint32
	if p, err := strconv.ParseUint(target, 10, 32); err == nil {
		pid = uint32(p)
	} else {
		p, err := findPIDByName(target)
		if err != nil {
			return "", err
		}
		pid = p
	}

	h, _, _ := procOpenProcess.Call(PROCESS_TERMINATE, 0, uintptr(pid))
	if h == 0 {
		return "", fmt.Errorf("open process failed")
	}
	defer procCloseHandle.Call(h)

	ret, _, _ := procTerminateProcess.Call(h, 1)
	if ret == 0 {
		return "", fmt.Errorf("terminate failed")
	}
	return fmt.Sprintf("killed pid %d", pid), nil
}
