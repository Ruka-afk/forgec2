package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

// TestASREPRoastIngestStoresHashes verifies hashcat -m 18200 lines emitted by
// the agent's asreproast task land in the vault with user/realm parsed from
// the principal, that failure notes and foreign lines are skipped, the same
// output re-delivery dedups, and the ingest is audited.
func TestASREPRoastIngestStoresHashes(t *testing.T) {
	s := newCredentialsTestServer(t)

	raw := "$krb5asrep$23$svc_roast@CORP.LOCAL:9d8f2ab1ab11c1d1e1f2a2b2c2d2e2f30\n" +
		"$krb5asrep$23$backup@corp.local:112233445566778899aabbccddeeff00\n" +
		"[!] svc_locked: KDC_ERR_PREAUTH_REQUIRED\n" +
		"not-a-roast\n" +
		"$krb5asrep$23$broken@C.I\n" +
		"$krb5asrep$23$svc_roast@CORP.LOCAL:9d8f2ab1ab11c1d1e1f2a2b2c2d2e2f30\n" +
		"\n"
	s.parseAndStoreASREPRoastResults("cred-agent-asrep", raw, 45)

	var entries []db.CredentialEntry
	if err := s.db.Where("agent_id = ? AND source = ?", "cred-agent-asrep", "asreproast").Order("id").Find(&entries).Error; err != nil {
		t.Fatalf("query entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 vault entries, got %d", len(entries))
	}

	e0 := entries[0]
	if e0.Username != "svc_roast" || e0.Domain != "CORP.LOCAL" {
		t.Fatalf("principal parse wrong: user=%q domain=%q", e0.Username, e0.Domain)
	}
	wantHash := "$krb5asrep$23$svc_roast@CORP.LOCAL:9d8f2ab1ab11c1d1e1f2a2b2c2d2e2f30"
	if e0.Hash != wantHash {
		t.Fatalf("full hashcat line not preserved: %q", e0.Hash)
	}
	if e0.Type != "krb_asrep" || e0.Source != "asreproast" || !strings.Contains(e0.Notes, "AS-REP: svc_roast@CORP.LOCAL") || e0.TaskID != 45 {
		t.Fatalf("entry metadata wrong: %+v", e0)
	}
	if e0.Password != "" {
		t.Fatalf("asreproast entries must not carry a password: %q", e0.Password)
	}

	e1 := entries[1]
	if e1.Username != "backup" || e1.Domain != "corp.local" {
		t.Fatalf("lowercase realm parse wrong: user=%q domain=%q", e1.Username, e1.Domain)
	}

	// Re-running the same output must not create duplicates.
	s.parseAndStoreASREPRoastResults("cred-agent-asrep", raw, 45)
	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ? AND source = ?", "cred-agent-asrep", "asreproast").Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 entries after re-run, got %d", count)
	}

	// The ingest must be audited.
	var logs []db.AuditLog
	if err := s.db.Where("action = ? AND agent_id = ?", "credential_ingest", "cred-agent-asrep").Find(&logs).Error; err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(logs))
	}
	if !strings.Contains(logs[0].Details, "stored 2 asreproast hashes") {
		t.Fatalf("audit details wrong: %q", logs[0].Details)
	}
}

// TestASREPRoastIngestSkipsGarbage verifies nothing is stored when every line
// is a failure note, foreign text or an undersized fragment — and no audit is
// written for an empty ingest.
func TestASREPRoastIngestSkipsGarbage(t *testing.T) {
	s := newCredentialsTestServer(t)

	raw := "[!] a: KDC_ERR\n" +
		"empty domain only: $krb5asrep$23$@C:\n" +
		"$krb5asrep$ab$user@C:0123456789abcdef\n" +
		"$krb5asrep$23$user@C:12\n"
	s.parseAndStoreASREPRoastResults("cred-agent-asrep-bad", raw, 0)

	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ?", "cred-agent-asrep-bad").Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 vault entries, got %d", count)
	}
	var logs int64
	s.db.Model(&db.AuditLog{}).Where("action = ? AND agent_id = ?", "credential_ingest", "cred-agent-asrep-bad").Count(&logs)
	if logs != 0 {
		t.Fatalf("expected no audit entry for empty ingest, got %d", logs)
	}
}