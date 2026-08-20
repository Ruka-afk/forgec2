//go:build !windows || !amd64

package main

func HardwareBreakpointAMSI() string { return "HWBP AMSI: Windows AMD64 only" }
func HardwareBreakpointETW() string  { return "HWBP ETW: Windows AMD64 only" }
