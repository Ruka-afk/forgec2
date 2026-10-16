package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJoinRoute(t *testing.T) {
	tests := []struct {
		prefix, path, want string
	}{
		{"", "/health", "/health"},
		{"/", "/agents", "/agents"},
		{"/", "agents", "/agents"},
		{"/agents/:id", "/mimikatz", "/agents/:id/mimikatz"},
		{"/agents/:id", "mimikatz", "/agents/:id/mimikatz"},
		{"/api/v1", "/agents", "/api/v1/agents"},
		{"/api/plugins", "/:id/execute", "/api/plugins/:id/execute"},
		{"/debug/pprof", "/", "/debug/pprof"},
	}
	for _, tt := range tests {
		got := joinRoute(tt.prefix, tt.path)
		if got != tt.want {
			t.Errorf("joinRoute(%q,%q)=%q want %q", tt.prefix, tt.path, got, tt.want)
		}
	}
}

func TestRouteInSpecParamConvert(t *testing.T) {
	spec := map[string]bool{
		"post /agents/{id}/mimikatz": true,
		"get /api/v1/agents":         true,
	}
	if !routeInSpec("post /agents/:id/mimikatz", spec) {
		t.Fatal("expected :id to match {id}")
	}
	if !routeInSpec("get /api/v1/agents", spec) {
		t.Fatal("expected exact match")
	}
	if routeInSpec("post /mimikatz", spec) {
		t.Fatal("relative path must NOT match full OpenAPI path")
	}
}

func TestExtractAgentCommandPrefix(t *testing.T) {
	src := `
func (s *Server) registerAgentCommandRoutes(auth *gin.RouterGroup) {
	agentCmd := auth.Group("/agents/:id")
	agentCmd.POST("/mimikatz", s.handleMimikatz)
	agentCmd.POST("/files/ls", s.handleListDir)
}
`
	callPrefixes := map[string]string{"registerAgentCommandRoutes": "/"}
	out := map[string]bool{}
	extractRoutesFromFile(src, callPrefixes, out)
	if !out["post /agents/:id/mimikatz"] {
		t.Fatalf("missing mimikatz full path, got %v", out)
	}
	if !out["post /agents/:id/files/ls"] {
		t.Fatalf("missing files/ls full path, got %v", out)
	}
	if out["post /mimikatz"] {
		t.Fatal("should not keep bare /mimikatz when group resolved")
	}
}

func TestExtractEmptyGroupRoot(t *testing.T) {
	src := `
func (s *Server) registerPluginRoutes(auth *gin.RouterGroup) {
	pluginsRead := auth.Group("/api/plugins")
	pluginsRead.GET("", s.handlePluginList)
	pluginsWrite := auth.Group("/api/plugins")
	pluginsWrite.POST("", s.handlePluginCreate)
}
`
	callPrefixes := map[string]string{"registerPluginRoutes": "/"}
	out := map[string]bool{}
	extractRoutesFromFile(src, callPrefixes, out)
	if !out["get /api/plugins"] {
		t.Fatalf("expected get /api/plugins for GET(\"\"), got %v", out)
	}
	if !out["post /api/plugins"] {
		t.Fatalf("expected post /api/plugins for POST(\"\"), got %v", out)
	}
}

func TestFindStaleOpenAPI(t *testing.T) {
	backend := map[string]bool{
		"post /agents/:id/mimikatz": true,
		"get /health":               true,
	}
	spec := map[string]bool{
		"post /agents/{id}/mimikatz": true, // live
		"get /health":                true, // live
		"post /mimikatz":             true, // stale relative
		"post /files/ls":             true, // stale relative
	}
	stale := findStaleOpenAPI(backend, spec)
	staleSet := map[string]bool{}
	for _, s := range stale {
		staleSet[s] = true
	}
	if !staleSet["post /mimikatz"] || !staleSet["post /files/ls"] {
		t.Fatalf("expected relative orphans in stale, got %v", stale)
	}
	if staleSet["post /agents/{id}/mimikatz"] || staleSet["get /health"] {
		t.Fatalf("live routes should not be stale, got %v", stale)
	}
}

func TestSchemaCensusRules(t *testing.T) {
	spec := &OpenAPISpec{Paths: map[string]map[string]interface{}{
		"/ok": {
			"post": map[string]interface{}{
				"operationId": "postOk",
				"requestBody": map[string]interface{}{
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{"type": "object"},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"type": "array"},
							},
						},
					},
				},
			},
		},
		"/ref": {
			"post": map[string]interface{}{
				"requestBody": map[string]interface{}{"$ref": "#/components/requestBodies/X"},
				"responses":   map[string]interface{}{"200": map[string]interface{}{"$ref": "#/components/responses/Y"}},
			},
		},
		"/gaps": {
			"post": map[string]interface{}{
				"responses": map[string]interface{}{"200": "not-an-object"},
			},
			"delete": "not-an-operation",
		},
		"/noid": {
			"get":  map[string]interface{}{"operationId": "   "},
			"head": map[string]interface{}{"responses": map[string]interface{}{"204": map[string]interface{}{}}},
		},
	}}

	cov := schemaCensusOf(spec)
	if cov.NoOperationID != 5 {
		t.Errorf("NoOperationID=%d want 5 (/ref post, /gaps post+delete, /noid get+head)", cov.NoOperationID)
	}
	if cov.NoRequestSchema != 1 {
		t.Errorf("NoRequestSchema=%d want 1 (/gaps post has none)", cov.NoRequestSchema)
	}
	if cov.NoResponseSchema != 3 {
		t.Errorf("NoResponseSchema=%d want 3 (/gaps post leaves 200 unschematized, /gaps delete malformed, /noid get has none)", cov.NoResponseSchema)
	}
}

func TestSchemaCensus204CountsDocumented(t *testing.T) {
	spec := &OpenAPISpec{Paths: map[string]map[string]interface{}{
		"/wipe": {
			"delete": map[string]interface{}{
				"operationId": "deleteWipe",
				"responses":   map[string]interface{}{"204": map[string]interface{}{}},
			},
		},
	}}
	cov := schemaCensusOf(spec)
	if cov.NoResponseSchema != 0 || cov.NoOperationID != 0 || cov.NoRequestSchema != 0 {
		t.Fatalf("bare 204 must count as documented, got %+v", cov.schemaCoverage)
	}
}

func TestSchemaCoverageGreater(t *testing.T) {
	base := schemaCoverage{NoOperationID: 3, NoRequestSchema: 4, NoResponseSchema: 5}
	if base.greaterThan(base) {
		t.Fatal("equal counts must not be a regression")
	}
	if !(schemaCoverage{NoOperationID: 4, NoRequestSchema: 4, NoResponseSchema: 5}).greaterThan(base) {
		t.Fatal("any field above baseline must be a regression")
	}
	if (schemaCoverage{NoOperationID: 0, NoRequestSchema: 0, NoResponseSchema: 0}).greaterThan(base) {
		t.Fatal("shrinking counts must pass")
	}
}

func TestParseDeepBaseline(t *testing.T) {
	f := filepath.Join(t.TempDir(), "debt.txt")
	good := "# ceiling, only-shrinking\n\nno_operation_id=12\nno_request_schema=34\nno_response_schema=56\n"
	if err := os.WriteFile(f, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	base, err := parseDeepBaseline(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if base.NoOperationID != 12 || base.NoRequestSchema != 34 || base.NoResponseSchema != 56 {
		t.Fatalf("parsed %+v, want 12/34/56", base)
	}
}

func TestParseDeepBaselineErrors(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, content string
	}{
		{"unknown key", "no_operation_id=1\nno_such_key=2\n"},
		{"bad value", "no_operation_id=many\n"},
		{"missing equals", "no_operation_id\n"},
	}
	for _, tc := range cases {
		f := filepath.Join(dir, tc.name+".txt")
		if err := os.WriteFile(f, []byte(tc.content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := parseDeepBaseline(f); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
	if _, err := parseDeepBaseline(filepath.Join(dir, "missing.txt")); err == nil {
		t.Error("missing file must error")
	}
}
