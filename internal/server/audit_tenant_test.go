package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

// TestAuditRecordStampsTenant proves HTTP-path audit entries carry the
// caller's tenant so audit reads can be scoped per tenant.
func TestAuditRecordStampsTenant(t *testing.T) {
	ginSetTestMode(t)
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}
	c, _ := tenantScopedAdminContext(s, t, "audit-admin", 7)

	s.LogAuditRecord(c, "unit_test", "resource", "", "details", true, nil)

	var row db.AuditLog
	if err := s.db.Where("action = ?", "unit_test").First(&row).Error; err != nil {
		t.Fatalf("read audit row: %v", err)
	}
	if row.TenantID != 7 {
		t.Fatalf("audit TenantID = %d, want 7", row.TenantID)
	}
}

// TestAuditRecordWithoutContextStaysUnscoped proves non-HTTP audit paths
// (beacons, workers) keep tenant 0 rather than borrowing a caller's scope.
func TestAuditRecordWithoutContextStaysUnscoped(t *testing.T) {
	ginSetTestMode(t)
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}

	s.LogAuditRecord(nil, "system_test", "resource", "agent-x", "details", true, nil)

	var row db.AuditLog
	if err := s.db.Where("action = ?", "system_test").First(&row).Error; err != nil {
		t.Fatalf("read audit row: %v", err)
	}
	if row.TenantID != 0 {
		t.Fatalf("system audit TenantID = %d, want 0", row.TenantID)
	}
}

// TestAuditReadsScopedToTenant proves both audit read surfaces show the
// caller's own tenant plus legacy (tenant 0) rows, never another tenant's.
func TestAuditReadsScopedToTenant(t *testing.T) {
	ginSetTestMode(t)
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}
	for _, row := range []*db.AuditLog{
		{User: "t1", Action: "a1", TenantID: 1, Success: true},
		{User: "t2", Action: "a2", TenantID: 2, Success: true},
		{User: "legacy", Action: "a0", TenantID: 0, Success: true},
	} {
		if err := s.db.Create(row).Error; err != nil {
			t.Fatalf("seed %s: %v", row.Action, err)
		}
	}

	// Page handler (operator UI).
	c, w := tenantScopedAdminContext(s, t, "tenant2-admin", 2)
	s.handleGetAuditLogs(c)
	if w.Code != http.StatusOK {
		t.Fatalf("page audit status=%d body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Data struct {
			Logs []db.AuditLog `json:"logs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode page audit: %v body=%s", err, w.Body.String())
	}
	assertAuditTenants(t, "page", page.Data.Logs)

	// REST API surface.
	c2, w2 := tenantScopedAdminContext(s, t, "tenant2-admin", 2)
	s.apiListAuditLogs(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("api audit status=%d body=%s", w2.Code, w2.Body.String())
	}
	var rest struct {
		Data []db.AuditLog `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &rest); err != nil {
		t.Fatalf("decode api audit: %v body=%s", err, w2.Body.String())
	}
	assertAuditTenants(t, "api", rest.Data)
}

func assertAuditTenants(t *testing.T, label string, rows []db.AuditLog) {
	t.Helper()
	seen := map[uint]bool{}
	for _, r := range rows {
		seen[r.TenantID] = true
	}
	if seen[1] {
		t.Fatalf("%s audit leaked tenant 1 rows: %+v", label, rows)
	}
	if !seen[2] || !seen[0] {
		t.Fatalf("%s audit missing own tenant (2) or legacy (0): %+v", label, rows)
	}
}

// TestAuditEntryHashIgnoresTenant documents that tenant stamping is not part
// of the tamper-evident chain input, so pre-migration EntryHash values stay
// verifiable.
func TestAuditEntryHashIgnoresTenant(t *testing.T) {
	base := db.AuditLog{User: "u", Action: "a", Resource: "r", AgentID: "ag", IP: "127.0.0.1", Success: true, Details: "d"}
	scoped := base
	scoped.TenantID = 42
	if auditEntryHash(&base) != auditEntryHash(&scoped) {
		t.Fatal("audit chain hash must not depend on TenantID")
	}
}
