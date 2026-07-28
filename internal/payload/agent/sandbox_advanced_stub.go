//go:build !windows

package main

func getWindowsRAMGB() float64         { return 0 }
func getWindowsDiskGB(string) float64  { return 0 }
func getWindowsProcessCount() int      { return 0 }
func getVMVendorMACs() []string        { return nil }
func checkVMRegistryKeys() []string    { return nil }
func getWindowsUptimeMinutes() float64 { return 0 }
func getDesktopResolution() (int, int) { return 0, 0 }
func checkMouseMoved() bool            { return false }
func checkRDTSCVariance() bool         { return false }
func checkDRRegisters() bool           { return false }
