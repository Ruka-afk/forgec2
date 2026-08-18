package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

// TestKerberoastIngestStoresHashes verifies SPN:TGS-hash lines land in the
// credential vault with user/domain extracted from the SPN, that re-delivery
// of the same output does not duplicate rows (regression: the dedup scan used
// to read encrypted hashes), and that the ingest is audited.
func TestKerberoastIngestStoresHashes(t *testing.T) {
	s := newCredentialsTestServer(t)

	raw := "svc/http@CORP.LOCAL:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d\n" +
		"CORP/svc_sql:2b2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d\n" +
		"badline-no-colon\n" +
		"svc/http@CORP.LOCAL:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d\n" +
		"\n"
	s.parseAndStoreKerberoastResults("cred-agent-krb", raw, 43)

	var entries []db.CredentialEntry
	if err := s.db.Where("agent_id = ? AND source = ?", "cred-agent-krb", "kerberoast").Order("id").Find(&entries).Error; err != nil {
		t.Fatalf("query entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 vault entries, got %d", len(entries))
	}

	e0 := entries[0]
	if e0.Username != "svc/http" || e0.Domain != "CORP.LOCAL" {
		t.Fatalf("SPN user@domain parse wrong: user=%q domain=%q", e0.Username, e0.Domain)
	}
	if e0.Hash != "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d" {
		t.Fatalf("hash not stored: %q", e0.Hash)
	}
	if e0.Type != "krb_tgs" || e0.Source != "kerberoast" || !strings.Contains(e0.Notes, "svc/http@CORP.LOCAL") || e0.TaskID != 43 {
		t.Fatalf("entry metadata wrong: %+v", e0)
	}
	if e0.Password != "" {
		t.Fatalf("kerberoast entries must not carry a password: %q", e0.Password)
	}

	e1 := entries[1]
	if e1.Username != "svc_sql" || e1.Domain != "CORP" {
		t.Fatalf("SPN domain/user parse wrong: user=%q domain=%q", e1.Username, e1.Domain)
	}

	// Re-running the same output must not create duplicates.
	s.parseAndStoreKerberoastResults("cred-agent-krb", raw, 43)
	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ? AND source = ?", "cred-agent-krb", "kerberoast").Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 entries after re-run, got %d", count)
	}

	// The ingest must be audited.
	var logs []db.AuditLog
	if err := s.db.Where("action = ? AND agent_id = ?", "credential_ingest", "cred-agent-krb").Find(&logs).Error; err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Details, "stored 2 kerberoast hashes") {
		t.Fatalf("audit details wrong: %q", logs[0].Details)
	}
}

// TestKerberoastIngestSkipsMalformed verifies garbage lines and empty SPN/hash
// fragments are dropped without side effects.
func TestKerberoastIngestSkipsMalformed(t *testing.T) {
	s := newCredentialsTestServer(t)

	raw := "not-a-hash\n" +
		"svc/one@CORP.LOCAL:\n" +
		":1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d\n" +
		"   \n"
	s.parseAndStoreKerberoastResults("cred-agent-krb-bad", raw, 0)

	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ?", "cred-agent-krb-bad").Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 vault entries, got %d", count)
	}
	var logs int64
	s.db.Model(&db.AuditLog{}).Where("action = ? AND agent_id = ?", "credential_ingest", "cred-agent-krb-bad").Count(&logs)
	if logs != 0 {
		t.Fatalf("expected no audit entry for empty ingest, got %d", logs)
	}
}