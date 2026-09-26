package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func newReportExportTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t)}
}

func seedReport(t *testing.T, s *Server, name, format, content string) db.GeneratedReport {
	t.Helper()
	r := db.GeneratedReport{Name: name, Format: format, Content: content, Template: "technical"}
	if err := s.db.Create(&r).Error; err != nil {
		t.Fatalf("seed report: %v", err)
	}
	return r
}

// The Report page's per-row download button targets this endpoint. It used to
// point at /report/:id/download?format=, which no route ever served, so the
// button 404'd.
func TestAPIExportGeneratedReport_StreamsAttachment(t *testing.T) {
	s := newReportExportTestServer(t)
	seeded := seedReport(t, s, "nightly sweep", "html", "<html><body>findings</body></html>")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/report/generated/1/download", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	s.handleAPIExportGeneratedReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	disp := w.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "attachment") {
		t.Errorf("expected an attachment disposition, got %q", disp)
	}
	// The stored name is operator-controlled; only the id goes in the header.
	if !strings.Contains(disp, `filename="report-1.html"`) {
		t.Errorf("unexpected filename in %q (seeded name %q)", disp, seeded.Name)
	}
	if got := w.Body.String(); got != seeded.Content {
		t.Errorf("body mismatch:\n got %q\nwant %q", got, seeded.Content)
	}
}

func TestAPIExportGeneratedReport_FormatDefaultsWhenEmpty(t *testing.T) {
	s := newReportExportTestServer(t)
	seedReport(t, s, "no format", "", "body")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/report/generated/1/download", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	s.handleAPIExportGeneratedReport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if disp := w.Header().Get("Content-Disposition"); !strings.Contains(disp, `filename="report-1.html"`) {
		t.Errorf("expected the html default, got %q", disp)
	}
}

func TestAPIExportGeneratedReport_NotFound(t *testing.T) {
	s := newReportExportTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/report/generated/99/download", nil)
	c.Params = gin.Params{{Key: "id", Value: "99"}}

	s.handleAPIExportGeneratedReport(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a missing report, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestAPIExportGeneratedReport_RejectsNonNumericID(t *testing.T) {
	s := newReportExportTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/report/generated/abc/download", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	s.handleAPIExportGeneratedReport(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a non-numeric id, got %d; body=%s", w.Code, w.Body.String())
	}
}
