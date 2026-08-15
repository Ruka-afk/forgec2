//go:build !windows
// +build !windows

package main

// runPowerShellStdin is only meaningful on Windows (powershell.exe). On other
// platforms mimikatz is unreachable anyway, so the stub returns an error to
// keep the symbol resolvable for the cross-platform caller in agent_creds.go.
func runPowerShellStdin(script string) (string, error) {
	return "", fmt.Errorf("powershell is Windows-only")
}
