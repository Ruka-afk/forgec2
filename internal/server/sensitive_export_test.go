package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func ensureExportCrypto(t *testing.T, s *Server) {
	t.Helper()
	// mustTenantServer leaves Crypto keys empty: init loot encryption so the
	// User/Credential hooks refuse plaintext storage.
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
}

func seedExportCred(t *testing.T, s *Server, agentID, username string, tenantID uint) {
	t.Helper()
	ensureExportCrypto(t, s)
	e := db.CredentialEntry{AgentID: agentID, Domain: "CORP", Username: username, Password: "s3cret-" + username, Type: "cleartext", Source: "manual", TenantID: tenantID}
	if err := s.db.Create(&e).Error; err != nil {
		t.Fatalf("seed cred: %v", err)
	}
}

func exportCtx(s *Server, t *testing.T, username string, tenantID uint, rawQuery string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	c.Request, _ = http.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	return c, w
}

// TestExportCredentialsScoped proves the CSV export honors tenancy
// (previously a global query: any reader could dump every tenant's vault).
func TestExportCredentialsScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ex-own", 1)
	seedExportCred(t, s, "ex-own", "alice-own", 1)
	seedExportCred(t, s, "ex-foreign-agent", "bob-foreign", 2)

	c, w := exportCtx(s, t, "ex-viewer", 1, "")
	s.handleExportCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "alice-own") {
		t.Fatalf("own credential missing from export")
	}
	if strings.Contains(body, "bob-foreign") || strings.Contains(body, "s3cret-bob-foreign") {
		t.Fatalf("foreign credential leaked into export")
	}
}

// TestExportTasksAndAudit proves the task export succeeds and writes a
// success audit row.
func TestExportTasksAndAudit(t *testing.T) {
	s := mustTenantServer(t)

	c, w := exportCtx(s, t, "ex-task", 1, "")
	s.handleExportTasks(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var ok int64
	s.db.Model(&db.AuditLog{}).Where("action = ? AND success = ?", "task_export", true).Count(&ok)
	if ok == 0 {
		t.Fatalf("no success audit row for task export")
	}
}

// TestHandoverScoped proves the handover bundle contains only the caller's
// tenant (agents manifest).
func TestHandoverScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ex-h-own", 1)
	seedTenantAgent(t, s, "ex-h-foreign", 2)

	c, w := exportCtx(s, t, "ex-hand", 1, "days=1")
	s.handleHandoverExport(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("response is not a zip: %v", err)
	}
	var agentsJSON []byte
	for _, f := range zr.File {
		if f.Name == "agents.json" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open agents.json: %v", err)
			}
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(rc)
			rc.Close()
			agentsJSON = buf.Bytes()
		}
	}
	if agentsJSON == nil {
		t.Fatalf("agents.json missing from bundle")
	}
	var agentsOut []map[string]interface{}
	if err := json.Unmarshal(agentsJSON, &agentsOut); err != nil {
		t.Fatalf("decode agents.json: %v", err)
	}
	if len(agentsOut) != 1 || agentsOut[0]["id"] != "ex-h-own" {
		t.Fatalf("handover agents manifest leaked: %+v", agentsOut)
	}
}

// TestReportExport proves the report HTML export succeeds.
func TestReportExport(t *testing.T) {
	s := mustTenantServer(t)

	c, w := exportCtx(s, t, "ex-report", 1, "start_date=2026-01-01&end_date=2026-12-31&template=executive")
	s.handleAPIExportReportHTML(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
}
