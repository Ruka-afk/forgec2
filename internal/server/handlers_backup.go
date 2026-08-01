package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type backupEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// sqliteMagic is the file header signature for SQLite database files.
var sqliteMagic = []byte("SQLite format 3\x00")

// isSQLiteFile peeks at the first 16 bytes to check the SQLite magic header.
// The caller must use a bufio.Reader to avoid consuming bytes.
func isSQLiteFile(br *bufio.Reader) bool {
	header, err := br.Peek(16)
	if err != nil {
		return false
	}
	return string(header) == string(sqliteMagic)
}

func (s *Server) backupDir() string {
	return filepath.Join(s.cfg.Server.DataDir, "backups")
}

// removeStaleDBArtifacts deletes WAL/SHM sidecar files that belonged to the
// pre-restore database. Without this, SQLite would try to replay the old
// write-ahead log against the newly restored file on the next startup.
func removeStaleDBArtifacts(dbPath string) {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to remove stale database sidecar", "path", dbPath+suffix, "error", err)
		}
	}
}

// verifyRestoredDB performs post-restore integrity checks on the SQLite database.
func verifyRestoredDB(dbPath string) error {
	f, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("cannot open restored file: %w", err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	if !isSQLiteFile(br) {
		return fmt.Errorf("restored file is not a valid SQLite database")
	}

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat restored file: %w", err)
	}
	if fi.Size() < 1024 {
		return fmt.Errorf("restored file too small (%d bytes) — likely truncated", fi.Size())
	}

	return nil
}

// prepareRestoreSource returns a reader over the raw SQLite bytes of an upload
// or on-disk backup. Plain SQLite files pass through untouched; encrypted .fbk
// files (produced by the scheduled BackupManager) are decrypted with the
// configured backup key and verified before use.
func (s *Server) prepareRestoreSource(src io.Reader, ext string) (io.Reader, error) {
	br := bufio.NewReader(src)
	if isSQLiteFile(br) {
		return br, nil
	}
	if ext != ".fbk" {
		return nil, errors.New("file is not a valid SQLite database")
	}
	data, err := io.ReadAll(br)
	if err != nil {
		return nil, errors.New("failed to read backup file")
	}
	plaintext, err := decryptBackupData(data, s.backupKey())
	if err != nil {
		return nil, errors.New("failed to decrypt backup (key mismatch or corrupt file)")
	}
	if !isSQLiteFile(bufio.NewReader(bytes.NewReader(plaintext))) {
		return nil, errors.New("decrypted backup is not a valid SQLite database")
	}
	return bytes.NewReader(plaintext), nil
}

// writeRestoredDB copies the source stream to a temp file in the database
// directory, verifies it, then atomically swaps it over the live database.
// The live database is left untouched if verification or the swap fails.
func writeRestoredDB(src io.Reader, dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("prepare database directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".forgec2-restore-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		return fmt.Errorf("write database file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := verifyRestoredDB(tmpPath); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		return fmt.Errorf("swap database file: %w", err)
	}

	removeStaleDBArtifacts(dbPath)
	return nil
}

func (s *Server) handleDBBackupList(c *gin.Context) {
	dir := s.backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []backupEntry{}})
			return
		}
		respondError(c, http.StatusInternalServerError, "failed to read backup directory")
		return
	}

	var backups []backupEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".db" && ext != ".fbk" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupEntry{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime > backups[j].ModTime
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "data": backups})
}

func (s *Server) handleDBBackup(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	if _, err := os.Stat(s.cfg.Database.Path); err != nil {
		respondError(c, http.StatusInternalServerError, "database file not found")
		return
	}

	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create backup directory")
		return
	}

	ts := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("forgec2_%s.db", ts)
	backupPath := filepath.Join(dir, backupName)

	// Use SQLite's VACUUM INTO to produce a consistent snapshot. A raw file
	// copy is unsafe while the database runs in WAL mode: it would omit the
	// -wal file and yield an incomplete/corrupt backup.
	if err := s.db.Exec("VACUUM INTO ?", backupPath).Error; err != nil {
		slog.Error("Failed to create database backup", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create backup")
		return
	}

	fi, err := os.Stat(backupPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to stat backup file")
		return
	}

	slog.Info("Database backup created", "path", backupPath, "size", fi.Size())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": backupEntry{
			Name:    backupName,
			Size:    fi.Size(),
			ModTime: time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func (s *Server) handleDBRestore(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	restoreType := c.PostForm("type")

	switch restoreType {
	case "upload":
		s.handleRestoreFromUpload(c)
	case "file":
		s.handleRestoreFromFile(c)
	default:
		respondError(c, http.StatusBadRequest, "restore type must be 'upload' or 'file'")
	}
}

func (s *Server) handleRestoreFromUpload(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		respondError(c, http.StatusBadRequest, "no file uploaded")
		return
	}
	defer file.Close()

	if header.Size == 0 {
		respondError(c, http.StatusBadRequest, "uploaded file is empty")
		return
	}

	if header.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("file too large (max %d bytes)", MaxUploadSize))
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".db" && ext != ".fbk" {
		respondError(c, http.StatusBadRequest, "only .db or .fbk files are accepted")
		return
	}

	src, err := s.prepareRestoreSource(file, ext)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	dbPath := s.cfg.Database.Path
	if err := writeRestoredDB(src, dbPath); err != nil {
		slog.Error("Database restore from upload failed", "error", err)
		respondError(c, http.StatusInternalServerError, "restore failed")
		return
	}

	slog.Info("Database restored from uploaded file", "filename", header.Filename, "size", header.Size)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Database restored. Server will restart to apply changes.",
		"restart": true,
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		time.Sleep(500 * time.Millisecond)
		s.Shutdown()
	}()
}

func (s *Server) handleRestoreFromFile(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	name := c.PostForm("name")
	if name == "" {
		respondError(c, http.StatusBadRequest, "backup name is required")
		return
	}

	name = filepath.Base(name)
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		respondError(c, http.StatusBadRequest, "invalid backup name")
		return
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".db" && ext != ".fbk" {
		respondError(c, http.StatusBadRequest, "invalid backup file type")
		return
	}

	backupPath := filepath.Join(s.backupDir(), name)
	if _, err := os.Stat(backupPath); err != nil {
		respondError(c, http.StatusNotFound, "backup file not found")
		return
	}

	srcFile, err := os.Open(backupPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open backup file")
		return
	}
	defer srcFile.Close()

	fi, err := srcFile.Stat()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to stat backup file")
		return
	}

	if fi.Size() > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("backup file too large (max %d bytes)", MaxUploadSize))
		return
	}

	src, err := s.prepareRestoreSource(srcFile, ext)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	dbPath := s.cfg.Database.Path
	if err := writeRestoredDB(src, dbPath); err != nil {
		slog.Error("Database restore from backup failed", "error", err)
		respondError(c, http.StatusInternalServerError, "restore failed")
		return
	}

	slog.Info("Database restored from backup", "backup", name, "size", fi.Size())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Database restored. Server will restart to apply changes.",
		"restart": true,
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		time.Sleep(500 * time.Millisecond)
		s.Shutdown()
	}()
}

func (s *Server) handleDBBackupDownload(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		respondError(c, http.StatusBadRequest, "backup name is required")
		return
	}
	name = filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".db" && ext != ".fbk" {
		respondError(c, http.StatusBadRequest, "invalid backup file type")
		return
	}
	backupPath := filepath.Join(s.backupDir(), name)
	if _, err := os.Stat(backupPath); err != nil {
		respondError(c, http.StatusNotFound, "backup file not found")
		return
	}
	serveFileSafe(c, backupPath, s.backupDir(), name)
}
