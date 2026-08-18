package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// TestRecordUsageCreatesEntry verifies the RecordUsage helper appends a
// credential_usages row and that zero credentialID is a silent no-op.
func TestRecordUsageCreatesEntry(t *testing.T) {
	s := newCredCheckTestServer(t)

	s.RecordUsage(999, 10, "agent-x", "spray", "ok", "test detail", "alice")
	var count int64
	s.db.Model(&db.CredentialUsage{}).Where("credential_id = ?", 999).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 usage entry, got %d", count)
	}

	var u db.CredentialUsage
	s.db.Where("credential_id = ?", 999).First(&u)
	if u.Action != "spray" || u.Result != "ok" || u.Detail != "test detail" || u.Operator != "alice" {
		t.Fatalf("usage fields wrong: %+v", u)
	}

	// Zero credentialID must not panic or insert.
	s.RecordUsage(0, 0, "", "manual", "ok", "", "")
	s.db.Model(&db.CredentialUsage{}).Where("credential_id = ?", 0).Count(&count)
	if count != 0 {
		t.Fatalf("zero credentialID must not create usage, got %d", count)
	}
}

// TestCredentialLifecycle verifies the lifecycle status computation from a
// vault entry and its latest usage record.
func TestCredentialLifecycle(t *testing.T) {
	now := time.Now()
	confirmed := db.CredentialEntry{Confirmed: true}
	unconfirmed := db.CredentialEntry{Confirmed: false}

	if got := db.CredentialLifecycle(unconfirmed, nil, now); got != "fresh" {
		t.Fatalf("fresh: got %q", got)
	}
	if got := db.CredentialLifecycle(confirmed, nil, now); got != "verified" {
		t.Fatalf("verified: got %q", got)
	}
	expired := db.CredentialEntry{ExpiresAt: now.Add(-time.Hour)}
	if got := db.CredentialLifecycle(expired, nil, now); got != "stale" {
		t.Fatalf("stale expired: got %q", got)
	}
	vOk := &db.CredentialUsage{Action: "verify", Result: "ok"}
	if got := db.CredentialLifecycle(unconfirmed, vOk, now); got != "verified" {
		t.Fatalf("verified via usage: got %q", got)
	}
	sOk := &db.CredentialUsage{Action: "spray", Result: "ok"}
	if got := db.CredentialLifecycle(unconfirmed, sOk, now); got != "used" {
		t.Fatalf("used: got %q", got)
	}
	lOk := &db.CredentialUsage{Action: "lateral", Result: "ok"}
	if got := db.CredentialLifecycle(unconfirmed, lOk, now); got != "used" {
		t.Fatalf("used lateral: got %q", got)
	}
	fail := &db.CredentialUsage{Action: "spray", Result: "fail"}
	if got := db.CredentialLifecycle(unconfirmed, fail, now); got != "stale" {
		t.Fatalf("stale fail: got %q", got)
	}
	locked := &db.CredentialUsage{Action: "spray", Result: "locked"}
	if got := db.CredentialLifecycle(unconfirmed, locked, now); got != "stale" {
		t.Fatalf("stale locked: got %q", got)
	}
}

// TestApiRecordUsage exercises POST /api/credentials/:id/usage.
func TestApiRecordUsage(t *testing.T) {
	s := newCredCheckTestServer(t)

	entry := db.CredentialEntry{AgentID: "agent-usage", Domain: "CORP", Username: "admin", Password: "pass", Type: "cleartext", Source: "manual"}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("create entry: %v", err)
	}

	postUsage := func(id, action, detail string) int {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "admin")
		c.Set("user", "alice")
		c.Params = gin.Params{{Key: "cred_id", Value: id}}
		form := url.Values{"action": {action}, "detail": {detail}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/credentials/"+id+"/usage", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		s.apiRecordUsage(c)
		return w.Code
	}

	if code := postUsage("not-a-number", "manual", ""); code != http.StatusBadRequest {
		t.Fatalf("bad id: expected 400, got %d", code)
	}
	if code := postUsage("99999", "manual", ""); code != http.StatusNotFound {
		t.Fatalf("missing cred: expected 404, got %d", code)
	}

	// Valid request.
	idStr := strconv.FormatUint(uint64(entry.ID), 10)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_role", "admin")
	c.Set("user", "alice")
	c.Params = gin.Params{{Key: "cred_id", Value: idStr}}
	form := url.Values{"action": {"spray"}, "detail": {"tested against dc01"}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/credentials/"+idStr+"/usage", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.apiRecordUsage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("valid request: expected 200, got %d", w.Code)
	}

	// Verify usage row was created.
	var count int64
	s.db.Model(&db.CredentialUsage{}).Where("credential_id = ?", entry.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 usage entry, got %d", count)
	}

	var u db.CredentialUsage
	s.db.Where("credential_id = ?", entry.ID).First(&u)
	if u.Action != "spray" || u.Result != "ok" || u.Operator != "alice" {
		t.Fatalf("usage fields wrong: %+v", u)
	}
}

// TestCredCheckRecordsUsage verifies that a valid cred_check result creates
// a usage ledger entry alongside the confirmation.
func TestCredCheckRecordsUsage(t *testing.T) {
	s := newCredCheckTestServer(t)

	entry := db.CredentialEntry{
		AgentID: "agent-cu", Domain: "CORP", Username: "jsmith",
		Password: "P@ssw0rd!", Source: "lsass", Type: "cleartext",
	}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("create entry: %v", err)
	}

	task := db.Task{ID: 77, Type: "cred_check", Command: "jsmith|CORP|P@ssw0rd!|"}
	raw := `{"results":[{"user":"jsmith","status":"valid","error":""}],"summary":{"total":1,"valid":1,"invalid":0,"locked":0,"errors":0}}`

	if !s.parseAndStoreCredCheckResult("agent-cu", task, raw) {
		t.Fatal("expected confirmed=true")
	}

	var count int64
	s.db.Model(&db.CredentialUsage{}).Where("credential_id = ?", entry.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 usage entry from cred_check, got %d", count)
	}

	var u db.CredentialUsage
	s.db.Where("credential_id = ?", entry.ID).First(&u)
	if u.Action != "verify" || u.Result != "ok" || u.TaskID != 77 {
		t.Fatalf("usage fields wrong: action=%q result=%q task_id=%d", u.Action, u.Result, u.TaskID)
	}
}
