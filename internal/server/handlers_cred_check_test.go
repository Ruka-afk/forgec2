package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func newCredCheckTestServer(t *testing.T) *Server {
	t.Helper()
	crypto.InitLootEncryption(testStorageKeyHex)
	s := newTasksTestServer(t)
	s.cfg = config.DefaultConfig()
	s.credCheckFuse = newCredCheckFuseTracker()
	return s
}

// TestCredCheckIngestConfirmsVaultEntry verifies a "valid" cred_check result
// marks the matching vault entry (agent + domain + username + password) as
// confirmed, keeps the password out of audit, and never creates new entries.
func TestCredCheckIngestConfirmsVaultEntry(t *testing.T) {
	s := newCredCheckTestServer(t)

	entry := db.CredentialEntry{
		AgentID: "cred-agent-check", Domain: "CORP", Username: "jsmith",
		Password: "P@ssw0rd!", Source: "lsass", Type: "cleartext",
	}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("create entry: %v", err)
	}

	task := db.Task{ID: 55, Type: "cred_check", Command: "jsmith|CORP|P@ssw0rd!|"}
	raw := `{"results":[{"user":"jsmith","status":"valid","error":""}],"summary":{"total":1,"valid":1,"invalid":0,"locked":0,"errors":0}}`

	if !s.parseAndStoreCredCheckResult("cred-agent-check", task, raw) {
		t.Fatal("expected confirmed=true")
	}

	var stored db.CredentialEntry
	if err := s.db.First(&stored, entry.ID).Error; err != nil {
		t.Fatalf("reload entry: %v", err)
	}
	if !stored.Confirmed {
		t.Fatal("expected Confirmed=true")
	}
	if stored.TaskID != 55 {
		t.Fatalf("expected TaskID=55, got %d", stored.TaskID)
	}
	if !strings.Contains(stored.Notes, "validated via credential check") {
		t.Fatalf("expected validation note, got %q", stored.Notes)
	}

	if s.credCheckFuse.tripped("cred-agent-check", "CORP") {
		t.Fatal("fuse must be reset after a valid result")
	}

	// Only the pre-existing entry may exist — cred_check never inserts.
	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ?", "cred-agent-check").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 vault entry, got %d", count)
	}

	// Audit must record the confirmation without the password.
	var logs []db.AuditLog
	if err := s.db.Where("action = ? AND agent_id = ?", "cred_check", "cred-agent-check").Find(&logs).Error; err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Details, "credential confirmed: jsmith@CORP") {
		t.Fatalf("audit details wrong: %q", logs[0].Details)
	}
	if strings.Contains(logs[0].Details, "P@ssw0rd!") {
		t.Fatalf("audit must never embed the password: %q", logs[0].Details)
	}
}

// TestCredCheckIngestUserFormats the result user is normalized like sprays:
// user@domain and domain\\user forms still confirm the vault row.
func TestCredCheckIngestUserFormats(t *testing.T) {
	s := newCredCheckTestServer(t)

	entry := db.CredentialEntry{
		AgentID: "cred-agent-fmt", Domain: "CORP", Username: "jsmith",
		Password: "P@ssw0rd!", Source: "lsass", Type: "cleartext",
	}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("create entry: %v", err)
	}

	cases := []struct{ user string }{
		{"jsmith@CORP.LOCAL"},
		{`CORP\jsmith`},
	}
	for i, tc := range cases {
		userJSON, _ := json.Marshal(tc.user)
		raw := `{"results":[{"user":` + string(userJSON) + `,"status":"valid"}]}`
		task := db.Task{ID: uint(60 + i), Type: "cred_check", Command: "jsmith|CORP|P@ssw0rd!|"}
		if !s.parseAndStoreCredCheckResult("cred-agent-fmt", task, raw) {
			t.Fatalf("case %d: expected confirmation", i)
		}
	}

	var stored db.CredentialEntry
	if err := s.db.First(&stored, entry.ID).Error; err != nil {
		t.Fatalf("reload entry: %v", err)
	}
	if !stored.Confirmed {
		t.Fatal("expected Confirmed=true after both formats")
	}
}

// TestCredCheckIngestFuse verifies invalid/locked results increment the
// per-(agent,domain) fuse, a valid result resets it, and unknown/error
// statuses never touch the fuse.
func TestCredCheckIngestFuse(t *testing.T) {
	s := newCredCheckTestServer(t)

	task := db.Task{ID: 70, Type: "cred_check", Command: "jsmith|CORP|wrong|"}
	rawInvalid := `{"results":[{"user":"jsmith","status":"invalid"}],"summary":{"total":1,"valid":0,"invalid":1,"locked":0,"errors":0}}`
	rawLocked := `{"results":[{"user":"jsmith","status":"locked","error":"account locked"}],"summary":{"total":1,"valid":0,"invalid":0,"locked":1,"errors":0}}`
	rawError := `{"results":[{"user":"jsmith","status":"error","error":"transport failure"}],"summary":{"total":1,"valid":0,"invalid":0,"locked":0,"errors":1}}`
	rawValid := `{"results":[{"user":"jsmith","status":"valid"}],"summary":{"total":1,"valid":1}}`

	for i := 0; i < credCheckFuseMax-1; i++ {
		if s.parseAndStoreCredCheckResult("cred-agent-fuse", task, rawInvalid) {
			t.Fatalf("attempt %d: invalid must not confirm anything", i+1)
		}
		if s.credCheckFuse.tripped("cred-agent-fuse", "CORP") {
			t.Fatalf("attempt %d: fuse must not trip before %d failures", i+1, credCheckFuseMax)
		}
	}
	if s.parseAndStoreCredCheckResult("cred-agent-fuse", task, rawLocked) {
		t.Fatal("locked must not confirm anything")
	}
	if !s.credCheckFuse.tripped("cred-agent-fuse", "CORP") {
		t.Fatal("fuse must trip after 5 failures")
	}

	// Error statuses are not deterministic auth failures: no fuse movement.
	if s.parseAndStoreCredCheckResult("cred-agent-fuse", task, rawError) {
		t.Fatal("error result must not count as confirmation")
	}
	if !s.credCheckFuse.tripped("cred-agent-fuse", "CORP") {
		t.Fatal("error status must leave the fuse untouched")
	}

	// A valid result resets the fuse (nothing in the vault to confirm).
	if s.parseAndStoreCredCheckResult("cred-agent-fuse", task, rawValid) {
		t.Fatal("valid result without a vault row must not confirm")
	}
	if s.credCheckFuse.tripped("cred-agent-fuse", "CORP") {
		t.Fatal("fuse must reset after a valid result")
	}
}

// TestHandleCredCheck exercises the HTTP handler: parameter validation, the
// 429 fuse response, successful dispatch with at-rest encryption, and audit
// without the password.
func TestHandleCredCheck(t *testing.T) {
	s := newCredCheckTestServer(t)

	agent := db.Implant{ID: "cred-agent-h", Hostname: "host-a"}
	if err := s.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	postCredCheck := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "admin")
		c.Set("user", "alice")
		c.Params = gin.Params{{Key: "id", Value: "cred-agent-h"}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/agents/cred-agent-h/cred_check", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		s.handleCredCheck(c)
		return w
	}

	if w := postCredCheck("user=jsmith&domain=CORP"); w.Code != http.StatusBadRequest {
		t.Fatalf("missing password: expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if w := postCredCheck("user=jsmith&password=x"); w.Code != http.StatusBadRequest {
		t.Fatalf("missing domain: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Seeded fuse → 429 without creating or dispatching a task.
	for i := 0; i < credCheckFuseMax; i++ {
		s.credCheckFuse.recordFailure("cred-agent-h", "CORP")
	}
	if w := postCredCheck("user=jsmith&domain=CORP&password=P%40ssw0rd!"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("fused: expected 429, got %d: %s", w.Code, w.Body.String())
	}
	var taskCount int64
	s.db.Model(&db.Task{}).Where("agent_id = ?", "cred-agent-h").Count(&taskCount)
	if taskCount != 0 {
		t.Fatalf("no task may be created while fused, got %d", taskCount)
	}

	// Reset fuse → dispatch succeeds and the task is encrypted at rest.
	s.credCheckFuse.reset("cred-agent-h", "CORP")
	w := postCredCheck("user=jsmith&domain=CORP&password=P%40ssw0rd!&dc=10.0.0.4")
	if w.Code != http.StatusOK {
		t.Fatalf("dispatch: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		TaskID  uint `json:"task_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Success {
		t.Fatalf("invalid dispatch response: %v; body=%s", err, w.Body.String())
	}

	var task db.Task
	if err := s.db.First(&task, resp.TaskID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Type != "cred_check" || task.Command != "jsmith|CORP|P@ssw0rd!|10.0.0.4" {
		t.Fatalf("task mismatch: type=%q command=%q", task.Type, task.Command)
	}

	var raw struct{ Command string }
	if err := s.db.Table(("tasks")).Where("id = ?", resp.TaskID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Command, "FC2ENC:") {
		t.Fatalf("cred_check command must be encrypted at rest, got %q", raw.Command)
	}

	var logs []db.AuditLog
	if err := s.db.Where("action = ? AND agent_id = ?", "cred_check", "cred-agent-h").Find(&logs).Error; err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(logs) < 1 {
		t.Fatalf("expected at least 1 audit entry, got %d", len(logs))
	}
	matched := false
	for _, l := range logs {
		if strings.Contains(l.Details, "Credential check: jsmith@CORP") {
			matched = true
		}
		if strings.Contains(l.Details, "P@ssw0rd!") {
			t.Fatalf("audit must never embed the password: %q", l.Details)
		}
	}
	if !matched {
		t.Fatalf("no audit entry with expected details: %+v", logs)
	}
}