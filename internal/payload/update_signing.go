package payload

// Update-signing trust root for agent self_update.
//
// The teamserver holds ONE ed25519 signing keypair; the public half is stamped
// into every built implant via buildLdflags (-X main.updatePinnedPubKeyHex),
// and the private half never leaves the server directory. self_update tasks
// therefore carry a signature the implant can verify against its pinned key,
// and an implant built without a stamp refuses to update at all (fail closed).
//
// Key material layout: the file stores the raw 64-byte ed25519 private key
// (seed + public suffix), permissions 0600 on POSIX.

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	updateSigningKey     []byte // ed25519.PrivateKey bytes (64)
	updateSigningKeyOnce sync.Once
	updateSigningKeyPath = filepath.Join("data", "update_signing.key")
	updateSigningErr     error
)

// SetUpdateSigningKeyFile overrides where the signing key persists. Must be
// called before the first use (server startup with the configured data dir).
func SetUpdateSigningKeyFile(path string) {
	if path != "" {
		updateSigningKeyPath = path
	}
}

func initUpdateSigningKey() {
	if data, err := os.ReadFile(updateSigningKeyPath); err == nil {
		if len(data) != ed25519.PrivateKeySize {
			updateSigningErr = fmt.Errorf(
				"update signing key %s has invalid length %d (want %d); refusing to overwrite",
				updateSigningKeyPath, len(data), ed25519.PrivateKeySize)
			return
		}
		updateSigningKey = data
		slog.Info("Update signing key loaded", "path", updateSigningKeyPath)
		return
	}
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		updateSigningErr = fmt.Errorf("generate update signing key: %w", err)
		return
	}
	updateSigningKey = priv
	if err := os.MkdirAll(filepath.Dir(updateSigningKeyPath), 0o750); err != nil {
		slog.Error("Failed to create update signing key directory",
			"dir", filepath.Dir(updateSigningKeyPath), "error", err)
		return
	}
	if err := os.WriteFile(updateSigningKeyPath, priv, 0o600); err != nil {
		slog.Error("Failed to persist update signing key; self_update pinning unavailable",
			"path", updateSigningKeyPath, "error", err)
		return
	}
	slog.Info("Update signing key generated and persisted", "path", updateSigningKeyPath)
}

// loadUpdateSigningKey lazily initialises the key and reports load errors.
func loadUpdateSigningKey() ([]byte, error) {
	updateSigningKeyOnce.Do(initUpdateSigningKey)
	if updateSigningErr != nil {
		return nil, updateSigningErr
	}
	if len(updateSigningKey) == 0 {
		return nil, fmt.Errorf("update signing key unavailable")
	}
	return updateSigningKey, nil
}

// UpdateSigningPublicKeyHex returns the hex-encoded ed25519 public key that
// gets stamped into builds. Used by buildLdflags and the operator API.
func UpdateSigningPublicKeyHex() (string, error) {
	priv, err := loadUpdateSigningKey()
	if err != nil {
		return "", err
	}
	pub := priv[32:]
	return hex.EncodeToString(pub), nil
}

// SignUpdateHash signs a SHA-256 digest (hex-encoded) of an update binary.
// The implant verifies exactly this: ed25519.Verify(pinnedPub, hash, sig).
func SignUpdateHash(sha256Hex string) (string, error) {
	hash, err := hex.DecodeString(sha256Hex)
	if err != nil || len(hash) != sha256.Size {
		return "", fmt.Errorf("invalid sha256 hex (want 64 chars): %q", sha256Hex)
	}
	priv, err := loadUpdateSigningKey()
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(ed25519.PrivateKey(priv), hash)
	return hex.EncodeToString(sig), nil
}
