//go:build windows
// +build windows

package main

import (
	"fmt"
	"net"
	"time"
)

func netScanSMB(cidr string) (string, error) {
	if cidr == "" {
		cidr = "192.168.1.0/24"
	}
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %v", err)
	}
	var liveHosts []string
	results := make(chan string, 256)
	timeout := 2 * time.Second

	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		go func(addr net.IP) {
			target := net.JoinHostPort(addr.String(), "445")
			conn, err := net.DialTimeout("tcp", target, timeout)
			if err != nil {
				return
			}
			conn.Close()
			results <- addr.String()
		}(ip)
	}

	count := 0
	maxHosts := 1 << (32 - ones(ipnet.Mask))
	done := make(chan struct{})
	go func() {
		for r := range results {
			liveHosts = append(liveHosts, r)
		}
		close(done)
	}()

	time.Sleep(time.Duration(maxHosts/100+5) * time.Second)
	close(results)
	<-done

	_ = count
	out := fmt.Sprintf("SMB scan of %s complete. Found %d hosts:\n", cidr, len(liveHosts))
	for _, h := range liveHosts {
		out += "  " + h + "\n"
	}
	return out, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func ones(mask net.IPMask) int {
	n := 0
	for _, b := range mask {
		for b != 0 {
			n += int(b & 1)
			b >>= 1
		}
	}
	return n
}
