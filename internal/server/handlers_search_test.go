package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSearchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&db.Implant{}, &db.Listener{}, &db.CredentialEntry{}, &db.BOFFile{}, &db.User{}, &db.Task{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Create(&db.Implant{ID: "agent-1", Hostname: "WORKSTATION01", Username: "admin", IP: "10.0.0.5", OS: "Windows"})
	database.Create(&db.Listener{Name: "primary-http", Host: "c2.local", Port: 443, Scheme: "https"})
	return database
}

func TestHandleAPISearch_ReturnsMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupSearchTestDB(t)
	s := &Server{db: database}

	r := gin.New()
	r.GET("/api/search", s.handleAPISearch)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=WORKSTATION", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Success bool           `json:"success"`
		Results []SearchResult `json:"results"`
		Query   string         `json:"query"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || resp.Query != "WORKSTATION" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(resp.Results) == 0 || resp.Results[0].Type != "agent" {
		t.Fatalf("expected agent result, got %+v", resp.Results)
	}
}

func TestHandleAPISearch_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: setupSearchTestDB(t)}

	r := gin.New()
	r.GET("/api/search", s.handleAPISearch)

	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Success bool           `json:"success"`
		Results []SearchResult `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || len(resp.Results) != 0 {
		t.Fatalf("expected empty results, got %+v", resp)
	}
}