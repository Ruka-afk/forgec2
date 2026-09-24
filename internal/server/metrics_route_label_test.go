package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricsRequestDurationUsesRouteTemplate proves the request-duration
// histogram labels the matched route template, not the raw request path.
// Raw paths embed per-request identifiers (payload ids, phishing tokens) and
// created one time series per request on unmatched routes.
func TestMetricsRequestDurationUsesRouteTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mc := NewMetricsCollector(&Server{})
	reg := prometheus.NewRegistry()
	mc.Register(reg)

	r := gin.New()
	r.Use(metricsMiddleware(mc))
	r.GET("/api/v1/agents/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	serve := func(path string) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
	}

	// Same route, different per-request ids -> must collapse to one series.
	serve("/api/v1/agents/aaa-111")
	serve("/api/v1/agents/bbb-222")
	// Unmatched route (NoRoute) -> constant "unmatched" bucket.
	serve("/phishing/l/secret-token-abc")
	serve("/phishing/l/secret-token-xyz")

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var labels []string
	for _, f := range families {
		if f.GetName() != "forgec2_request_duration_seconds" {
			continue
		}
		for _, m := range f.GetMetric() {
			var path string
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "path" {
					path = lp.GetValue()
				}
			}
			labels = append(labels, path)
		}
	}
	if len(labels) != 2 {
		t.Fatalf("want 2 route series (template + unmatched), got %d: %v", len(labels), labels)
	}
	seen := map[string]bool{}
	for _, l := range labels {
		seen[l] = true
		if strings.Contains(l, "aaa-111") || strings.Contains(l, "secret-token") {
			t.Fatalf("raw request path leaked into metric label: %q", l)
		}
	}
	if !seen["/api/v1/agents/:id"] || !seen["unmatched"] {
		t.Fatalf("unexpected route labels: %v", labels)
	}
}
