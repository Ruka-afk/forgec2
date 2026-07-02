//go:build !windows
// +build !windows

package main

func exportWifiCreds() string {
	return "wifi_creds: Windows-only (netsh wlan)\n"
}