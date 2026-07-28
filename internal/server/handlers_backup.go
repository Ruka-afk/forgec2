package server

import (
	"bufio"
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
	src := s.cfg.Database.Path
	if _, err := os.Stat(src); err != nil {
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

	srcFile, err := os.Open(src)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open database file")
		return
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create backup file")
		return
	}
	defer dstFile.Close()

	n, err := io.Copy(dstFile, srcFile)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to copy database")
		return
	}

	slog.Info("Database backup created", "path", backupPath, "size", n)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": backupEntry{
			Name:    backupName,
			Size:    n,
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

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".db" && ext != ".fbk" {
		respondError(c, http.StatusBadRequest, "only .db or .fbk files are accepted")
		return
	}

	br := bufio.NewReader(file)
	if !isSQLiteFile(br) {
		respondError(c, http.StatusBadRequest, "uploaded file is not a valid SQLite database")
		return
	}

	dbPath := s.cfg.Database.Path
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to prepare database directory")
		return
	}

	dstFile, err := os.OpenFile(dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open database for writing")
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, br); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to write database file")
		return
	}
	dstFile.Close()

	if err := verifyRestoredDB(dbPath); err != nil {
		slog.Error("Restored database failed integrity check", "error", err)
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("restore failed verification: %v", err))
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

	backupPath := filepath.Join(s.backupDir(), name)
	if _, err := os.Stat(backupPath); err != nil {
		respondError(c, http.StatusNotFound, "backup file not found")
		return
	}

	dbPath := s.cfg.Database.Path
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		respondError(c, http.StatusInternalServerError, "failed to prepare database directory")
		return
	}

	srcFile, err := os.Open(backupPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open backup file")
		return
	}
	defer srcFile.Close()

	br := bufio.NewReader(srcFile)
	if !isSQLiteFile(br) {
		respondError(c, http.StatusBadRequest, "backup file is not a valid SQLite database")
		return
	}

	dstFile, err := os.OpenFile(dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to open database for writing")
		return
	}
	defer dstFile.Close()

	n, err := io.Copy(dstFile, br)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to restore database")
		return
	}
	dstFile.Close()

	if err := verifyRestoredDB(dbPath); err != nil {
		slog.Error("Restored database failed integrity check", "error", err)
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("restore failed verification: %v", err))
		return
	}

	slog.Info("Database restored from backup", "backup", name, "bytes", n)

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
