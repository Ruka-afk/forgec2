package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// These handlers used to log a database failure and fall through to a
// successful 200 with an empty payload. To a client that is indistinguishable
// from "there is genuinely nothing here", which is how a broken read turns
// into a false claim. Every list handler below must answer 5xx instead.

// closedDBServer builds a server whose database handle is already closed, so
// every query fails with "sql: database is closed".
func closedDBServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := &Server{db: testutil.SetupTestDB(t)}
	sqlDB, err := s.db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	return s
}

func assertQueryFailureIs500(t *testing.T, name, target string, invoke func(*Server, *gin.Context)) {
	t.Helper()
	s := closedDBServer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	// Pin the tenant so tenantScope does not fail closed on the dead handle
	// (it would return Where("1=0") and a 200, hiding the query it is meant
	// to scope).
	c.Set(tenantIDContextKey, tenantResolution{tid: 1, ok: true})

	invoke(s, c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("%s: expected 500 on a failed read, got %d; body=%s", name, w.Code, w.Body.String())
	}
	// Guard the specific regression: the body must not be a 200-shaped empty
	// list that a client would read as "no rows".
	if body := w.Body.String(); body == "" {
		t.Fatalf("%s: expected an error body, got empty", name)
	}
}

func TestBOFList_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "bof list", "/api/bof/list", func(s *Server, c *gin.Context) {
		s.handleBOFList(c)
	})
}

func TestBOFRecentResults_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "bof results", "/api/bof/results", func(s *Server, c *gin.Context) {
		s.handleBOFRecentResults(c)
	})
}

func TestScriptList_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "scripts", "/api/scripts", func(s *Server, c *gin.Context) {
		s.handleAPIGetScripts(c)
	})
}

func TestAutoTagRules_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "autotag rules", "/api/autotag/rules", func(s *Server, c *gin.Context) {
		s.handleAutoTagRules(c)
	})
}

func TestOpsecRulesList_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "opsec rules", "/api/opsec/rules", func(s *Server, c *gin.Context) {
		s.handleOpsecRulesList(c)
	})
}

func TestOpsecHistory_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "opsec history", "/opsec/history", func(s *Server, c *gin.Context) {
		s.handleOpsecHistory(c)
	})
}

func TestAutomationRules_QueryFailureIsNotAnEmptyList(t *testing.T) {
	assertQueryFailureIs500(t, "automation rules", "/api/automation/rules", func(s *Server, c *gin.Context) {
		s.handleListAutomationRules(c)
	})
}

func TestBuildLogs_QueryFailureIsNotAnEmptyList(t *testing.T) {
	// The build-logs count runs before getNavStats, so a dead handle short
	// circuits with 500 without reaching the cfg-dependent nav stats.
	assertQueryFailureIs500(t, "build logs", "/builds", func(s *Server, c *gin.Context) {
		s.handleBuildLogs(c)
	})
}
