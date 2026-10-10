package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// newReportTestServer builds a plain server (no tenant context: legacy
// unscoped visibility) for report tests.
func newReportTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}
}

// TestBuildReportDataMarksPartialOnSectionFailure proves a failed section is
// recorded as partial instead of silently producing an empty report that
// reads as "no data".
func TestBuildReportDataMarksPartialOnSectionFailure(t *testing.T) {
	s := newReportTestServer(t)
	sqlDB, err := s.db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	report := s.buildReportData(c, "2026-01-01", "2026-01-31", []string{"summary", "agents", "tasks", "credentials", "audit"})

	if !reportFlag(report, "partial") {
		t.Fatal("partial flag not set on section failure; an empty report reads as 'no data'")
	}
	failures := reportSectionFailures(report)
	if len(failures) == 0 {
		t.Fatal("no partial failure sections recorded")
	}
	t.Logf("partial failures: %v", failures)
}

// TestReportHTMLBannerOnPartial proves the exported HTML visibly marks an
// incomplete report so a reader cannot mistake a broken section for an
// empty one.
func TestReportHTMLBannerOnPartial(t *testing.T) {
	report := gin.H{
		"title":            "ForgeC2 Action Report",
		"generated":        "2026-01-01 00:00:00",
		"date_range":       "2026-01-01 to 2026-01-31",
		"summary":          gin.H{"total_agents": 3},
		"agents":           []gin.H{},
		"partial":          true,
		"partial_failures": []string{"tasks", "credentials"},
	}
	html := generateHTMLReport(report)
	if !strings.Contains(html, `class="partial-banner"`) {
		t.Fatal("HTML report is missing the partial banner")
	}
	if !strings.Contains(html, "tasks") || !strings.Contains(html, "credentials") {
		t.Fatal("partial banner does not name the failed sections")
	}

	// A complete report must not gain a banner.
	clean := gin.H{
		"title":      "ForgeC2 Action Report",
		"generated":  "2026-01-01 00:00:00",
		"date_range": "2026-01-01 to 2026-01-31",
		"summary":    gin.H{"total_agents": 3},
		"agents":     []gin.H{},
	}
	if strings.Contains(generateHTMLReport(clean), `class="partial-banner"`) {
		t.Fatal("complete report gained a partial banner")
	}
}

// TestReportSectionEndpointFailsLoudly proves a broken section query answers
// 500, not an empty 200 that a client would read as "no rows".
func TestReportSectionEndpointFailsLoudly(t *testing.T) {
	s := newReportTestServer(t)
	sqlDB, err := s.db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	for _, name := range []string{"agents", "tasks", "credentials", "network", "findings", "history"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		switch name {
		case "agents":
			s.handleAPIGetReportAgents(c)
		case "tasks":
			s.handleAPIGetReportTasks(c)
		case "credentials":
			s.handleAPIGetReportCredentials(c)
		case "network":
			s.handleAPIGetReportNetwork(c)
		case "findings":
			s.handleAPIGetReportFindings(c)
		case "history":
			s.handleAPIGetReportHistory(c)
		}
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("report %s section: status=%d body=%s, want 500 on a failed read",
				name, w.Code, w.Body.String())
		}
	}
}
