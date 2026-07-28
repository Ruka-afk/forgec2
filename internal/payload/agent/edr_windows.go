//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	procAdvapiOpenSCManagerW      *syscall.LazyProc
	procAdvapiEnumServicesStatusW *syscall.LazyProc
	procAdvapiCloseServiceHandle  *syscall.LazyProc
	advapiOnce                    bool
)

func initEDRProcs() {
	if advapiOnce {
		return
	}
	advapiOnce = true
	adv := syscall.NewLazyDLL("advapi32.dll")
	procAdvapiOpenSCManagerW = adv.NewProc("OpenSCManagerW")
	procAdvapiEnumServicesStatusW = adv.NewProc("EnumServicesStatusW")
	procAdvapiCloseServiceHandle = adv.NewProc("CloseServiceHandle")
}

var edrProcessSignatures = map[string][]string{
	EDRCrowdStrike: {"csfalconservice.exe", "csfalconcontainer.exe"},
	EDRDefender:    {"msmpeng.exe", "nissrv.exe"},
	EDRCarbonBlack: {"repmgr.exe", "reputils.exe"},
	EDRSentinelOne: {"sentinelagent.exe", "sentinelstaticengine.exe"},
	EDRCylance:     {"cylancesvc.exe", "cylanceui.exe"},
	EDRBitDefender: {"bdagent.exe", "vsserv.exe"},
	EDRSymantec:    {"symantecsysplant.exe", "rtvscan.exe"},
	EDRTrendMicro:  {"tmcc.exe", "tmlisten.exe"},
}

var edrServiceSignatures = map[string][]string{
	EDRCrowdStrike: {"csagent", "csfalcon"},
	EDRDefender:    {"windefend", "msmpeng"},
	EDRCarbonBlack: {"carbonblack", "cb"},
	EDRSentinelOne: {"sentinel", "sentinellogger"},
	EDRCylance:     {"cylance"},
	EDRBitDefender: {"bitdefender", "bdredline"},
	EDRSymantec:    {"symantec", "sep"},
	EDRTrendMicro:  {"trendmicro", "tmcc"},
}

func DetectEDR() *EDRInfo {
	info := &EDRInfo{
		Processes: make([]string, 0),
		Services:  make([]string, 0),
		Drivers:   make([]string, 0),
	}

	info.detectProcesses()
	info.detectServices()
	info.identifyEDR()

	return info
}

func (e *EDRInfo) detectProcesses() {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 {
		return
	}
	defer procCloseHandle.Call(snap)

	var pe processEntry32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		name := syscall.UTF16ToString(pe.szExeFile[:])
		if name != "" {
			e.Processes = append(e.Processes, name)
		}
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
}

func (e *EDRInfo) detectServices() {
	initEDRProcs()

	machineName, _ := syscall.UTF16PtrFromString("")
	databaseName, _ := syscall.UTF16PtrFromString("ServicesActive")

	const scManagerEnumerateService = 0x0004

	scmHandle, _, _ := procAdvapiOpenSCManagerW.Call(
		uintptr(unsafe.Pointer(machineName)),
		uintptr(unsafe.Pointer(databaseName)),
		uintptr(scManagerEnumerateService),
	)
	if scmHandle == 0 {
		return
	}
	defer procAdvapiCloseServiceHandle.Call(scmHandle)

	const (
		serviceWin32    = 0x00000030
		serviceDriver   = 0x0000000A
		serviceStateAll = 0x00000003
	)

	var bytesNeeded uint32
	var servicesReturned uint32
	var resumeHandle uint32

	procAdvapiEnumServicesStatusW.Call(
		scmHandle,
		uintptr(serviceWin32|serviceDriver),
		uintptr(serviceStateAll),
		0,
		0,
		uintptr(unsafe.Pointer(&bytesNeeded)),
		uintptr(unsafe.Pointer(&servicesReturned)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)

	if bytesNeeded == 0 {
		return
	}

	buf := make([]byte, bytesNeeded+4096)

	ret, _, _ := procAdvapiEnumServicesStatusW.Call(
		scmHandle,
		uintptr(serviceWin32|serviceDriver),
		uintptr(serviceStateAll),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&bytesNeeded)),
		uintptr(unsafe.Pointer(&servicesReturned)),
		uintptr(unsafe.Pointer(&resumeHandle)),
	)
	if ret == 0 {
		return
	}

	type enumServiceStatusW struct {
		serviceName             uintptr
		displayName             uintptr
		serviceType             uint32
		currentState            uint32
		controlsAccepted        uint32
		win32ExitCode           uint32
		serviceSpecificExitCode uint32
		checkPoint              uint32
		waitHint                uint32
	}

	ess := (*[1 << 16]enumServiceStatusW)(unsafe.Pointer(&buf[0]))
	for i := uint32(0); i < servicesReturned; i++ {
		if ess[i].serviceName != 0 {
			var nameChars []uint16
			p := unsafe.Pointer(ess[i].serviceName)
			for j := 0; ; j++ {
				c := *(*uint16)(unsafe.Pointer(uintptr(p) + uintptr(j)*2))
				if c == 0 {
					break
				}
				nameChars = append(nameChars, c)
			}
			name := syscall.UTF16ToString(nameChars)
			if name != "" {
				e.Services = append(e.Services, name)
			}
		}
	}
}

func (e *EDRInfo) identifyEDR() {
	for edrName, procs := range edrProcessSignatures {
		for _, proc := range e.Processes {
			lower := strings.ToLower(proc)
			for _, sig := range procs {
				if strings.EqualFold(lower, sig) {
					e.Detected = true
					e.Name = edrName
					return
				}
			}
		}
	}

	for edrName, svcs := range edrServiceSignatures {
		for _, svc := range e.Services {
			lower := strings.ToLower(svc)
			for _, sig := range svcs {
				if strings.Contains(lower, sig) {
					e.Detected = true
					e.Name = edrName
					return
				}
			}
		}
	}
}
