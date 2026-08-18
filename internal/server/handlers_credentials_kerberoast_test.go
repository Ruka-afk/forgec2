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

// TestKerberoastIngestHashcatLines verifies hashcat-mode lines produced by the
// agent's DER converter land in the vault with user/realm taken from the
// account segment, the full line preserved as the crackable hash, and that
// malformed variation (missing etype, empty account segment) are dropped.
func TestKerberoastIngestHashcatLines(t *testing.T) {
	s := newCredentialsTestServer(t)

	line23 := "$krb5tgs$23$*svc_http$CORP.LOCAL$svc_http@CORP.LOCAL*$aabbccddeeff00112233445566778899$00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff0011"
	raw := line23 + "\n" +
		"$krb5tgs$23$*svc_sql$CORP.LOCAL$sql/svc@CORP.LOCAL*$112233445566778899aabbccddeeff00$ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa99\n" +
		"$krb5tgs$24$*svc_weird$CORP.LOCAL$weird/svc@CORP.LOCAL*$112233445566778899aabbccddeeff00$ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100ffeeddccbbaa99\n" +
		"$krb5tgs$23$missing-checksum\n" +
		"$krb5tgs$23$**$checksum$edata2\n" +
		"\n"
	s.parseAndStoreKerberoastResults("cred-agent-krb-hc", raw, 44)

	var entries []db.CredentialEntry
	if err := s.db.Where("agent_id = ? AND source = ?", "cred-agent-krb-hc", "kerberoast").Order("id").Find(&entries).Error; err != nil {
		t.Fatalf("query entries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 vault entries, got %d", len(entries))
	}

	e0 := entries[0]
	if e0.Username != "svc_http" || e0.Domain != "CORP.LOCAL" {
		t.Fatalf("account segment parse wrong: user=%q domain=%q", e0.Username, e0.Domain)
	}
	if e0.Hash != line23 {
		t.Fatalf("full hashcat line not preserved: %q", e0.Hash)
	}
	if !strings.Contains(e0.Notes, "svc_http@CORP.LOCAL") || e0.TaskID != 44 {
		t.Fatalf("entry metadata wrong: %+v", e0)
	}

	e1 := entries[1]
	if e1.Username != "svc_sql" || e1.Domain != "CORP.LOCAL" || !strings.HasPrefix(e1.Notes, "SPN: sql/svc@CORP.LOCAL") {
		t.Fatalf("slashed spn entry wrong: %+v", e1)
	}
	e2 := entries[2]
	if e2.Username != "svc_weird" || e2.Domain != "CORP.LOCAL" {
		t.Fatalf("non-23 etype entry wrong: %+v", e2)
	}

	// Dedup must also work for hashcat lines.
	s.parseAndStoreKerberoastResults("cred-agent-krb-hc", raw, 44)
	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("agent_id = ? AND source = ?", "cred-agent-krb-hc", "kerberoast").Count(&count)
	if count != 3 {
		t.Fatalf("expected 3 entries after re-run, got %d", count)
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