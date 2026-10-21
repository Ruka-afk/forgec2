package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

func newProfileTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Server.DataDir = t.TempDir()
	return &Server{cfg: cfg}
}

func postProfileJSON(t *testing.T, s *Server, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(path, "/validate"):
		s.handleValidateProfile(c)
	case strings.HasSuffix(path, "/import-text"):
		s.handleImportProfileText(c)
	default:
		t.Fatalf("unexpected path %s", path)
	}
	return w
}

const handlerCSProfile = `
set sleeptime "30000";
set jitter "12";
http-post {
  set uri "/beacon";
  set verb "POST";
  client { header "Accept" "*/*"; }
  server { output { base64; } }
}
forgec2 {
  set request_header "X-Test" "one";
}
`

func TestValidateProfileCSReturnsDiagnosticsAndCompiledSummary(t *testing.T) {
	s := newProfileTestServer(t)
	w := postProfileJSON(t, s, "/api/generate/profile/validate", map[string]string{
		"name": "editor_cs", "format": "cs", "content": handlerCSProfile,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Valid       bool `json:"valid"`
			Diagnostics []struct {
				Line     int    `json:"line"`
				Column   int    `json:"column"`
				Severity string `json:"severity"`
				Code     string `json:"code"`
			} `json:"diagnostics"`
			Warnings []string `json:"warnings"`
			Compiled struct {
				Method string   `json:"method"`
				URIs   []string `json:"uris"`
				Sleep  int      `json:"sleep"`
			} `json:"compiled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success || !resp.Data.Valid {
		t.Fatalf("validate response = %s", w.Body.String())
	}
	if resp.Data.Compiled.Method != "POST" || resp.Data.Compiled.Sleep != 30 ||
		len(resp.Data.Compiled.URIs) != 1 || resp.Data.Compiled.URIs[0] != "/beacon" {
		t.Fatalf("compiled = %+v", resp.Data.Compiled)
	}
	if resp.Data.Diagnostics == nil {
		t.Fatal("diagnostics should be an array, not null")
	}
}

func TestValidateProfileCSReturnsSyntaxErrorsAs200(t *testing.T) {
	s := newProfileTestServer(t)
	w := postProfileJSON(t, s, "/api/generate/profile/validate", map[string]string{
		"name": "broken_cs", "format": "cs", "content": "http-get { set uri \"/x\";",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("syntax errors should be 200 for the editor, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Valid       bool `json:"valid"`
			Diagnostics []struct {
				Code string `json:"code"`
			} `json:"diagnostics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Valid {
		t.Fatal("expected valid=false")
	}
	found := false
	for _, d := range resp.Data.Diagnostics {
		if d.Code == "cs.unclosed_block" {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", resp.Data.Diagnostics)
	}
}

func TestImportProfileTextCSSavesSource(t *testing.T) {
	s := newProfileTestServer(t)
	w := postProfileJSON(t, s, "/api/generate/profile/import-text", map[string]string{
		"name": "saved_cs", "format": "cs", "content": handlerCSProfile,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	src, err := payload.ProfileSource(s.implantDataDir(), "saved_cs")
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if strings.TrimSpace(src) != strings.TrimSpace(handlerCSProfile) {
		t.Fatalf("stored source changed:\n%s", src)
	}
}

func TestDeleteBuiltinProfileIsRejected(t *testing.T) {
	s := newProfileTestServer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/generate/profile/default", nil)
	c.Params = gin.Params{{Key: "name", Value: "default"}}
	s.handleDeleteProfile(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
