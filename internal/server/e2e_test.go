package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestE2E_Smoke_HealthEndpoint(t *testing.T) {
	db := testutil.SetupTestDB(t)
	s := &Server{db: db}
	gin.SetMode(gin.TestMode)

	t.Run("health returns ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/health", nil)
		s.handleHealth(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertValidJSON(t, w.Body.Bytes(), "health")
	})

	t.Run("ready returns ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/ready", nil)
		s.handleHealth(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertValidJSON(t, w.Body.Bytes(), "ready")
	})
}

func TestE2E_Smoke_ListEndpoints(t *testing.T) {
	db := testutil.SetupTestDB(t)
	s := &Server{db: db}
	gin.SetMode(gin.TestMode)

	t.Run("list agents returns empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/agents", nil)
		s.handleListAgents(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertKeyExists(t, w.Body.Bytes(), "list agents", "agents")
	})

	t.Run("list listeners returns empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/listeners", nil)
		s.handleListListeners(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertKeyExists(t, w.Body.Bytes(), "list listeners", "data")
	})

	t.Run("dashboard heatmap returns ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/activity-heatmap?range=24h", nil)
		s.handleDashboardActivityHeatmap(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertValidJSON(t, w.Body.Bytes(), "dashboard heatmap")
	})
}
