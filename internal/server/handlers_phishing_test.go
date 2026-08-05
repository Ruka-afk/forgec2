package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func seedPhishingEvent(t *testing.T, s *Server, token string) {
	t.Helper()
	if err := s.db.Create(&db.PhishingEvent{
		CampaignID: 1,
		Token:      token,
		Email:      "target@example.com",
		EventType:  "sent",
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed phishing event: %v", err)
	}
}

func phishingLandingRequest(t *testing.T, s *Server, method, token string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(method, "/phishing/l/"+token, nil)
	c.Request.RemoteAddr = "203.0.113.9:1234"
	c.Params = gin.Params{{Key: "token", Value: token}}
	s.handlePhishingLanding(c)
	return w
}

func TestPhishingLandingRateLimitedPerTokenIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newContractDB(t), cfg: config.DefaultConfig()}
	seedPhishingEvent(t, s, "rl-token-a")

	for i := 0; i < phishingLandingLimit; i++ {
		if w := phishingLandingRequest(t, s, http.MethodGet, "rl-token-a"); w.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200 within budget", i+1, w.Code)
		}
	}
	if w := phishingLandingRequest(t, s, http.MethodGet, "rl-token-a"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("over budget: got %d, want 429", w.Code)
	}
}

func TestPhishingLandingRateLimitIsPerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newContractDB(t), cfg: config.DefaultConfig()}
	seedPhishingEvent(t, s, "rl-token-a")
	seedPhishingEvent(t, s, "rl-token-b")

	for i := 0; i < phishingLandingLimit; i++ {
		phishingLandingRequest(t, s, http.MethodGet, "rl-token-a")
	}
	if w := phishingLandingRequest(t, s, http.MethodGet, "rl-token-a"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("token-a over budget: got %d, want 429", w.Code)
	}
	if w := phishingLandingRequest(t, s, http.MethodGet, "rl-token-b"); w.Code != http.StatusOK {
		t.Fatalf("token-b should have independent budget: got %d, want 200", w.Code)
	}
}

func TestPhishingLandingOpenDedup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newContractDB(t), cfg: config.DefaultConfig()}
	seedPhishingEvent(t, s, "dedup-token")

	// First GET records an open.
	phishingLandingRequest(t, s, http.MethodGet, "dedup-token")
	var opens int64
	if err := s.db.Model(&db.PhishingEvent{}).Where("token = ? AND event_type = ?", "dedup-token", "open").Count(&opens).Error; err != nil {
		t.Fatalf("count opens: %v", err)
	}
	if opens != 1 {
		t.Fatalf("first GET: got %d open events, want 1", opens)
	}

	// Repeat GET from the same IP within the dedup window must not duplicate.
	for i := 0; i < 5; i++ {
		phishingLandingRequest(t, s, http.MethodGet, "dedup-token")
	}
	if err := s.db.Model(&db.PhishingEvent{}).Where("token = ? AND event_type = ?", "dedup-token", "open").Count(&opens).Error; err != nil {
		t.Fatalf("count opens: %v", err)
	}
	if opens != 1 {
		t.Fatalf("after repeats: got %d open events, want 1 (dedup failed)", opens)
	}
}
