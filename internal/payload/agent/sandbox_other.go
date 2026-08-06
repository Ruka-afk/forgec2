//go:build !windows
// +build !windows

package main

// The following checks rely on Windows-only APIs (kernel32/user32/ntdll/
// iphlpapi, tasklist). On Linux/macOS they are compiled out and report
// no sandbox signal.

func (sd *SandboxDetector) checkMemorySize() bool          { return false }
func (sd *SandboxDetector) checkDiskSize() bool            { return false }
func (sd *SandboxDetector) checkVMProcesses() bool         { return false }
func (sd *SandboxDetector) checkVMMAC() bool               { return false }
func (sd *SandboxDetector) checkMouseMovement() bool       { return false }
func (sd *SandboxDetector) checkRDTSC() bool               { return false }
func (sd *SandboxDetector) checkHumanPresence() bool       { return false }
func (sd *SandboxDetector) checkHardwareBreakpoints() bool { return false }
func (sd *SandboxDetector) checkVMMACEnhanced() bool       { return false }
func (sd *SandboxDetector) checkDiskSizeSmall() bool       { return false }
func (sd *SandboxDetector) checkRAMSizeSmall() bool        { return false }

func (ad *AntiDebug) IsDebuggerPresent() bool        { return false }
func (ad *AntiDebug) CheckRemoteDebuggerPresent() bool { return false }
