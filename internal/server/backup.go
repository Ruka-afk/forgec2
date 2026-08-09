package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	backupKeySize   = 32
	backupIVSize    = 12
	backupSaltSize  = 16
	backupTagSize   = 16
	backupTimestamp = "20060102_150405"
)

type BackupManager struct {
	db        *gorm.DB
	dbPath    string
	backupDir string
	key       []byte
	keyID     string
	running   bool
	mu        sync.Mutex
	ticker    *time.Ticker
	stopCh    chan struct{}
}

func NewBackupManager(db *gorm.DB, dbPath, backupDir, key string) (*BackupManager, error) {
	var backupKey []byte
	if key != "" {
		parsedKey, err := hex.DecodeString(key)
		if err != nil {
			return nil, err
		}
		if len(parsedKey) != backupKeySize {
			return nil, fmt.Errorf("backup key must be %d bytes (64 hex chars)", backupKeySize)
		}
		backupKey = parsedKey
	} else {
		backupKey = make([]byte, backupKeySize)
		if _, err := rand.Read(backupKey); err != nil {
			return nil, err
		}
		slog.Warn("No backup encryption key provided, using random key - backups cannot be restored if key is lost")
	}

	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return nil, err
	}

	return &BackupManager{
		db:        db,
		dbPath:    dbPath,
		backupDir: backupDir,
		key:       backupKey,
		keyID:     fmt.Sprintf("k%s", time.Now().Format("20060102")),
	}, nil
}

func (bm *BackupManager) Start(cronSchedule string) error {
	bm.mu.Lock()
	if bm.running {
		bm.mu.Unlock()
		return nil
	}
	bm.running = true
	bm.mu.Unlock()

	duration, err := parseCronSchedule(cronSchedule)
	if err != nil {
		return err
	}

	slog.Info("Backup manager started", "schedule", cronSchedule, "interval", duration)

	bm.ticker = time.NewTicker(duration)
	stopCh := make(chan struct{})
	bm.stopCh = stopCh
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		bm.PerformBackup()
		for {
			select {
			case <-bm.ticker.C:
				bm.PerformBackup()
			case <-stopCh:
				return
			}
		}
	}()

	return nil
}

func (bm *BackupManager) Stop() {
	bm.mu.Lock()
	bm.running = false
	if bm.ticker != nil {
		bm.ticker.Stop()
		bm.ticker = nil
	}
	if bm.stopCh != nil {
		close(bm.stopCh)
		bm.stopCh = nil
	}
	bm.mu.Unlock()
	slog.Info("Backup manager stopped")
}

func parseCronSchedule(schedule string) (time.Duration, error) {
	switch schedule {
	case "hourly":
		return 1 * time.Hour, nil
	case "daily":
		return 24 * time.Hour, nil
	case "weekly":
		return 7 * 24 * time.Hour, nil
	case "monthly":
		return 30 * 24 * time.Hour, nil
	default:
		duration, err := time.ParseDuration(schedule)
		if err != nil {
			return 0, fmt.Errorf("invalid backup schedule: %s", schedule)
		}
		return duration, nil
	}
}

func (bm *BackupManager) PerformBackup() error {
	bm.mu.Lock()
	if !bm.running {
		bm.mu.Unlock()
		return nil
	}
	bm.mu.Unlock()

	start := time.Now()
	slog.Info("Starting database backup")

	backupPath := filepath.Join(os.TempDir(), fmt.Sprintf("forgec2_backup_%d.db", time.Now().UnixNano()))
	defer os.Remove(backupPath)

	if err := bm.db.Exec("VACUUM INTO ?", backupPath).Error; err != nil {
		slog.Warn("VACUUM INTO backup failed, falling back to file copy", "error", err)
		if err := bm.db.Exec("BEGIN IMMEDIATE").Error; err != nil {
			slog.Error("Failed to begin transaction", "error", err)
		}
		dbFile, err := os.Open(bm.dbPath)
		if err != nil {
			if rerr := bm.db.Exec("ROLLBACK").Error; rerr != nil {
				slog.Warn("Backup rollback failed", "error", rerr)
			}
			slog.Error("Failed to open database file", "error", err)
			return err
		}
		tmpFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			dbFile.Close()
			if rerr := bm.db.Exec("ROLLBACK").Error; rerr != nil {
				slog.Warn("Backup rollback failed", "error", rerr)
			}
			slog.Error("Failed to create temp backup file", "error", err)
			return err
		}
		if _, err := io.Copy(tmpFile, dbFile); err != nil {
			tmpFile.Close()
			dbFile.Close()
			if rerr := bm.db.Exec("ROLLBACK").Error; rerr != nil {
				slog.Warn("Backup rollback failed", "error", rerr)
			}
			slog.Error("Failed to copy database file", "error", err)
			return err
		}
		tmpFile.Close()
		dbFile.Close()
		if err := bm.db.Exec("ROLLBACK").Error; err != nil {
			slog.Error("Failed to rollback transaction", "error", err)
		}
	}

	timestamp := time.Now().Format(backupTimestamp)
	backupFile := filepath.Join(bm.backupDir, fmt.Sprintf("forgec2_backup_%s.fbk", timestamp))

	data, err := os.ReadFile(backupPath)
	if err != nil {
		slog.Error("Failed to read backup file", "error", err)
		return err
	}

	encryptedData, err := bm.encrypt(data)
	if err != nil {
		slog.Error("Failed to encrypt backup", "error", err)
		return err
	}

	if err := os.WriteFile(backupFile, encryptedData, 0600); err != nil {
		slog.Error("Failed to write backup file", "error", err)
		return err
	}

	slog.Info("Backup completed", "file", backupFile, "size", len(encryptedData), "duration", time.Since(start))

	bm.cleanupOldBackups()

	return nil
}

func (bm *BackupManager) encrypt(data []byte) ([]byte, error) {
	return bm.encryptWithKey(data, bm.key)
}

func (bm *BackupManager) decrypt(encryptedData []byte) ([]byte, error) {
	return bm.decryptWithKey(encryptedData, bm.key)
}

func (bm *BackupManager) cleanupOldBackups() {
	files, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return
	}

	keepCount := 7
	var backupFiles []os.FileInfo

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".fbk" {
			info, _ := file.Info()
			backupFiles = append(backupFiles, info)
		}
	}

	if len(backupFiles) <= keepCount {
		return
	}

	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].ModTime().Before(backupFiles[j].ModTime())
	})

	for i := 0; i < len(backupFiles)-keepCount; i++ {
		if err := os.Remove(filepath.Join(bm.backupDir, backupFiles[i].Name())); err != nil {
			slog.Warn("Failed to remove old backup", "file", backupFiles[i].Name(), "error", err)
		}
	}

	slog.Info("Cleaned up old backups", "deleted", len(backupFiles)-keepCount, "remaining", keepCount)
}

func (bm *BackupManager) RotateKey(newKeyHex string) error {
	parsedKey, err := hex.DecodeString(newKeyHex)
	if err != nil {
		return fmt.Errorf("invalid hex key: %w", err)
	}
	if len(parsedKey) != backupKeySize {
		return fmt.Errorf("key must be %d bytes", backupKeySize)
	}

	files, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return err
	}

	oldKey := bm.key
	reencrypted := 0
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".fbk" {
			continue
		}
		path := filepath.Join(bm.backupDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("Failed to read backup for re-encryption", "file", file.Name(), "error", err)
			continue
		}
		plaintext, err := bm.decryptWithKey(data, oldKey)
		if err != nil {
			slog.Warn("Failed to decrypt backup with old key", "file", file.Name(), "error", err)
			continue
		}
		enc, err := bm.encryptWithKey(plaintext, parsedKey)
		if err != nil {
			slog.Warn("Failed to re-encrypt backup", "file", file.Name(), "error", err)
			continue
		}
		if err := os.WriteFile(path, enc, 0600); err != nil {
			slog.Error("Failed to write re-encrypted backup, aborting key rotation", "file", file.Name(), "error", err)
			return fmt.Errorf("key rotation aborted: failed to write re-encrypted backup %s: %w", file.Name(), err)
		}
		reencrypted++
	}

	bm.key = parsedKey
	bm.keyID = fmt.Sprintf("k%s", time.Now().Format("20060102"))
	slog.Info("Backup key rotated", "reencrypted", reencrypted)
	return nil
}

func (bm *BackupManager) encryptWithKey(data []byte, key []byte) ([]byte, error) {
	salt := make([]byte, backupSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	iv := make([]byte, backupIVSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext := append(salt, data...)
	ciphertext := aead.Seal(nil, iv, plaintext, nil)
	result := make([]byte, 0, len(iv)+len(ciphertext))
	result = append(result, iv...)
	result = append(result, ciphertext...)
	return result, nil
}

func (bm *BackupManager) decryptWithKey(encryptedData []byte, key []byte) ([]byte, error) {
	return decryptBackupData(encryptedData, key)
}

// decryptBackupData decrypts an encrypted .fbk backup blob with a 32-byte key.
// It accepts both the newer HMAC-authenticated format and the legacy format.
func decryptBackupData(encryptedData []byte, key []byte) ([]byte, error) {
	// Try HMAC-SHA256 verification first (new format)
	hasHMAC := false
	if len(encryptedData) >= backupIVSize+backupTagSize+32 {
		dataWithIV := encryptedData[:len(encryptedData)-32]
		receivedHMAC := encryptedData[len(encryptedData)-32:]

		mac := hmac.New(sha256.New, key)
		mac.Write(dataWithIV)
		expectedHMAC := mac.Sum(nil)

		if hmac.Equal(receivedHMAC, expectedHMAC) {
			hasHMAC = true
			encryptedData = dataWithIV
		}
	}

	if !hasHMAC {
		if len(encryptedData) < backupIVSize+backupTagSize {
			return nil, fmt.Errorf("backup data too short")
		}
	}

	iv := encryptedData[:backupIVSize]
	ciphertext := encryptedData[backupIVSize:]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	if len(plaintext) < backupSaltSize {
		return nil, fmt.Errorf("invalid backup format")
	}
	return plaintext[backupSaltSize:], nil
}

// backupKeyHex returns the hex key used to encrypt/decrypt .fbk backup files.
// crypto.backup_key is REQUIRED (validated at startup): the legacy derivation
// cascade (crypto.key, then the JWT secret) was removed so backups are
// cryptographically independent of other secrets. Backups created by older
// versions that re-used the derived key can no longer be restored.
func (s *Server) backupKeyHex() string {
	return s.cfg.Crypto.BackupKey
}

// backupKey decodes backupKeyHex into the 32 raw key bytes used for .fbk
// encryption/decryption.
func (s *Server) backupKey() []byte {
	hexKey := s.backupKeyHex()
	b, err := hex.DecodeString(hexKey)
	if err != nil || len(b) != backupKeySize {
		h := sha256.Sum256([]byte(hexKey))
		return h[:32]
	}
	return b
}
