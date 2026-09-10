package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func setupAIRunReviewTest(t *testing.T) *Server {
	t.Helper()
	return setupAIRunStorageTest(t)
}

func TestAIRunReviewValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := setupAIRunStorageTest(t)
	s.cfg.AI.Enabled = true
	s.cfg.AI.APIKey = "test-key"
	s.db.Create(&db.User{Username: "alice", Role: db.RoleAdmin, TenantID: 1})

	call := func(body string, user string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/review", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		if user != "" {
			c.Set("user", user)
			c.Set("user_role", db.RoleAdmin)
		}
		_ = config.Config{}
		s.handleAIRunReview(c)
		return w
	}

	// No principal -> forbidden.
	if w := call(`{"run_id":"x"}`, ""); w.Code != http.StatusForbidden {
		t.Fatalf("missing principal = %d, want 403", w.Code)
	}
	// Empty body -> bad request.
	if w := call(`{}`, "alice"); w.Code != http.StatusBadRequest {
		t.Fatalf("empty body = %d, want 400", w.Code)
	}
	// Unknown run -> not found.
	if w := call(`{"run_id":"does-not-exist"}`, "alice"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown run = %d, want 404", w.Code)
	}

	// Unfinished run -> bad request.
	s.db.Create(&db.AIChatRun{ID: "run-active", TenantID: 1, OwnerID: 1, Owner: "alice", SessionID: 3, Status: "running", IdempotencyKey: "k-active"})
	var alice db.User
	s.db.Where("username = ?", "alice").First(&alice)
	s.db.Model(&db.AIChatRun{}).Where("id = ?", "run-active").Update("owner_id", alice.ID)
	if w := call(`{"run_id":"run-active"}`, "alice"); w.Code != http.StatusBadRequest {
		t.Fatalf("unfinished run = %d, want 400", w.Code)
	}

	// Finished run without events -> bad request.
	s.db.Create(&db.AIChatRun{ID: "run-empty", TenantID: 1, OwnerID: alice.ID, Owner: "alice", SessionID: 3, Status: "completed", IdempotencyKey: "k-empty"})
	if w := call(`{"run_id":"run-empty"}`, "alice"); w.Code != http.StatusBadRequest {
		body, _ := json.Marshal(w.Body.String())
		t.Fatalf("eventless run = %d, want 400 (%s)", w.Code, body)
	}
}
