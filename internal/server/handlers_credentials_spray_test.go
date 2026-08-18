package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

// TestPasswordSprayIngestStoresValidHits verifies that only status "valid"
// spray results land in the credential vault (with password/domain copied from
// the task command), that duplicates are skipped, and the ingest is audited.
func TestPasswordSprayIngestStoresValidHits(t *testing.T) {
	s := newCredentialsTestServer(t)

	task := db.Task{
		ID:      77,
		Type:    "password_spray",
		Command: "P@ssw0rd!|CORP||500",
	}
	raw := `{"results":[
		{"user":"jsmith","status":"valid"},
		{"user":"svc_sql","status":"valid"},
		{"user":"lockout","status":"locked","error":"account locked"},
		{"user":"nobody","status":"invalid"},
		{"user":"","status":"valid"}
	],"summary":{"total":4,"valid":3,"invalid":1,"locked":1,"errors":0}}`

	stored := s.parseAndStorePasswordSprayResults("cred-agent-spray", task, raw)
	if stored != 2 {
		t.Fatalf("expected 2 stored entries, got %d", stored)
	}

	var entries []db.CredentialEntry
	if err := s.db.Where("agent_id = ?", "cred-agent-spray").Find(&entries).Error; err != nil {
		t.Fatalf("query entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 vault entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Password != "P@ssw0rd!" {
			t.Fatalf("password not copied from command: %q", e.Password)
		}
		if e.Domain != "CORP" {
			t.Fatalf("domain not copied from command: %q", e.Domain)
		}
		if e.Source != "password_spray" || e.Type != "cleartext" || !e.Confirmed {
			t.Fatalf("entry metadata wrong: %+v", e)
		}
		if e.TaskID != 77 {
			t.Fatalf("task id not set: %d", e.TaskID)
		}
	}
	for _, want := range []string{"jsmith", "svc_sql"} {
		found := false
		for _, e := range entries {
			if e.Username == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected entry for %q, got %+v", want, entries)
		}
	}

	// Re-running the same spray result must not duplicate entries.
	if again := s.parseAndStorePasswordSprayResults("cred-agent-spray", task, raw); again != 0 {
		t.Fatalf("expected 0 duplicates on re-run, got %d", again)
	}
	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ?", "cred-agent-spray").Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 entries after re-run, got %d", count)
	}

	// The ingest must be audited.
	var logs []db.AuditLog
	if err := s.db.Where("action = ? AND agent_id = ?", "credential_ingest", "cred-agent-spray").Find(&logs).Error; err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Details, "stored 2 password spray credentials") {
		t.Fatalf("audit details wrong: %q", logs[0].Details)
	}
	if strings.Contains(logs[0].Details, "P@ssw0rd!") {
		t.Fatalf("audit must never embed secrets: %q", logs[0].Details)
	}
}

// TestPasswordSprayIngestUserFormats verifies domain extraction from user
// formats like user@domain and domain\\user when the task command lacks a domain.
func TestPasswordSprayIngestUserFormats(t *testing.T) {
	s := newCredentialsTestServer(t)

	cases := []struct {
		cmd    string
		user   string
		expect string
		domain string
	}{
		{"Pw1|CORP||0", "a@CORP.LOCAL", "a", "CORP"},
		{"Pw1|||0", "b@LAB.LOCAL", "b", "LAB.LOCAL"},
		{"Pw1|||0", `CORP\c`, "c", "CORP"},
	}
	for i, tc := range cases {
		task := db.Task{ID: uint(100 + i), Command: tc.cmd}
		userJSON, _ := json.Marshal(tc.user)
		raw := `{"results":[{"user":` + string(userJSON) + `,"status":"valid"}]}`
		stored := s.parseAndStorePasswordSprayResults("cred-agent-fmt", task, raw)
		if stored != 1 {
			t.Fatalf("case %d: expected 1 stored, got %d", i, stored)
		}
		var e db.CredentialEntry
		if err := s.db.Where("agent_id = ? AND username = ?", "cred-agent-fmt", tc.expect).First(&e).Error; err != nil {
			t.Fatalf("case %d: entry not found: %v", i, err)
		}
		if e.Domain != tc.domain {
			t.Fatalf("case %d: domain = %q, want %q", i, e.Domain, tc.domain)
		}
		if e.Username != tc.expect {
			t.Fatalf("case %d: username = %q, want %q", i, e.Username, tc.expect)
		}
	}
}