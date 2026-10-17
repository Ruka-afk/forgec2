package payload

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// mustModulePlan resolves a plan or fails the test.
func mustModulePlan(t *testing.T, cfg *ImplantConfig) agentBuildPlan {
	t.Helper()
	plan, err := resolveAgentPlan(cfg)
	if err != nil {
		t.Fatalf("resolveAgentPlan: %v", err)
	}
	return plan
}

var moduleRegistryAssignmentRe = regexp.MustCompile(`taskHandlers\["([a-z0-9_]+)"\]\s*=`)

// TestAgentModuleCatalogIntegrity holds the catalogue to the facts on disk:
// every registry/exclusive file exists in the embedded agent dir, names are
// unique and shell-safe, and no task type is claimed by two modules.
func TestAgentModuleCatalogIntegrity(t *testing.T) {
	catalog := AgentModuleCatalog()
	if len(catalog) == 0 {
		t.Fatal("empty module catalogue")
	}
	seenNames := map[string]bool{}
	seenFiles := map[string]string{}
	seenTasks := map[string]string{}
	nameRe := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, m := range catalog {
		if !nameRe.MatchString(m.Name) {
			t.Errorf("module name %q is not [a-z][a-z0-9_]*", m.Name)
		}
		if seenNames[m.Name] {
			t.Errorf("duplicate module name %q", m.Name)
		}
		seenNames[m.Name] = true
		if strings.TrimSpace(m.Title) == "" || strings.TrimSpace(m.Description) == "" {
			t.Errorf("module %q needs a title and description", m.Name)
		}
		if m.Special != "" && m.Special != SpecialTransports {
			t.Errorf("module %q has unknown Special %q", m.Name, m.Special)
		}
		if m.Special == SpecialTransports && len(m.Tasks) != 0 {
			t.Errorf("module %q is a transport slice and must not claim tasks", m.Name)
		}
		if len(m.RegistryFiles) == 0 && m.Special == "" {
			t.Errorf("module %q has no registry file", m.Name)
		}
		for _, f := range append(append([]string{}, m.RegistryFiles...), m.ExclusiveFiles...) {
			if prev, ok := seenFiles[f]; ok {
				t.Errorf("file %s is claimed by both %q and %q", f, prev, m.Name)
			}
			seenFiles[f] = m.Name
			if _, err := payloadFS.ReadFile("agent/" + f); err != nil {
				t.Errorf("module %q file %s missing from embedded agent dir: %v", m.Name, f, err)
			}
		}
		for _, task := range m.Tasks {
			if prev, ok := seenTasks[task]; ok {
				t.Errorf("task %q is claimed by both %q and %q", task, prev, m.Name)
			}
			seenTasks[task] = m.Name
		}
		for _, dep := range m.DependsOn {
			if _, ok := LookupAgentModule(dep); !ok {
				t.Errorf("module %q depends on unknown module %q", m.Name, dep)
			}
		}
	}
}

// TestAgentModuleCatalogMatchesRegistry pins the catalogue against the actual
// registry files: each module's Tasks must equal the set of taskHandlers
// assignments in its registry file(s), module registry files must not
// re-register core tasks, and the core registry must not re-register any
// module task.
func TestAgentModuleCatalogMatchesRegistry(t *testing.T) {
	catalog := AgentModuleCatalog()
	claimed := map[string]string{} // task -> module

	// Core keys: the map literal in task_registry.go and RegisterTask specs in
	// task_registry_v2.go.
	coreSrc, err := payloadFS.ReadFile("agent/task_registry.go")
	if err != nil {
		t.Fatalf("read core registry: %v", err)
	}
	coreKeys := map[string]bool{}
	for _, m := range regexp.MustCompile(`"([a-z0-9_]+)"\s*:\s*handle`).FindAllStringSubmatch(string(coreSrc), -1) {
		coreKeys[m[1]] = true
	}
	v2Src, err := payloadFS.ReadFile("agent/task_registry_v2.go")
	if err != nil {
		t.Fatalf("read v2 registry: %v", err)
	}
	for _, m := range regexp.MustCompile(`MustSpec\((?:protocol\.TaskType(\w+)|"([a-z0-9_]+)")\)`).FindAllStringSubmatch(string(v2Src), -1) {
		if m[1] != "" {
			coreKeys[strings.ToLower(m[1])] = true // conservative: constant names normalize lower
		}
		if m[2] != "" {
			coreKeys[m[2]] = true
		}
	}

	moduleFiles := map[string]bool{}
	for _, mod := range catalog {
		for _, f := range mod.RegistryFiles {
			moduleFiles[f] = true
		}
		if mod.Special == SpecialTransports {
			continue
		}
		for _, f := range mod.RegistryFiles {
			src, err := payloadFS.ReadFile("agent/" + f)
			if err != nil {
				t.Fatalf("module %q registry file: %v", mod.Name, err)
			}
			found := map[string]bool{}
			for _, m := range moduleRegistryAssignmentRe.FindAllStringSubmatch(string(src), -1) {
				found[m[1]] = true
			}
			want := map[string]bool{}
			for _, task := range mod.Tasks {
				want[task] = true
			}
			for task := range want {
				if !found[task] {
					t.Errorf("module %q registry file %s does not register %q", mod.Name, f, task)
				}
			}
			for task := range found {
				if !want[task] {
					t.Errorf("module %q registry file %s registers undeclared task %q", mod.Name, f, task)
				}
				if coreKeys[task] {
					t.Errorf("task %q is registered by both the core registry and module %q", task, mod.Name)
				}
				if prev, ok := claimed[task]; ok {
					t.Errorf("task %q registered by two modules (%q, %q)", task, prev, mod.Name)
				}
				claimed[task] = mod.Name
			}
		}
	}
	for task := range claimed {
		if coreKeys[task] {
			t.Errorf("task %q lives in both core and module %q", task, claimed[task])
		}
	}
}

// TestNormalizeAgentModules covers the selection semantics: empty means
// everything, unknown names fail, "core" is accepted, and slim is exactly the
// transports_advanced exclusion.
func TestNormalizeAgentModules(t *testing.T) {
	all, err := NormalizeAgentModules(nil, false)
	if err != nil {
		t.Fatalf("nil selection: %v", err)
	}
	if len(all) != len(AgentModuleCatalog()) {
		t.Fatalf("nil selection must include every module, got %d of %d", len(all), len(AgentModuleCatalog()))
	}
	if _, err := NormalizeAgentModules([]string{"no_such_module"}, false); err == nil {
		t.Fatal("unknown module name must fail")
	}
	coreOnly, err := NormalizeAgentModules([]string{"core"}, false)
	if err != nil {
		t.Fatalf("core-only selection: %v", err)
	}
	if len(coreOnly) != 0 {
		t.Fatalf("core-only selection must include no optional modules, got %v", coreOnly)
	}
	slim, err := NormalizeAgentModules(nil, true)
	if err != nil {
		t.Fatalf("slim selection: %v", err)
	}
	for _, name := range slim {
		if name == "transports_advanced" {
			t.Fatal("slim must exclude transports_advanced")
		}
	}
	if len(slim) != len(all)-1 {
		t.Fatalf("slim must exclude exactly transports_advanced: got %d want %d", len(slim), len(all)-1)
	}
}

// TestResolveAgentPlanDefaults pins the plan derivation used by every build.
func TestResolveAgentPlanDefaults(t *testing.T) {
	full := mustModulePlan(t, &ImplantConfig{})
	if full.TransportsExcluded || len(full.Excluded) != 0 {
		t.Fatalf("default plan must include everything, got excluded=%v transportsExcluded=%v", full.Excluded, full.TransportsExcluded)
	}
	slim := mustModulePlan(t, &ImplantConfig{Slim: true})
	if !slim.TransportsExcluded {
		t.Fatal("slim must exclude the transports")
	}
	// Excluding transports_advanced explicitly (without Slim) is equivalent.
	var keep []string
	for _, name := range AgentModuleNames() {
		if name != "transports_advanced" {
			keep = append(keep, name)
		}
	}
	explicit := mustModulePlan(t, &ImplantConfig{Modules: keep})
	if !explicit.TransportsExcluded {
		t.Fatal("omitting transports_advanced must exclude the transports")
	}
}

// TestExtractAgentSourcesDropsModules exercises the staged tree: excluded
// modules lose their registry + exclusive files, everything else survives, and
// the generated modules_excluded.go replaces the hand-build default.
func TestExtractAgentSourcesDropsModules(t *testing.T) {
	// Select every module except screen and tun.
	var keep []string
	for _, name := range AgentModuleNames() {
		if name != "screen" && name != "tun" {
			keep = append(keep, name)
		}
	}
	plan := mustModulePlan(t, &ImplantConfig{Modules: keep})

	dir := t.TempDir()
	if err := extractAgentSources(payloadFS, dir, plan); err != nil {
		t.Fatalf("extractAgentSources: %v", err)
	}
	for _, gone := range []string{
		"task_registry_screen.go", "task_registry_tun.go",
		"task_window.go", "window_windows.go", "task_tun.go",
		"task_window_test.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("excluded module file %s must be dropped (err=%v)", gone, err)
		}
	}
	for _, kept := range []string{
		"task_registry.go", "task_registry_v2.go", "task_registry_evasion.go",
		"agent.go", "socks_relay.go",
	} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("non-excluded file %s must survive: %v", kept, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "modules_default.go")); !os.IsNotExist(err) {
		t.Error("modules_default.go must be replaced by modules_excluded.go in staged trees")
	}
	gen, err := os.ReadFile(filepath.Join(dir, "modules_excluded.go"))
	if err != nil {
		t.Fatalf("modules_excluded.go missing: %v", err)
	}
	genSrc := string(gen)
	for _, want := range []string{
		`"screenshot": "screen"`,
		`"tun_start": "tun"`,
		`"window_close": "screen"`,
	} {
		if !strings.Contains(genSrc, want) {
			t.Errorf("generated task map missing %q", want)
		}
	}
	// The included-module list is everything before the task map; verify against
	// that slice so task-map VALUES (": "screen",") don't false-positive.
	head := genSrc
	if idx := strings.Index(genSrc, "var excludedModuleTasks"); idx >= 0 {
		head = genSrc[:idx]
	}
	if strings.Contains(head, `"screen",`) || strings.Contains(head, `"tun",`) {
		t.Error("generated included module list must not contain excluded modules")
	}
	for _, want := range []string{`"evasion",`, `"transports_advanced",`} {
		if !strings.Contains(head, want) {
			t.Errorf("generated included module list missing %q", want)
		}
	}
}

// TestModuleExcludedSourceDeterministic pins the generated source shape and
// byte-stability (same plan → same bytes).
func TestModuleExcludedSourceDeterministic(t *testing.T) {
	plan := mustModulePlan(t, &ImplantConfig{Modules: []string{"core"}})
	first := plan.moduleExcludedSource()
	second := plan.moduleExcludedSource()
	if first != second {
		t.Fatal("moduleExcludedSource must be deterministic")
	}
	if !strings.Contains(first, "var compiledAgentModules = []string{}") {
		t.Error("core-only plan must declare an empty non-nil module list ([]string{}), not a nil one (the agent reads nil as the hand-build fallback)")
	}
	if !strings.Contains(first, "var excludedModuleTasks = map[string]string{") {
		t.Error("core-only plan must map every excluded task")
	}
	if strings.Contains(first, "core\"") {
		t.Error("core is not a catalogue entry and must not appear in the generated list")
	}
	// All-modes plan: every module listed, empty task map.
	full := mustModulePlan(t, &ImplantConfig{})
	src := full.moduleExcludedSource()
	if strings.Contains(src, "excludedModuleTasks = map[string]string{\n") {
		// generated map must be empty: the literal closes immediately.
		if !strings.Contains(src, "var excludedModuleTasks = map[string]string{}\n") {
			t.Error("full-module plan must emit an empty task map")
		}
	}
	for _, name := range AgentModuleNames() {
		if !strings.Contains(src, fmt.Sprintf("%q", name)) {
			t.Errorf("full-module plan must list %q", name)
		}
	}
}

// TestSlimEqualsTransportsModuleExcluded pins the unification: the legacy Slim
// flag stages byte-identical trees to an explicit selection that omits
// transports_advanced.
func TestSlimEqualsTransportsModuleExcluded(t *testing.T) {
	slimDir := t.TempDir()
	if err := extractAgentSources(payloadFS, slimDir, mustModulePlan(t, &ImplantConfig{Slim: true})); err != nil {
		t.Fatalf("slim extract: %v", err)
	}
	var keep []string
	for _, name := range AgentModuleNames() {
		if name != "transports_advanced" {
			keep = append(keep, name)
		}
	}
	modDir := t.TempDir()
	if err := extractAgentSources(payloadFS, modDir, mustModulePlan(t, &ImplantConfig{Modules: keep})); err != nil {
		t.Fatalf("module extract: %v", err)
	}
	slimFiles := listTree(t, slimDir)
	modFiles := listTree(t, modDir)
	if strings.Join(slimFiles, "\n") != strings.Join(modFiles, "\n") {
		t.Fatalf("slim and transports-excluded trees differ:\nslim: %v\nmod:  %v", slimFiles, modFiles)
	}
	for _, rel := range slimFiles {
		if strings.HasSuffix(rel, "strxor.go") {
			continue // strxor is re-randomized per extraction by design
		}
		a, err := os.ReadFile(filepath.Join(slimDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(modDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("file %s differs between slim and transports-excluded extraction", rel)
		}
	}
}

// listTree returns every file under dir as sorted relative slash paths.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

// TestAgentModuleExclusionBuilds is the exhaustive compile gate for the
// module-exclusive file lists: for the full selection, the all-excluded
// (core-only) selection and every single-module exclusion it stages a build
// tree exactly like the generator does and runs a real `go build`. A file in
// an ExclusiveFiles list that something else still references fails here —
// that is the only enforcement that keeps the lists honest.
//
// Opt-in: it stages and links the agent once per case. Enable with
// FORGEC2_AGENT_MODULE_BUILD_TEST=1. Targets come from
// FORGEC2_AGENT_MODULE_BUILD_TARGETS (comma list, default windows/amd64);
// cases from FORGEC2_AGENT_MODULE_BUILD_CASES (comma list, default every
// case). CI runs a reduced case list per platform.
func TestAgentModuleExclusionBuilds(t *testing.T) {
	if os.Getenv("FORGEC2_AGENT_MODULE_BUILD_TEST") == "" {
		t.Skip("set FORGEC2_AGENT_MODULE_BUILD_TEST=1 to run the module exclusion compile gate")
	}
	goCmd, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	rootMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatalf("read root go.mod: %v", err)
	}
	// Reuse the repository's module file + sums so the staged build resolves
	// dependencies from the same (warm) module cache as everything else; only
	// the module name changes and the root module is pulled in by replace.
	stagedMod := moduleLineRe.ReplaceAllString(string(rootMod), "module agent")
	stagedMod += "\nreplace github.com/forgec2/forgec2 => " + filepath.ToSlash(repoRoot) + "\n"
	goSum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatalf("read root go.sum: %v", err)
	}

	type target struct{ goos, goarch string }
	targets := []target{{"windows", "amd64"}}
	if raw := os.Getenv("FORGEC2_AGENT_MODULE_BUILD_TARGETS"); raw != "" {
		targets = targets[:0]
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			bits := strings.SplitN(part, "/", 2)
			if len(bits) != 2 {
				t.Fatalf("bad target %q (want goos/goarch)", part)
			}
			targets = append(targets, target{bits[0], bits[1]})
		}
		if len(targets) == 0 {
			t.Fatal("FORGEC2_AGENT_MODULE_BUILD_TARGETS produced no targets")
		}
	}

	type buildCase struct {
		name string
		keep []string
	}
	cases := []buildCase{{"all", nil}, {"core-only", []string{"core"}}}
	for _, name := range AgentModuleNames() {
		var keep []string
		for _, other := range AgentModuleNames() {
			if other != name {
				keep = append(keep, other)
			}
		}
		cases = append(cases, buildCase{"no-" + name, keep})
	}
	if raw := os.Getenv("FORGEC2_AGENT_MODULE_BUILD_CASES"); raw != "" {
		want := map[string]bool{}
		for _, part := range strings.Split(raw, ",") {
			want[strings.TrimSpace(part)] = true
		}
		filtered := cases[:0]
		for _, c := range cases {
			if want[c.name] {
				filtered = append(filtered, c)
				delete(want, c.name)
			}
		}
		// A typo in the filtered list (CI runs a reduced matrix) must fail, not
		// silently run zero cases and pass.
		if len(want) > 0 {
			unknown := make([]string, 0, len(want))
			for name := range want {
				unknown = append(unknown, name)
			}
			sort.Strings(unknown)
			t.Fatalf("unknown case name(s) in FORGEC2_AGENT_MODULE_BUILD_CASES: %s", strings.Join(unknown, ", "))
		}
		if len(cases) == 0 {
			t.Fatal("FORGEC2_AGENT_MODULE_BUILD_CASES produced no cases")
		}
		cases = filtered
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			plan := mustModulePlan(t, &ImplantConfig{Modules: c.keep})
			dir := t.TempDir()
			if err := extractAgentSources(payloadFS, dir, plan); err != nil {
				t.Fatalf("extractAgentSources: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(stagedMod), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "go.sum"), goSum, 0600); err != nil {
				t.Fatal(err)
			}
			for _, tgt := range targets {
				out := filepath.Join(dir, "out.bin")
				cmd := exec.Command(goCmd, "build", "-o", out, ".")
				cmd.Dir = dir
				cmd.Env = append(os.Environ(),
					"GOOS="+tgt.goos, "GOARCH="+tgt.goarch,
					"GOFLAGS=-mod=mod", "GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0",
				)
				started := time.Now()
				combined, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("go build %s/%s failed after %s: %v\n%s", tgt.goos, tgt.goarch, time.Since(started).Round(time.Second), err, lastLines(string(combined), 40))
				}
				t.Logf("%s/%s OK in %s (%d bytes)", tgt.goos, tgt.goarch, time.Since(started).Round(time.Second), fileSize(out))
			}
		})
	}
}

var moduleLineRe = regexp.MustCompile(`(?m)^module .*$`)

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
