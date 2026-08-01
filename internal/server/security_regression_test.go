package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestFailStaleAcknowledgedTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newContractDB(t), cfg: config.DefaultConfig()}

	now := time.Now()
	staleAck := now.Add(-40 * time.Minute)
	freshAck := now.Add(-time.Minute)

	mk := func(id string, status string, ack *time.Time) *db.Task {
		task := &db.Task{AgentID: "agent-1", Type: "cmd", Status: status}
		if ack != nil {
			task.AcknowledgedAt = ack
		}
		if err := s.db.Create(task).Error; err != nil {
			t.Fatalf("create task %s: %v", id, err)
		}
		return task
	}

	stale := mk("stale", "running", &staleAck)
	fresh := mk("fresh", "running", &freshAck)
	noAck := mk("no-ack", "running", nil)
	pending := mk("pending", "pending", &staleAck)

	s.failStaleAcknowledgedTasks()

	reload := func(task *db.Task) db.Task {
		var out db.Task
		if err := s.db.First(&out, task.ID).Error; err != nil {
			t.Fatalf("reload task %d: %v", task.ID, err)
		}
		return out
	}

	if got := reload(stale); got.Status != "failed" {
		t.Errorf("stale acknowledged task: status = %q, want failed", got.Status)
	} else if !strings.Contains(got.Error, "no result") {
		t.Errorf("stale acknowledged task: error = %q, want timeout message", got.Error)
	}
	if got := reload(fresh); got.Status != "running" {
		t.Errorf("fresh acknowledged task must stay running, got %q", got.Status)
	}
	if got := reload(noAck); got.Status != "running" {
		t.Errorf("unacknowledged running task must stay running, got %q", got.Status)
	}
	if got := reload(pending); got.Status != "pending" {
		t.Errorf("pending task must stay pending, got %q", got.Status)
	}
}

func TestPhishingLandingEscapesAttackerValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newContractDB(t), cfg: config.DefaultConfig()}

	// The token and email are both attacker-controlled (embedded in the lure).
	token := `tok"><script>alert(1)</script>`
	email := `<script>alert(document.cookie)</script>@example.com`
	if err := s.db.Create(&db.PhishingEvent{
		CampaignID: 1,
		Token:      token,
		Email:      email,
		EventType:  "sent",
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("create phishing event: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	escaped := url.PathEscape(token)
	c.Request, _ = http.NewRequest(http.MethodGet, "/phishing/l/"+escaped, nil)
	c.Params = gin.Params{{Key: "token", Value: token}}

	s.handlePhishingLanding(c)

	if w.Code != http.StatusOK {
		t.Fatalf("landing page: got %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, `<script>alert`) {
		t.Fatalf("raw attacker markup present in landing page:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("email value not HTML-escaped:\n%s", body)
	}
	if !strings.Contains(body, "&#34;") {
		t.Errorf("token not HTML-escaped (quote should become &#34;):\n%s", body)
	}
}

func TestValidateAndSaveConfig_RejectsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Implant.DefaultJitter = -5 // invalid per Validate()
	s := &Server{cfg: cfg}

	// Invalid config: must not be saved, must return 400.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/save", nil)
	if s.validateAndSaveConfig(c, "test") {
		t.Fatal("validateAndSaveConfig accepted an invalid config")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid config: got %d, want 400", w.Code)
	}

	// Valid config: saved to disk and returns 200.
	dir := t.TempDir()
	s.configPath = filepath.Join(dir, "config.yaml")
	s.cfg.Implant.DefaultJitter = 10
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/save", nil)
	if !s.validateAndSaveConfig(c, "test") {
		t.Fatal("validateAndSaveConfig rejected a valid config")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("valid config: got %d, want 200", w.Code)
	}
	if _, err := os.Stat(s.configPath); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}
