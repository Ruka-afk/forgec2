package server

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestHandleDBBackup_ProducesValidSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := testutil.SetupTestDB(t)
	if err := d.Exec("CREATE TABLE probe_backup (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	if err := d.Exec("INSERT INTO probe_backup (name) VALUES ('a'), ('b'), ('c')").Error; err != nil {
		t.Fatalf("seed probe data: %v", err)
	}

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "forgec2.db")
	if err := os.WriteFile(dbPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("create placeholder db file: %v", err)
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("rand: %v", err)
	}
	keyHex := hex.EncodeToString(keyBytes)

	cfg := &config.Config{}
	cfg.Database.Path = dbPath
	cfg.Server.DataDir = tmp

	s := &Server{db: d, cfg: cfg}
	bm, err := NewBackupManager(d, dbPath, filepath.Join(tmp, "backups"), keyHex)
	if err != nil {
		t.Fatalf("NewBackupManager: %v", err)
	}
	s.backupManager = bm

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup", nil)
	c.Set("user_role", "admin")

	s.handleDBBackup(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	backupDir := filepath.Join(tmp, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	// Encrypted .fbk by default (P0-2): exactly one .fbk, no plaintext .db.
	var fbk string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".fbk") {
			fbk = e.Name()
		}
		if strings.HasSuffix(e.Name(), ".db") {
			t.Fatalf("plaintext .db backup produced by default: %s", e.Name())
		}
	}
	if fbk == "" {
		t.Fatalf("no .fbk backup produced, dir: %v", entries)
	}
	enc, err := os.ReadFile(filepath.Join(backupDir, fbk))
	if err != nil {
		t.Fatalf("read fbk: %v", err)
	}
	if isSQLiteFile(bufio.NewReader(bytes.NewReader(enc))) {
		t.Fatalf(".fbk payload is plaintext sqlite")
	}
	plain, err := decryptBackupData(enc, keyBytes)
	if err != nil {
		t.Fatalf("decrypt fbk: %v", err)
	}
	decPath := filepath.Join(tmp, "decrypted.db")
	if err := os.WriteFile(decPath, plain, 0600); err != nil {
		t.Fatalf("write decrypted: %v", err)
	}

	verify := testutil.SetupTestDB(t)
	if err := verify.Exec("ATTACH DATABASE ? AS bak", decPath).Error; err != nil {
		t.Fatalf("attach backup: %v", err)
	}
	var n int
	if err := verify.Raw("SELECT COUNT(*) FROM bak.probe_backup").Scan(&n).Error; err != nil {
		t.Fatalf("query backup contents: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 rows in backup snapshot, got %d", n)
	}

	verifySQL, err := verify.DB()
	if err != nil {
		t.Fatalf("get verify sql db: %v", err)
	}
	if err := verifySQL.Close(); err != nil {
		t.Fatalf("close verify db: %v", err)
	}
}

// TestHandleDBBackup_RequiresKeyForEncryptedDefault proves the manual
// endpoint fails closed (not plaintext) when no backup manager/key exists.
func TestHandleDBBackup_RequiresKeyForEncryptedDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := testutil.SetupTestDB(t)
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "forgec2.db")
	if err := os.WriteFile(dbPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("create placeholder db file: %v", err)
	}
	cfg := &config.Config{}
	cfg.Database.Path = dbPath
	cfg.Server.DataDir = tmp
	s := &Server{db: d, cfg: cfg} // no backupManager: no crypto.backup_key

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup", nil)
	c.Set("user_role", "admin")
	s.handleDBBackup(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "backup_key") {
		t.Fatalf("error must name crypto.backup_key: %s", w.Body.String())
	}
}

// TestHandleDBBackup_PlaintextOptOut proves ?encrypt=false still yields a
// restorable plaintext snapshot and audits the opt-out explicitly.
func TestHandleDBBackup_PlaintextOptOut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := testutil.SetupTestDB(t)
	if err := d.Exec("CREATE TABLE probe_plain (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "forgec2.db")
	if err := os.WriteFile(dbPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("create placeholder db file: %v", err)
	}
	cfg := &config.Config{}
	cfg.Database.Path = dbPath
	cfg.Server.DataDir = tmp
	s := &Server{db: d, cfg: cfg}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup?encrypt=false", nil)
	c.Set("user_role", "admin")
	c.Set("user", "backup-op")
	s.handleDBBackup(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(tmp, "backups"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly 1 backup file, got %v err %v", entries, err)
	}
	if !strings.HasSuffix(entries[0].Name(), ".db") {
		t.Fatalf("opt-out must produce .db, got %s", entries[0].Name())
	}
	raw, err := os.ReadFile(filepath.Join(tmp, "backups", entries[0].Name()))
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if !isSQLiteFile(bufio.NewReader(bytes.NewReader(raw))) {
		t.Fatalf("opt-out backup is not plaintext sqlite")
	}
	var audited int64
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "db_backup").Count(&audited).Error; err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if audited == 0 {
		t.Fatalf("no db_backup audit row for plaintext opt-out")
	}
}

func TestWriteRestoredDB_KeepsLiveDBOnInvalidSource(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "live.db")
	if err := os.WriteFile(dbPath, []byte("ORIGINAL DATABASE CONTENT"), 0600); err != nil {
		t.Fatalf("write live db: %v", err)
	}

	br := bufio.NewReader(strings.NewReader("this is not a sqlite database at all"))
	if err := writeRestoredDB(br, dbPath); err == nil {
		t.Fatal("expected error for invalid sqlite source")
	}

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read live db: %v", err)
	}
	if string(data) != "ORIGINAL DATABASE CONTENT" {
		t.Fatalf("live database was modified on failed restore: %q", string(data))
	}
}

func TestWriteRestoredDB_SwapsValidDatabase(t *testing.T) {
	src := testutil.SetupTestDB(t)
	if err := src.Exec("CREATE TABLE probe_restore (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	if err := src.Exec("INSERT INTO probe_restore (name) VALUES ('x')").Error; err != nil {
		t.Fatalf("seed probe data: %v", err)
	}
	tmp := t.TempDir()
	srcPath := filepath.Join(tmp, "src.db")
	if err := src.Exec("VACUUM INTO ?", srcPath).Error; err != nil {
		t.Fatalf("create source sqlite file: %v", err)
	}

	dbPath := filepath.Join(tmp, "live.db")
	if err := os.WriteFile(dbPath, []byte("ORIGINAL DATABASE CONTENT"), 0600); err != nil {
		t.Fatalf("write live db: %v", err)
	}

	f, err := os.Open(srcPath)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	if err := writeRestoredDB(bufio.NewReader(f), dbPath); err != nil {
		f.Close()
		t.Fatalf("writeRestoredDB: %v", err)
	}
	f.Close()

	verify := testutil.SetupTestDB(t)
	if err := verify.Exec("ATTACH DATABASE ? AS bak", dbPath).Error; err != nil {
		t.Fatalf("attach restored db: %v", err)
	}
	var n int
	if err := verify.Raw("SELECT COUNT(*) FROM bak.probe_restore").Scan(&n).Error; err != nil {
		t.Fatalf("query restored db: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row in restored db, got %d", n)
	}
	verifySQL, _ := verify.DB()
	verifySQL.Close()

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".forgec2-restore-") {
			t.Fatalf("leftover restore temp file: %s", e.Name())
		}
	}
}

func TestHandleRestoreFromUpload_RejectsOversizedFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "x.db")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(bytes.Repeat([]byte{0}, int(MaxUploadSize)+1)); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	mw.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup/restore", &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c.Set("user_role", "admin")

	s.handleRestoreFromUpload(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleRestoreFromFile_RejectsOversizedBackup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	d := testutil.SetupTestDB(t)
	tmp := t.TempDir()

	dbPath := filepath.Join(tmp, "live.db")
	if err := os.WriteFile(dbPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("create live db: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.Path = dbPath
	cfg.Server.DataDir = tmp

	backupPath := filepath.Join(tmp, "backups", "oversized.db")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0700); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	if err := os.WriteFile(backupPath, bytes.Repeat([]byte{0}, int(MaxUploadSize)+1), 0600); err != nil {
		t.Fatalf("create oversized backup: %v", err)
	}

	s := &Server{db: d, cfg: cfg}

	form := url.Values{}
	form.Set("type", "file")
	form.Set("name", "oversized.db")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup/restore", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "admin")

	s.handleRestoreFromFile(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}

	live, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read live db: %v", err)
	}
	if string(live) != "placeholder" {
		t.Fatalf("live database was modified on rejected restore: %q", string(live))
	}
}

func encryptBackupForTest(t *testing.T, keyHex string, plaintext []byte) []byte {
	t.Helper()
	bm, err := NewBackupManager(nil, "dummy.db", t.TempDir(), keyHex)
	if err != nil {
		t.Fatalf("NewBackupManager: %v", err)
	}
	encrypted, err := bm.encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt backup: %v", err)
	}
	return encrypted
}

func TestPrepareRestoreSource_DecryptsFbk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("rand: %v", err)
	}
	keyHex := hex.EncodeToString(keyBytes)

	src := testutil.SetupTestDB(t)
	tmp := t.TempDir()
	sqlitePath := filepath.Join(tmp, "src.sqlite")
	if err := src.Exec("VACUUM INTO ?", sqlitePath).Error; err != nil {
		t.Fatalf("vacuum source: %v", err)
	}
	raw, err := os.ReadFile(sqlitePath)
	if err != nil {
		t.Fatalf("read sqlite: %v", err)
	}
	encrypted := encryptBackupForTest(t, keyHex, raw)

	s := &Server{cfg: &config.Config{}}
	s.cfg.Crypto.BackupKey = keyHex

	reader, err := s.prepareRestoreSource(bytes.NewReader(encrypted), ".fbk")
	if err != nil {
		t.Fatalf("prepareRestoreSource: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read restored source: %v", err)
	}
	if !bytes.Equal(out, raw) {
		t.Fatal("decrypted .fbk source does not match original sqlite bytes")
	}
}

func TestPrepareRestoreSource_RejectsWrongKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	goodKeyBytes := make([]byte, 32)
	badKeyBytes := make([]byte, 32)
	if _, err := rand.Read(goodKeyBytes); err != nil {
		t.Fatalf("rand: %v", err)
	}
	if _, err := rand.Read(badKeyBytes); err != nil {
		t.Fatalf("rand: %v", err)
	}
	goodHex := hex.EncodeToString(goodKeyBytes)

	encrypted := encryptBackupForTest(t, goodHex, []byte("sensitive database content"))

	s := &Server{cfg: &config.Config{}}
	s.cfg.Crypto.BackupKey = hex.EncodeToString(badKeyBytes)

	if _, err := s.prepareRestoreSource(bytes.NewReader(encrypted), ".fbk"); err == nil {
		t.Fatal("expected error when decrypting .fbk with a mismatched backup key")
	}
}

func TestPrepareRestoreSource_RejectsGarbageFbk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := &Server{cfg: &config.Config{}}
	if _, err := s.prepareRestoreSource(bytes.NewReader([]byte("not a backup at all")), ".fbk"); err == nil {
		t.Fatal("expected error for corrupt .fbk content")
	}
}

func TestHandleRestoreFromFile_RestoresEncryptedFbk(t *testing.T) {
	gin.SetMode(gin.TestMode)

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("rand: %v", err)
	}
	keyHex := hex.EncodeToString(keyBytes)

	src := testutil.SetupTestDB(t)
	if err := src.Exec("CREATE TABLE probe_fbk (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	if err := src.Exec("INSERT INTO probe_fbk (name) VALUES ('restored')").Error; err != nil {
		t.Fatalf("seed probe data: %v", err)
	}

	tmp := t.TempDir()
	sqlitePath := filepath.Join(tmp, "src.sqlite")
	if err := src.Exec("VACUUM INTO ?", sqlitePath).Error; err != nil {
		t.Fatalf("vacuum source: %v", err)
	}
	raw, err := os.ReadFile(sqlitePath)
	if err != nil {
		t.Fatalf("read sqlite: %v", err)
	}
	encrypted := encryptBackupForTest(t, keyHex, raw)

	backupPath := filepath.Join(tmp, "backups", "forgec2_backup_20260101.fbk")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0700); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	if err := os.WriteFile(backupPath, encrypted, 0600); err != nil {
		t.Fatalf("write fbk: %v", err)
	}

	dbPath := filepath.Join(tmp, "live.db")
	if err := os.WriteFile(dbPath, []byte("placeholder"), 0600); err != nil {
		t.Fatalf("write live db: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.Path = dbPath
	cfg.Server.DataDir = tmp
	cfg.Crypto.BackupKey = keyHex

	s := &Server{db: src, cfg: cfg}

	form := url.Values{}
	form.Set("type", "file")
	form.Set("name", "forgec2_backup_20260101.fbk")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup/restore", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "admin")

	s.handleRestoreFromFile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	verify := testutil.SetupTestDB(t)
	if err := verify.Exec("ATTACH DATABASE ? AS bak", dbPath).Error; err != nil {
		t.Fatalf("attach restored db: %v", err)
	}
	var n int
	if err := verify.Raw("SELECT COUNT(*) FROM bak.probe_fbk").Scan(&n).Error; err != nil {
		t.Fatalf("query restored db: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row in restored db, got %d", n)
	}
	verifySQL, _ := verify.DB()
	verifySQL.Close()
}

func TestWriteRestoredDB_SavesPreRestoreCopy(t *testing.T) {
	tmp := t.TempDir()
	livePath := filepath.Join(tmp, "live.db")
	if err := os.WriteFile(livePath, []byte("live-marker-bytes"), 0600); err != nil {
		t.Fatalf("write live: %v", err)
	}

	src := testutil.SetupTestDB(t)
	if err := src.Exec("CREATE TABLE probe_pre (id INTEGER PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	candidate := filepath.Join(tmp, "candidate.db")
	if err := src.Exec("VACUUM INTO ?", candidate).Error; err != nil {
		t.Fatalf("vacuum candidate: %v", err)
	}
	data, err := os.ReadFile(candidate)
	if err != nil {
		t.Fatalf("read candidate: %v", err)
	}

	if err := writeRestoredDB(bytes.NewReader(data), livePath); err != nil {
		t.Fatalf("restore: %v", err)
	}
	matches, err := filepath.Glob(livePath + ".pre-restore-*")
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected 1 pre-restore copy, got %v (%v)", matches, err)
	}
	kept, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read pre-restore: %v", err)
	}
	if string(kept) != "live-marker-bytes" {
		t.Fatalf("pre-restore copy does not hold the old live bytes: %q", kept)
	}
}

func TestHandleRestoreFromFile_RejectsSidecarKeyMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		t.Fatalf("rand: %v", err)
	}
	keyHex := hex.EncodeToString(keyBytes)

	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backups", "forgec2_backup_20260101.fbk")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0700); err != nil {
		t.Fatalf("create backup dir: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("opaque"), 0600); err != nil {
		t.Fatalf("write fbk: %v", err)
	}
	// Sidecar sealed under some OTHER key generation.
	meta := `{"version":"fbk1","created_at":"2026-01-01T00:00:00Z","key_id":"deadbeef","server_version":"dev"}`
	if err := os.WriteFile(backupPath+".meta.json", []byte(meta), 0600); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	cfg := &config.Config{}
	cfg.Database.Path = filepath.Join(tmp, "live.db")
	cfg.Server.DataDir = tmp
	cfg.Crypto.BackupKey = keyHex

	s := &Server{db: testutil.SetupTestDB(t), cfg: cfg}

	form := url.Values{}
	form.Set("type", "file")
	form.Set("name", "forgec2_backup_20260101.fbk")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/admin/backup/restore", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "admin")

	s.handleRestoreFromFile(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 key mismatch, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "different key") {
		t.Fatalf("expected explicit key-mismatch message, got %s", w.Body.String())
	}
}
