package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func newStageTestServer(t *testing.T, dataDir string) (*Server, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	cfg := &config.Config{}
	cfg.Server.DataDir = dataDir
	s := &Server{
		db:          database,
		cfg:         cfg,
		eventManager: NewEventManager(database),
	}
	r := gin.New()
	r.GET("/stage/:token", s.handleServeStage)
	s.router = r
	return s, database
}

// stageFixture provisions a signed token whose encrypted blob is pre-written.
func stageFixture(t *testing.T, dataDir string, expiresAt time.Time) (*Server, string, string, []byte, []byte) {
	t.Helper()
	payload.SetStagerKey([]byte("0123456789abcdef0123456789abcdef"))
	s, database := newStageTestServer(t, dataDir)

	token, sig, _, err := payload.NewStageToken()
	if err != nil {
		t.Fatalf("NewStageToken: %v", err)
	}
	st := db.StagerToken{
		Token:        token,
		ListenerID:   1,
		OS:           "linux",
		Architecture: "amd64",
		Format:       "exe",
		ExpiresAt:    expiresAt,
	}
	if err := database.Create(&st).Error; err != nil {
		t.Fatalf("create stager token: %v", err)
	}

	plaintext := []byte("MZ-fake-stage2-payload-fake-stage2-payload")
	if err := payload.WriteStage2Blob(dataDir, token, plaintext); err != nil {
		t.Fatalf("WriteStage2Blob: %v", err)
	}
	blob, err := payload.LoadStage2Blob(dataDir, token)
	if err != nil {
		t.Fatalf("LoadStage2Blob: %v", err)
	}
	return s, token, sig, plaintext, blob
}

func stageGet(t *testing.T, s *Server, urlPath string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, urlPath, nil)
	s.router.ServeHTTP(w, req)
	return w
}

func TestStageServeSigned(t *testing.T) {
	s, token, sig, plaintext, blob := stageFixture(t, t.TempDir(), time.Now().Add(time.Hour))

	w := stageGet(t, s, "/stage/"+token+"?s="+sig)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", w.Code, w.Body.String())
	}
	if w.Body.String() == string(plaintext) {
		t.Fatal("stage served plaintext")
	}
	if w.Body.String() != string(blob) {
		t.Fatal("stage body does not match encrypted blob")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}
}

func TestStageServeBadSignature(t *testing.T) {
	s, token, _, _, _ := stageFixture(t, t.TempDir(), time.Now().Add(time.Hour))

	w := stageGet(t, s, "/stage/"+token+"?s=tampered")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	// Missing signature entirely must also be rejected.
	w = stageGet(t, s, "/stage/"+token)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestStageServeExpired(t *testing.T) {
	s, token, sig, _, _ := stageFixture(t, t.TempDir(), time.Now().Add(-time.Hour))

	w := stageGet(t, s, "/stage/"+token+"?s="+sig)
	if w.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", w.Code)
	}
}

func TestStageServeUnknownToken(t *testing.T) {
	s, _, sig, _, _ := stageFixture(t, t.TempDir(), time.Now().Add(time.Hour))

	other, _, _, err := payload.NewStageToken()
	if err != nil {
		t.Fatalf("NewStageToken: %v", err)
	}
	w := stageGet(t, s, "/stage/"+other+"?s="+sig)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (signature bound to a different token)", w.Code)
	}
}

func TestStageServeNonHexToken(t *testing.T) {
	s, _, _, _, _ := stageFixture(t, t.TempDir(), time.Now().Add(time.Hour))

	w := stageGet(t, s, "/stage/abcdefgh-not-hex")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestStageServeConsumesToken(t *testing.T) {
	dataDir := t.TempDir()
	s, token, sig, _, _ := stageFixture(t, dataDir, time.Now().Add(time.Hour))

	w := stageGet(t, s, "/stage/"+token+"?s="+sig)
	if w.Code != http.StatusOK {
		t.Fatalf("first fetch status = %d, want 200; body=%q", w.Code, w.Body.String())
	}
	// If a lazy rebuild path is hit the decrypted plaintext would be rebuilt;
	// the encrypted blob must not remain on disk after consumption.
	if _, err := os.Stat(payload.Stage2BlobPath(dataDir, token)); !os.IsNotExist(err) {
		t.Fatalf("stage blob should be removed after consumption, err=%v", err)
	}

	w = stageGet(t, s, "/stage/"+token+"?s="+sig)
	if w.Code != http.StatusGone {
		t.Fatalf("second fetch status = %d, want 410 (single-use)", w.Code)
	}
}

func TestStageServeConcurrentConsumesOnce(t *testing.T) {
	s, token, sig, _, _ := stageFixture(t, t.TempDir(), time.Now().Add(time.Hour))

	results := make(chan int, 2)
	done := make(chan struct{})
	for i := 0; i < 2; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			w := stageGet(t, s, "/stage/"+token+"?s="+sig)
			results <- w.Code
		}()
	}
	for i := 0; i < 2; i++ {
		<-done
	}
	close(results)

	ok, gone := 0, 0
	for code := range results {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusGone:
			gone++
		}
	}
	if ok != 1 || gone != 1 {
		t.Fatalf("expected exactly one 200 + one 410, got ok=%d gone=%d", ok, gone)
	}
}