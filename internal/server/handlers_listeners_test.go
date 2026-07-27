package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func newListenerTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t)}
}

func TestHandleListListeners_Empty(t *testing.T) {
	s := newListenerTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/listeners", nil)

	s.handleListListeners(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success  bool           `json:"success"`
		Data     []db.Listener  `json:"data"`
		Total    int64          `json:"total"`
		Page     int            `json:"page"`
		PageSize int            `json:"page_size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Total != 0 {
		t.Fatalf("expected total=0, got %d", resp.Total)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data, got %d entries", len(resp.Data))
	}
}

func TestHandleListListeners_WithData(t *testing.T) {
	s := newListenerTestServer(t)
	listener := db.Listener{
		Name:   "primary-http",
		Scheme: "https",
		Host:   "c2.example.com",
		Port:   443,
		Status: "running",
	}
	if err := s.db.Create(&listener).Error; err != nil {
		t.Fatalf("seed listener: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/listeners", nil)

	s.handleListListeners(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool          `json:"success"`
		Data    []db.Listener `json:"data"`
		Total   int64         `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Total != 1 {
		t.Fatalf("expected total=1, got %d", resp.Total)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.Data))
	}
	if resp.Data[0].Name != "primary-http" {
		t.Fatalf("expected name 'primary-http', got %q", resp.Data[0].Name)
	}
}
