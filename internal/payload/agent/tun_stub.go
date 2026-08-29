//go:build !linux

package main

import "fmt"

func startAgentTUN(cidr string) (string, error) {
	return "", fmt.Errorf("tun_start requires Linux /dev/net/tun (CAP_NET_ADMIN); not available on this OS (cidr %s). Windows needs a Wintun TAP helper which is not bundled", cidr)
}

func stopAgentTUN() error { return nil }

func tunWritePacket(pkt []byte) { _ = pkt }
