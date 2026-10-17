//go:build !linux && !(windows && wintun)

package main

import (
	"fmt"
	"runtime"
)

func startAgentTUN(cidr string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("tun_start on Windows needs the bundled Wintun runtime; this implant was built without it (stage the runtime with scripts/fetch-wintun.ps1 and rebuild) (cidr %s)", cidr)
	}
	return "", fmt.Errorf("tun_start requires Linux /dev/net/tun (CAP_NET_ADMIN); not available on this OS (cidr %s)", cidr)
}

func stopAgentTUN() error { return nil }

func tunWritePacket(pkt []byte) { _ = pkt }

// preloadWintunBeforeHardening is a no-op without the bundled runtime: there
// is nothing the signature policy could block later.
func preloadWintunBeforeHardening() {}
