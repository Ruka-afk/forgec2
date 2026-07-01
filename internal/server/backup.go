package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
	running   bool
	mu        sync.Mutex
	ticker    *time.Ticker
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
	go func() {
		bm.PerformBackup()
		for range bm.ticker.C {
			bm.PerformBackup()
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
		data, err := os.ReadFile(bm.dbPath)
		if err != nil {
			slog.Error("Failed to read database file", "error", err)
			return err
		}
		if err := os.WriteFile(backupPath, data, 0600); err != nil {
			slog.Error("Failed to write temp backup file", "error", err)
			return err
		}
	}

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

	timestamp := time.Now().Format(backupTimestamp)
	backupFile := filepath.Join(bm.backupDir, fmt.Sprintf("forgec2_backup_%s.fbk", timestamp))

	if err := os.WriteFile(backupFile, encryptedData, 0600); err != nil {
		slog.Error("Failed to write backup file", "error", err)
		return err
	}

	slog.Info("Backup completed", "file", backupFile, "size", len(encryptedData), "duration", time.Since(start))

	bm.cleanupOldBackups()

	return nil
}

func (bm *BackupManager) encrypt(data []byte) ([]byte, error) {
	salt := make([]byte, backupSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	iv := make([]byte, backupIVSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(bm.key)
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

func (bm *BackupManager) decrypt(encryptedData []byte) ([]byte, error) {
	if len(encryptedData) < backupIVSize+backupTagSize {
		return nil, fmt.Errorf("backup data too short")
	}

	iv := encryptedData[:backupIVSize]
	ciphertext := encryptedData[backupIVSize:]

	block, err := aes.NewCipher(bm.key)
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

func (bm *BackupManager) Restore(backupPath string) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	slog.Info("Starting database restore", "file", backupPath)

	data, err := os.ReadFile(backupPath)
	if err != nil {
		slog.Error("Failed to read backup file", "error", err)
		return err
	}

	decryptedData, err := bm.decrypt(data)
	if err != nil {
		slog.Error("Failed to decrypt backup", "error", err)
		return err
	}

	tempPath := bm.dbPath + ".restore"
	if err := os.WriteFile(tempPath, decryptedData, 0600); err != nil {
		slog.Error("Failed to write temp restore file", "error", err)
		return err
	}

	slog.Info("Restore file prepared. Server restart required to complete restore", "temp_file", tempPath)
	return fmt.Errorf("database restore file prepared at %s - restart server to complete restore", tempPath)
}

func (bm *BackupManager) ListBackups() ([]string, error) {
	files, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, err
	}

	var backups []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".fbk" {
			backups = append(backups, file.Name())
		}
	}

	return backups, nil
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

	for i := 0; i < len(backupFiles)-keepCount; i++ {
		os.Remove(filepath.Join(bm.backupDir, backupFiles[i].Name()))
	}

	slog.Info("Cleaned up old backups", "remaining", keepCount)
}

func (bm *BackupManager) GenerateKey() string {
	key := make([]byte, backupKeySize)
	if _, err := rand.Read(key); err != nil {
		return ""
	}
	return hex.EncodeToString(key)
}

func (bm *BackupManager) ValidateKey(key string) bool {
	parsedKey, err := hex.DecodeString(key)
	if err != nil {
		return false
	}
	return len(parsedKey) == backupKeySize
}

func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (bm *BackupManager) VerifyBackup(backupPath string) (bool, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return false, err
	}

	_, err = bm.decrypt(data)
	if err != nil {
		return false, err
	}

	return true, nil
}