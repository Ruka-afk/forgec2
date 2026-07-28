//go:build windows && arm64
// +build windows,arm64

package main

import "fmt"

func syscallNtCreateThreadExParam(mgr *syscallManager, hProc uintptr, startAddr uintptr, param uintptr) (uintptr, error) {
	hThread, _, _ := arm64CreateRemoteThread.Call(hProc, 0, 0, startAddr, param, 0, 0)
	if hThread == 0 {
		return 0, fmt.Errorf("CreateRemoteThread failed")
	}
	return hThread, nil
}
