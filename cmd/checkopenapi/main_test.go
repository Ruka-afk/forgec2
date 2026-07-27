package main

import "testing"

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
