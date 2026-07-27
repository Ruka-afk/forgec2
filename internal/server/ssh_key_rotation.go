package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

type SSHKeyRotator struct {
	db           *gorm.DB
	keyDir       string
	interval     time.Duration
	currentKeyID int
	mu           sync.RWMutex
	stopCh       chan struct{}
}

func NewSSHKeyRotator(database *gorm.DB, keyDir string, interval time.Duration) *SSHKeyRotator {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &SSHKeyRotator{
		db:       database,
		keyDir:   keyDir,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (r *SSHKeyRotator) Start(ctx context.Context) {
	if err := os.MkdirAll(r.keyDir, 0700); err != nil {
		slog.Error("Failed to create SSH key directory", "dir", r.keyDir, "err", err)
		return
	}

	slog.Info("SSH key rotator started", "interval", r.interval, "dir", r.keyDir)

	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopCh:
				return
			case <-ticker.C:
				if err := r.RotateKey(); err != nil {
					slog.Error("SSH key rotation failed", "err", err)
				}
			}
		}
	}()
}

func (r *SSHKeyRotator) Stop() {
	close(r.stopCh)
}

func (r *SSHKeyRotator) RotateKey() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currentKeyID++
	filename := fmt.Sprintf("host_key_%d_%s", r.currentKeyID, time.Now().Format("20060102"))
	keyPath := filepath.Join(r.keyDir, filename)

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to create SSH signer: %w", err)
	}

	privBlock, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(privBlock)

	keyFile := keyPath + "_ed25519"
	if err := os.WriteFile(keyFile, pemBytes, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	pubFile := keyPath + "_ed25519.pub"
	pubSSH := ssh.MarshalAuthorizedKey(signer.PublicKey())
	if err := os.WriteFile(pubFile, pubSSH, 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	var redirectors []db.Redirector
	if err := r.db.Where("ssh_key != '' AND ssh_key IS NOT NULL").Find(&redirectors).Error; err != nil {
		slog.Error("Failed to query redirectors for SSH key rotation", "err", err)
	} else {
		for _, rd := range redirectors {
			slog.Info("SSH key rotated for redirector",
				"redirector_id", rd.ID,
				"name", rd.Name,
				"new_pub_key", pubFile,
			)
		}
	}

	slog.Info("SSH host key rotated",
		"key_id", r.currentKeyID,
		"private_key", keyFile,
		"public_key", pubFile,
	)

	return nil
}

func (r *SSHKeyRotator) GetCurrentKeyPath() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filepath.Join(r.keyDir, fmt.Sprintf("host_key_%d_ed25519", r.currentKeyID))
}

func (r *SSHKeyRotator) GetCurrentPubKeyPath() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return filepath.Join(r.keyDir, fmt.Sprintf("host_key_%d_ed25519.pub", r.currentKeyID))
}

func (r *SSHKeyRotator) KeyCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentKeyID
}
