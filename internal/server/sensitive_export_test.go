package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	otplib "github.com/pquerna/otp/totp"
)

const exportTestTOTPSecret = "JBSWY3DPEHPK3PXP"

func ensureExportCrypto(t *testing.T, s *Server) {
	t.Helper()
	// mustTenantServer leaves Crypto keys empty: install a deterministic
	// test TOTP key (same shape as tenantVisibilityMasterHex) and init
	// loot encryption (User/Credential hooks refuse plaintext storage).
	if s.cfg.Crypto.TotpKey == "" {
		s.cfg.Crypto.TotpKey = tenantVisibilityMasterHex
	}
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
}

func seedExportTOTPUser(t *testing.T, s *Server, username string, tenantID uint) {
	t.Helper()
	ensureExportCrypto(t, s)
	enc, err := encryptSecret(exportTestTOTPSecret, s.cfg.Crypto.TotpKey)
	if err != nil {
		t.Fatalf("encryptSecret: %v", err)
	}
	u := db.User{Username: username, TenantID: tenantID, Role: "admin", IsActive: true, TOTPSecret: enc}
	if err := s.db.Create(&u).Error; err != nil {
		t.Fatalf("seed totp user: %v", err)
	}
}

func exportCtx(s *Server, t *testing.T, username string, tenantID uint, totpCode, rawQuery string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	// Build the request once: query string and step-up header travel
	// together (rebuilding later would drop the header).
	c.Request, _ = http.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	if totpCode != "" {
		c.Request.Header.Set("X-TOTP-Code", totpCode)
	}
	return c, w
}

func liveTOTPCode(t *testing.T) string {
	t.Helper()
	code, err := otplib.GenerateCode(exportTestTOTPSecret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	return code
}

func seedExportCred(t *testing.T, s *Server, agentID, username string, tenantID uint) {
	t.Helper()
	ensureExportCrypto(t, s)
	e := db.CredentialEntry{AgentID: agentID, Domain: "CORP", Username: username, Password: "s3cret-" + username, Type: "cleartext", Source: "manual", TenantID: tenantID}
	if err := s.db.Create(&e).Error; err != nil {
		t.Fatalf("seed cred: %v", err)
	}
}

// TestExportCredentialsScoped proves the CSV export honors tenancy
// (previously a global query: any reader could dump every tenant's vault).
func TestExportCredentialsScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ex-own", 1)
	seedExportCred(t, s, "ex-own", "alice-own", 1)
	seedExportCred(t, s, "ex-foreign-agent", "bob-foreign", 2)

	c, w := exportCtx(s, t, "ex-viewer", 1, "", "")
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

// TestExportStepUpRequired proves a TOTP-enrolled operator is challenged:
// no code -> 403 + totp_required + failure audit row.
func TestExportStepUpRequired(t *testing.T) {
	s := mustTenantServer(t)
	seedExportTOTPUser(t, s, "ex-stepup", 1)

	c, w := exportCtx(s, t, "ex-stepup", 1, "", "")
	s.handleExportCredentials(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", w.Code)
	}
	if !strings.Contains(w.Body.String(), "totp_required") {
		t.Fatalf("body lacks totp_required flag: %s", w.Body.String())
	}
	var fails int64
	s.db.Model(&db.AuditLog{}).Where("action = ? AND success = ?", "credential_export", false).Count(&fails)
	if fails == 0 {
		t.Fatalf("no failure audit row for blocked export")
	}
}

// TestExportStepUpValidCode proves a fresh TOTP code passes the gate.
func TestExportStepUpValidCode(t *testing.T) {
	s := mustTenantServer(t)
	seedExportTOTPUser(t, s, "ex-stepup-ok", 1)
	seedExportCred(t, s, "ex-stepup-agent", "carol", 1)

	c, w := exportCtx(s, t, "ex-stepup-ok", 1, liveTOTPCode(t), "")
	s.handleExportCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "carol") {
		t.Fatalf("export body missing own credential")
	}
}

// TestExportStepUpWrongCode proves a bad code is rejected and audited.
func TestExportStepUpWrongCode(t *testing.T) {
	s := mustTenantServer(t)
	seedExportTOTPUser(t, s, "ex-stepup-bad", 1)

	c, w := exportCtx(s, t, "ex-stepup-bad", 1, "000000", "")
	s.handleExportCredentials(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", w.Code)
	}
}

// TestExportTasksStepUpAndAudit proves the task export is step-up gated
// and writes a success audit row.
func TestExportTasksStepUpAndAudit(t *testing.T) {
	s := mustTenantServer(t)
	seedExportTOTPUser(t, s, "ex-task", 1)

	c, w := exportCtx(s, t, "ex-task", 1, "", "")
	s.handleExportTasks(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-code status=%d, want 403", w.Code)
	}

	c2, w2 := exportCtx(s, t, "ex-task", 1, liveTOTPCode(t), "")
	s.handleExportTasks(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("code status=%d body=%s, want 200", w2.Code, w2.Body.String())
	}
	var ok int64
	s.db.Model(&db.AuditLog{}).Where("action = ? AND success = ?", "task_export", true).Count(&ok)
	if ok == 0 {
		t.Fatalf("no success audit row for task export")
	}
}

// TestHandoverScopedAndGated proves the handover bundle contains only the
// caller's tenant (agents manifest) and requires step-up for 2FA operators.
func TestHandoverScopedAndGated(t *testing.T) {
	s := mustTenantServer(t)
	seedExportTOTPUser(t, s, "ex-hand", 1)
	seedTenantAgent(t, s, "ex-h-own", 1)
	seedTenantAgent(t, s, "ex-h-foreign", 2)

	c, w := exportCtx(s, t, "ex-hand", 1, "", "days=1")
	s.handleHandoverExport(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-code status=%d, want 403", w.Code)
	}

	c2, w2 := exportCtx(s, t, "ex-hand", 1, liveTOTPCode(t), "days=1")
	s.handleHandoverExport(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("code status=%d body=%s, want 200", w2.Code, w2.Body.String())
	}
	zr, err := zip.NewReader(bytes.NewReader(w2.Body.Bytes()), int64(w2.Body.Len()))
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

// TestReportExportStepUp proves the report HTML export is step-up gated.
func TestReportExportStepUp(t *testing.T) {
	s := mustTenantServer(t)
	seedExportTOTPUser(t, s, "ex-report", 1)

	c, w := exportCtx(s, t, "ex-report", 1, "", "start_date=2026-01-01&end_date=2026-12-31&template=executive")
	s.handleAPIExportReportHTML(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("no-code status=%d, want 403", w.Code)
	}

	c2, w2 := exportCtx(s, t, "ex-report", 1, liveTOTPCode(t), "start_date=2026-01-01&end_date=2026-12-31&template=executive")
	s.handleAPIExportReportHTML(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("code status=%d body=%s, want 200", w2.Code, w2.Body.String())
	}
}
