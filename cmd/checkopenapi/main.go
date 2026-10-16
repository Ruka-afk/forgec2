package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type OpenAPISpec struct {
	Paths map[string]map[string]interface{} `yaml:"paths"`
}

var (
	// name := parent.Group("path") or name = parent.Group("path")
	groupAssignRe = regexp.MustCompile(`(?m)(\w+)\s*:?=\s*([\w.]+)\.Group\(\s*["\x60]([^"\x60]*)["\x60]\s*\)`)
	// recv.METHOD("path" or METHOD("") for group root
	routeCallRe = regexp.MustCompile(`(\w+)\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\(\s*["\x60]([^"\x60]*)["\x60]`)
	// s.router.METHOD("path"
	routerCallRe = regexp.MustCompile(`(?:s\.)?router\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\(\s*["\x60]([^"\x60]*)["\x60]`)
	// func (s *Server) registerFoo(auth *gin.RouterGroup)
	funcRouterParamRe = regexp.MustCompile(`(?m)func\s+\([^)]*\)\s+(\w+)\((\w+)\s+\*gin\.RouterGroup\)`)
	// s.registerFoo(auth)
	registerCallRe = regexp.MustCompile(`s\.(\w+)\((\w+)\)`)
)

// Debug endpoints from the standard library that are intentionally not part of
// the public REST API documentation (e.g. net/http/pprof handlers).
var skippedRoutePrefixes = []string{"/debug/pprof"}

// Core routes that must always appear in OpenAPI (hard CI gate).
var coreRoutes = []string{
	"post /login",
	"get /health",
	"get /ready",
	"get /api/v1/agents",
	"get /api/v1/tasks",
	"get /api/v1/dashboard",
	"get /api/v1/listeners",
	"get /api/v1/credentials",
	"get /api/modules",
	"post /api/modules",
	"delete /api/modules/{name}",
	"get /phishing/templates",
	"post /phishing/templates",
	"get /phishing/campaigns",
	"post /phishing/campaigns",
	"post /phishing/campaigns/{id}/launch",
	"get /phishing/captures",
	"get /integrations",
	"post /integrations",
	"delete /integrations/{id}",
	"get /api/dns/status",
	"post /api/dns/start",
	"post /api/dns/stop",
	"get /notifications",
	"get /api/scripts",
	"post /api/scripts",
	"get /loot",
	"get /api/tags",
	"post /api/tags",
	"get /agents",
	"post /agents/{id}/mimikatz",
	"post /agents/{id}/modules/deploy",
	"get /bloodhound/list",
	"get /bloodhound/status",
	"get /api/automation/rules",
	"get /api/webhooks",
	"get /api/listeners",
	"get /api/listeners/{id}",
}

func main() {
	strict := flag.Bool("strict", false, "fail if any backend route is missing from OpenAPI")
	minCoverage := flag.Float64("min-coverage", 1.0, "minimum fraction of backend routes that must be documented (0-1)")
	listMissing := flag.Bool("list-missing", false, "print all missing routes (no limit)")
	dumpBackend := flag.Bool("dump-backend", false, "print all extracted backend routes and exit")
	listStale := flag.Bool("list-stale", false, "print OpenAPI paths not present in backend (orphans)")
	deepLint := flag.Bool("deep-lint", false, "report schema-coverage gaps (operationId, request/response bodies) with examples")
	deepBaseline := flag.String("deep-baseline", "", "enforce schema-coverage counts against a baseline file (e.g. scripts/openapi-debt.txt); exits 1 if current counts exceed it")
	flag.Parse()

	backendPaths, err := extractBackendPaths("internal/server")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting backend routes: %v\n", err)
		os.Exit(1)
	}

	if *dumpBackend {
		var keys []string
		for k := range backendPaths {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Println(k)
		}
		return
	}

	spec, err := readOpenAPI("api/openapi.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading OpenAPI spec: %v\n", err)
		os.Exit(1)
	}
	if len(spec.Paths) == 0 {
		fmt.Fprintln(os.Stderr, "OpenAPI spec has no paths")
		os.Exit(1)
	}

	specSet := specPaths(spec)
	exitCode := 0

	if *listStale {
		stale := findStaleOpenAPI(backendPaths, specSet)
		sort.Strings(stale)
		fmt.Printf("OpenAPI-only paths (not in backend): %d\n", len(stale))
		for _, s := range stale {
			fmt.Println("  " + s)
		}
		return
	}

	var missingCore []string
	for _, r := range coreRoutes {
		if !routeInSpec(r, specSet) {
			missingCore = append(missingCore, r)
		}
	}
	if len(missingCore) > 0 {
		fmt.Println("CORE OpenAPI paths missing (required):")
		for _, p := range missingCore {
			fmt.Println("  " + p)
		}
		exitCode = 1
	} else {
		fmt.Printf("Core OpenAPI paths: OK (%d)\n", len(coreRoutes))
	}

	missing := findMissing(backendPaths, specSet)
	sort.Strings(missing)
	total := len(backendPaths)
	documented := total - len(missing)
	coverage := 0.0
	if total > 0 {
		coverage = float64(documented) / float64(total)
	}
	fmt.Printf("OpenAPI coverage: %d/%d (%.1f%%)\n", documented, total, coverage*100)

	if coverage < *minCoverage {
		fmt.Printf("Coverage %.1f%% is below minimum %.1f%%\n", coverage*100, *minCoverage*100)
		exitCode = 1
	}

	if *deepLint || *deepBaseline != "" {
		cov := schemaCensusOf(spec)
		if *deepLint {
			fmt.Printf("Schema coverage: no_operation_id=%d no_request_schema=%d no_response_schema=%d\n",
				cov.NoOperationID, cov.NoRequestSchema, cov.NoResponseSchema)
			for _, rule := range deepLintRules {
				for i, e := range cov.examples[rule.key] {
					if i >= deepLintExamplesPerRule {
						fmt.Printf("  %s: ... and %d more\n", rule.key, len(cov.examples[rule.key])-i)
						break
					}
					fmt.Printf("  %s: %s\n", rule.key, e)
				}
			}
		}
		if *deepBaseline != "" {
			base, err := parseDeepBaseline(*deepBaseline)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading schema-coverage baseline: %v\n", err)
				os.Exit(1)
			}
			if cov.greaterThan(base) {
				fmt.Printf("Schema coverage regression against %s: current no_operation_id=%d no_request_schema=%d no_response_schema=%d, baseline %d/%d/%d\n",
					*deepBaseline, cov.NoOperationID, cov.NoRequestSchema, cov.NoResponseSchema,
					base.NoOperationID, base.NoRequestSchema, base.NoResponseSchema)
				exitCode = 1
			} else if !*deepLint {
				fmt.Printf("Schema coverage vs %s: OK\n", *deepBaseline)
			}
		}
	}

	if *strict || *listMissing {
		if len(missing) > 0 {
			if *strict {
				fmt.Println("Routes registered in backend but missing from OpenAPI (strict mode):")
			} else {
				fmt.Println("Routes registered in backend but missing from OpenAPI:")
			}
			limit := len(missing)
			if *strict && !*listMissing {
				limit = 80
			}
			for i, p := range missing {
				if i >= limit {
					fmt.Printf("  ... and %d more\n", len(missing)-limit)
					break
				}
				fmt.Println("  " + p)
			}
			if *strict {
				exitCode = 1
			}
		} else if *strict {
			fmt.Println("All backend routes are documented in the OpenAPI spec.")
		}
	} else if len(missing) > 0 {
		fmt.Printf("Note: %d routes not yet in OpenAPI (use -strict or -list-missing)\n", len(missing))
	}

	if exitCode == 0 {
		fmt.Println("OpenAPI check passed.")
	}
	os.Exit(exitCode)
}

func readOpenAPI(path string) (*OpenAPISpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec OpenAPISpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func specPaths(spec *OpenAPISpec) map[string]bool {
	paths := make(map[string]bool)
	for p, methods := range spec.Paths {
		for method := range methods {
			m := strings.ToLower(method)
			switch m {
			case "get", "post", "put", "delete", "patch", "head", "options":
				paths[m+" "+normalizePath(p)] = true
			}
		}
	}
	return paths
}

func extractBackendPaths(root string) (map[string]bool, error) {
	// Collect call-site prefixes: registerAgentRoutes -> auth -> "/"
	// Built from all .go files so SetupRoutes is included.
	callPrefixes := map[string]string{} // funcName -> group path for first *gin.RouterGroup arg
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// First pass: local group vars on s.router in each file, plus register calls
	globalVarPrefixes := map[string]map[string]string{} // file -> var -> prefix
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		src := string(data)
		varPrefixes := map[string]string{
			"router":   "",
			"s.router": "",
		}
		// Resolve Group assignments iteratively (parent before child)
		for pass := 0; pass < 8; pass++ {
			changed := false
			for _, m := range groupAssignRe.FindAllStringSubmatch(src, -1) {
				name, parent, gpath := m[1], m[2], m[3]
				parentPrefix, ok := resolveParentPrefix(parent, varPrefixes)
				if !ok {
					continue
				}
				full := joinRoute(parentPrefix, gpath)
				if varPrefixes[name] != full {
					varPrefixes[name] = full
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		globalVarPrefixes[path] = varPrefixes

		// Map registerX(y) calls using known vars in this file
		for _, m := range registerCallRe.FindAllStringSubmatch(src, -1) {
			fn, arg := m[1], m[2]
			if pref, ok := varPrefixes[arg]; ok {
				// Prefer more specific (longer) prefixes if multiple calls
				if prev, exists := callPrefixes[fn]; !exists || len(pref) >= len(prev) {
					callPrefixes[fn] = pref
				}
			}
		}
	}

	// Known defaults from SetupRoutes shape
	if _, ok := callPrefixes["registerAPIRoutes"]; !ok {
		callPrefixes["registerAPIRoutes"] = "/api/v1"
	}
	for _, name := range []string{
		"registerAgentRoutes", "registerAgentCommandRoutes", "registerGenerateRoutes",
		"registerListenerRoutes", "registerReconRoutes", "registerDebugRoutes",
		"registerSettingsRoutes", "registerExtendedRoutes", "registerMonitorRoutes",
		"registerDashboardCharts", "registerBOFRoutes", "registerMiscRoutes",
		"registerAutomationRoutes", "registerPluginRoutes", "registerTaskRoutes",
		"registerCredentialRoutes", "registerUserRoutes", "registerCampaignRoutes",
		"registerIntegrationRoutes", "registerDNSRoutes",
	} {
		if _, ok := callPrefixes[name]; !ok {
			callPrefixes[name] = "/"
		}
	}

	paths := make(map[string]bool)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		src := string(data)
		extractRoutesFromFile(src, callPrefixes, paths)
	}
	return paths, nil
}

func resolveParentPrefix(parent string, vars map[string]string) (string, bool) {
	if parent == "s.router" || parent == "router" {
		return "", true
	}
	if p, ok := vars[parent]; ok {
		return p, true
	}
	// parent may be qualified like s.router already handled
	return "", false
}

func extractRoutesFromFile(src string, callPrefixes map[string]string, out map[string]bool) {
	// Split into function bodies approximately by "func "
	// Track RouterGroup params and local Group vars per function.
	type scope struct {
		vars map[string]string
	}

	// Process whole file with a sliding function scope
	// When we see func (s *Server) name(param *gin.RouterGroup), set param prefix
	lines := strings.Split(src, "\n")
	cur := &scope{vars: map[string]string{
		"router":   "",
		"s.router": "",
	}}
	braceDepth := 0
	inFunc := false

	flushRouterCalls := func(line string) {
		for _, m := range routerCallRe.FindAllStringSubmatch(line, -1) {
			method := strings.ToLower(m[1])
			route := normalizePath(m[2])
			key := method + " " + route
			if !skipRoute(key) {
				out[key] = true
			}
		}
	}

	for _, line := range lines {
		// New function?
		if strings.HasPrefix(strings.TrimSpace(line), "func ") {
			// reset scope for top-level funcs (braceDepth will be 0 at func start usually)
			if braceDepth == 0 {
				cur = &scope{vars: map[string]string{
					"router":   "",
					"s.router": "",
				}}
				inFunc = true
				if m := funcRouterParamRe.FindStringSubmatch(line); m != nil {
					fn, param := m[1], m[2]
					pref := callPrefixes[fn]
					// empty string is valid for router root; missing key => assume /
					if _, ok := callPrefixes[fn]; !ok {
						pref = "/"
					}
					cur.vars[param] = pref
				}
			}
		}

		// Track braces roughly
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
		if braceDepth < 0 {
			braceDepth = 0
		}
		if inFunc && braceDepth == 0 && strings.Contains(line, "}") {
			inFunc = false
		}

		// Group assignments
		for _, m := range groupAssignRe.FindAllStringSubmatch(line, -1) {
			name, parent, gpath := m[1], m[2], m[3]
			parentPrefix, ok := resolveParentPrefix(parent, cur.vars)
			if !ok {
				// Try global-ish: auth often means /
				if parent == "auth" {
					parentPrefix = "/"
					ok = true
				} else if parent == "restAPI" || parent == "beaconAPI" || parent == "api" {
					parentPrefix = "/api/v1"
					ok = true
				}
			}
			if ok {
				cur.vars[name] = joinRoute(parentPrefix, gpath)
			}
		}

		// Direct s.router / router calls
		flushRouterCalls(line)

		// Group-relative route calls
		for _, m := range routeCallRe.FindAllStringSubmatch(line, -1) {
			recv, method, rpath := m[1], strings.ToLower(m[2]), m[3]
			if recv == "router" {
				// handled by routerCallRe possibly as bare router
				key := method + " " + normalizePath(rpath)
				if !skipRoute(key) {
					out[key] = true
				}
				continue
			}
			pref, ok := cur.vars[recv]
			if !ok {
				// Heuristic fallbacks
				switch recv {
				case "auth", "agentsRead", "agentsWrite", "agentsDelete",
					"genRead", "genWrite", "listenersRead", "listenersWrite", "listenersDelete",
					"settingsRead", "settingsWrite", "extRead", "extWrite",
					"monRead", "monWrite", "autoRead", "autoWrite",
					"tasksRead", "credsRead", "credsWrite", "credsDelete",
					"userRead", "userWrite", "usersRead", "usersWrite", "usersDelete",
					"campaignsRead", "campaignsWrite", "schedulerRead", "schedulerWrite",
					"notificationsRead", "notificationsWrite", "redirectorRead", "redirectorWrite",
					"rolesRead", "rolesWrite", "collabWrite", "bofRead", "bofWrite",
					"dashRead", "opsecRead", "opsecWrite", "reconRead", "reconWrite",
					"auditRead", "miscWrite", "intelRead", "intelWrite",
					"integrationRead", "integrationWrite", "bhRead", "bhWrite",
					"autoTagRead", "autoTagWrite", "scheduledRead", "scheduledWrite",
					"linkWrite", "groupsWrite", "workflowsRead", "workflowsWrite",
					"phishingRead", "phishingWrite", "cbRead", "cbWrite", "tagsWrite":
					pref = "/"
					ok = true
				case "agentCmd":
					pref = "/agents/:id"
					ok = true
				case "pluginsRead", "pluginsWrite", "pluginsExecute", "pluginsDelete":
					pref = "/api/plugins"
					ok = true
				case "pprofGroup":
					pref = "/debug/pprof"
					ok = true
				case "restAPI", "beaconAPI":
					pref = "/api/v1"
					ok = true
				}
			}
			if !ok {
				// Unresolved: keep relative path so we don't invent wrong prefixes
				key := method + " " + normalizePath(rpath)
				if !skipRoute(key) {
					out[key] = true
				}
				continue
			}
			full := joinRoute(pref, rpath)
			key := method + " " + normalizePath(full)
			if !skipRoute(key) {
				out[key] = true
			}
		}
	}
}

// skipRoute reports whether a registered route should be excluded from the
// OpenAPI documentation coverage check (stdlib debug endpoints).
// route is a "<method> <path>" key.
func skipRoute(route string) bool {
	path := route
	if i := strings.IndexByte(route, ' '); i >= 0 {
		path = route[i+1:]
	}
	for _, p := range skippedRoutePrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func joinRoute(prefix, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return path
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	// Gin joins group + relative; Group("/") + "/agents" => "/agents"
	prefix = strings.TrimSuffix(prefix, "/")
	if path == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	return prefix + path
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// collapse accidental //
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

func findMissing(backend, spec map[string]bool) []string {
	var missing []string
	for route := range backend {
		if routeInSpec(route, spec) {
			continue
		}
		missing = append(missing, route)
	}
	return missing
}

// findStaleOpenAPI returns OpenAPI method+path entries with no matching backend route.
func findStaleOpenAPI(backend, spec map[string]bool) []string {
	// Build backend lookup with both :id and {id} forms
	be := make(map[string]bool, len(backend)*2)
	for r := range backend {
		be[r] = true
		parts := strings.SplitN(r, " ", 2)
		if len(parts) != 2 {
			continue
		}
		method, path := parts[0], parts[1]
		segs := strings.Split(path, "/")
		for i, s := range segs {
			if strings.HasPrefix(s, ":") {
				segs[i] = "{" + strings.TrimPrefix(s, ":") + "}"
			} else if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
				segs[i] = ":" + strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
			}
		}
		be[method+" "+strings.Join(segs, "/")] = true
	}
	var stale []string
	for r := range spec {
		if be[r] {
			continue
		}
		// try colon form of openapi braces
		parts := strings.SplitN(r, " ", 2)
		if len(parts) == 2 {
			method, path := parts[0], parts[1]
			segs := strings.Split(path, "/")
			for i, s := range segs {
				if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
					segs[i] = ":" + strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
				}
			}
			if be[method+" "+strings.Join(segs, "/")] {
				continue
			}
		}
		stale = append(stale, r)
	}
	return stale
}

func routeInSpec(backendRoute string, spec map[string]bool) bool {
	if spec[backendRoute] {
		return true
	}
	parts := strings.SplitN(backendRoute, " ", 2)
	if len(parts) != 2 {
		return false
	}
	method, path := parts[0], parts[1]
	// :param -> {param}
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			segs[i] = "{" + strings.TrimPrefix(s, ":") + "}"
		}
	}
	converted := method + " " + strings.Join(segs, "/")
	if spec[converted] {
		return true
	}
	// Also try matching if OpenAPI has full path and backend still relative (legacy)
	// e.g. backend post /mimikatz vs openapi post /agents/{id}/mimikatz
	// Reverse: if backend is full, already handled.
	return false
}

// Schema-coverage linting: the path-coverage gate above only proves a route is
// documented somewhere. These rules measure contract depth (operationId,
// request body, success response body). Counts only ever shrink as the spec
// matures; scripts/openapi-debt.txt is the ceiling, mirroring the lint
// backlog's only-shrinking rule.

const deepLintExamplesPerRule = 10

type deepLintRule struct {
	key string
}

var deepLintRules = []deepLintRule{
	{"no_operation_id"},
	{"no_request_schema"},
	{"no_response_schema"},
}

type schemaCoverage struct {
	NoOperationID    int `json:"no_operation_id"`
	NoRequestSchema  int `json:"no_request_schema"`
	NoResponseSchema int `json:"no_response_schema"`
}

func (c schemaCoverage) greaterThan(base schemaCoverage) bool {
	return c.NoOperationID > base.NoOperationID ||
		c.NoRequestSchema > base.NoRequestSchema ||
		c.NoResponseSchema > base.NoResponseSchema
}

type schemaCensus struct {
	schemaCoverage
	examples map[string][]string
}

func (s schemaCensus) add(rule, method, path string) {
	s.examples[rule] = append(s.examples[rule], method+" "+normalizePath(path))
}

// schemaCensusOf walks every documented operation and tallies the three
// depth rules. YAML objects arrive via interface{}, so each level is
// type-asserted defensively; malformed shapes count as undocumented.
func schemaCensusOf(spec *OpenAPISpec) schemaCensus {
	cens := schemaCensus{examples: map[string][]string{}}
	paths := make([]string, 0, len(spec.Paths))
	for p := range spec.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		methods := spec.Paths[p]
		names := make([]string, 0, len(methods))
		for m := range methods {
			names = append(names, m)
		}
		sort.Strings(names)
		for _, m := range names {
			lm := strings.ToLower(m)
			if !isDocumentedMethod(lm) {
				continue
			}
			op, ok := methods[m].(map[string]interface{})
			if !ok {
				cens.NoOperationID++
				cens.add("no_operation_id", lm, p)
				cens.NoResponseSchema++
				cens.add("no_response_schema", lm, p)
				continue
			}
			if opID, _ := op["operationId"].(string); strings.TrimSpace(opID) == "" {
				cens.NoOperationID++
				cens.add("no_operation_id", lm, p)
			}
			if lm == "post" || lm == "put" || lm == "patch" {
				rb, _ := op["requestBody"].(map[string]interface{})
				if !bodyHasSchema(rb) {
					cens.NoRequestSchema++
					cens.add("no_request_schema", lm, p)
				}
			}
			if !responseDocumented(op["responses"]) {
				cens.NoResponseSchema++
				cens.add("no_response_schema", lm, p)
			}
		}
	}
	return cens
}

func isDocumentedMethod(m string) bool {
	switch m {
	case "get", "post", "put", "delete", "patch", "head", "options":
		return true
	}
	return false
}

// contentHasSchema reports whether at least one media-type entry documents a
// schema object.
func contentHasSchema(content map[string]interface{}) bool {
	if content == nil {
		return false
	}
	for _, v := range content {
		media, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if schema, has := media["schema"].(map[string]interface{}); has && schema != nil {
			return true
		}
	}
	return false
}

func bodyHasSchema(rb map[string]interface{}) bool {
	if rb == nil {
		return false
	}
	if _, has := rb["$ref"]; has {
		return true
	}
	content, _ := rb["content"].(map[string]interface{})
	return contentHasSchema(content)
}

// responseDocumented checks the first documented success status of
// 200/201/202/206/204. A $ref counts; a content-carrying entry needs at
// least one schema; a bare 204 is a declared empty body and counts as
// documented. No success status at all counts as undocumented.
func responseDocumented(raw interface{}) bool {
	responses, ok := raw.(map[string]interface{})
	if !ok {
		return false
	}
	for _, key := range []string{"200", "201", "202", "206", "204"} {
		entry, ok := responses[key]
		if !ok {
			continue
		}
		resp, ok := entry.(map[string]interface{})
		if !ok {
			return false
		}
		if _, has := resp["$ref"]; has {
			return true
		}
		content, _ := resp["content"].(map[string]interface{})
		if contentHasSchema(content) {
			return true
		}
		if key == "204" {
			return true
		}
		return false
	}
	return false
}

// parseDeepBaseline reads a scripts/openapi-debt.txt-style file
// (key=value lines; blank lines and #-comments permitted).
func parseDeepBaseline(path string) (schemaCoverage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return schemaCoverage{}, err
	}
	var base schemaCoverage
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return schemaCoverage{}, fmt.Errorf("bad baseline line %q", line)
		}
		key := strings.TrimSpace(parts[0])
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return schemaCoverage{}, fmt.Errorf("bad baseline value in %q", line)
		}
		switch key {
		case "no_operation_id":
			base.NoOperationID = n
		case "no_request_schema":
			base.NoRequestSchema = n
		case "no_response_schema":
			base.NoResponseSchema = n
		default:
			return schemaCoverage{}, fmt.Errorf("unknown baseline key %q", key)
		}
	}
	return base, nil
}
