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
	EDRDefender:    {"msmpeng.exe", "nissrv.exe", "mssense.exe", "sensecncproxy.exe", "smartscreen.exe"},
	EDRCarbonBlack: {"repmgr.exe", "reputils.exe", "cb.exe", "cbstream.exe"},
	EDRSentinelOne: {"sentinelagent.exe", "sentinelstaticengine.exe", "sentinelhelper.exe"},
	EDRCylance:     {"cylancesvc.exe", "cylanceui.exe"},
	EDRBitDefender: {"bdagent.exe", "vsserv.exe", "bdredline.exe"},
	EDRSymantec:    {"symantecsysplant.exe", "rtvscan.exe", "ccsvchst.exe", "sepsearchhelper.exe"},
	EDRTrendMicro:  {"tmcc.exe", "tmlisten.exe", "ntrtscan.exe", "tmlwf.exe"},
	EDRKaspersky:   {"avp.exe", "avpui.exe", "klnagent.exe"},
	EDRESET:        {"ekrn.exe", "egui.exe", "eoppmonitor.exe"},
	EDRMcAfee:      {"mcshield.exe", "mctray.exe", "mfemms.exe", "mfevtps.exe"},
	EDRSophos:      {"sophoshealth.exe", "sophosfs.exe", "sophoships.exe", "savservice.exe"},
	EDRCortex:      {"cyserver.exe", "cyvera.exe", "cyvrmt.exe", "traps.exe"},
	EDRElastic:     {"elastic-agent.exe", "elastic-endpoint.exe", "filebeat.exe"},
	EDRSysmon:      {"sysmon.exe", "sysmon64.exe"},
	EDR360:         {"360sd.exe", "360tray.exe", "zhudongfangyu.exe", "360safe.exe", "360rp.exe"},
	EDRHuorong:     {"hipsdaemon.exe", "hipsmain.exe", "hipstray.exe", "usysdiag.exe"},
	EDRTencent:     {"qqpcrtp.exe", "qqpctray.exe", "tencentdl.exe"},
}

var edrServiceSignatures = map[string][]string{
	EDRCrowdStrike: {"csagent", "csfalcon"},
	EDRDefender:    {"windefend", "msmpeng", "sense", "wscsvc"},
	EDRCarbonBlack: {"carbonblack", "cb"},
	EDRSentinelOne: {"sentinel", "sentinellogger"},
	EDRCylance:     {"cylance"},
	EDRBitDefender: {"bitdefender", "bdredline"},
	EDRSymantec:    {"symantec", "sep"},
	EDRTrendMicro:  {"trendmicro", "tmcc"},
	EDRKaspersky:   {"kavfs", "klnagent", "avp"},
	EDRESET:        {"ekrn", "eset"},
	EDRMcAfee:      {"mcshield", "mfe", "mcafee"},
	EDRSophos:      {"sophos", "savservice"},
	EDRCortex:      {"cyserver", "cyvera", "traps"},
	EDRElastic:     {"elastic", "endpoint"},
	EDRSysmon:      {"sysmon"},
	EDR360:         {"360", "zhudongfangyu"},
	EDRHuorong:     {"hipsdaemon", "huorong"},
	EDRTencent:     {"qqpcrtp", "tencent"},
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
