//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"io"
	"net"
)

func startSocksServer(addr string) {
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			debugLog("SOCKS listen failed: " + err.Error())
			return
		}
		debugLog("SOCKS5 listening on " + addr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleSocksConn(conn)
		}
	}()
}

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
	conn.Write([]byte{0x05, 0x00})

	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != 0x05 || header[1] != 0x01 {
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var dstAddr string
	switch header[3] {
	case 0x01:
		ip := make([]byte, 4)
		io.ReadFull(conn, ip)
		portb := make([]byte, 2)
		io.ReadFull(conn, portb)
		dstAddr = fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], int(portb[0])<<8|int(portb[1]))
	case 0x03:
		l := make([]byte, 1)
		io.ReadFull(conn, l)
		dom := make([]byte, int(l[0]))
		io.ReadFull(conn, dom)
		portb := make([]byte, 2)
		io.ReadFull(conn, portb)
		dstAddr = fmt.Sprintf("%s:%d", string(dom), int(portb[0])<<8|int(portb[1]))
	default:
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	target, err := net.Dial("tcp", dstAddr)
	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	go io.Copy(target, conn)
	io.Copy(conn, target)
}