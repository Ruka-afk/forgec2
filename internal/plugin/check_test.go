package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeWasmCheckPackage scaffolds a WASM package and drops module over its
// entry, so the checker inspects exactly the module under test.
func writeWasmCheckPackage(t *testing.T, name string, module []byte) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: name, Type: "command", Interpreter: "wasm", Dir: dir}); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.wasm"), module, 0o644); err != nil {
		t.Fatalf("write module: %v", err)
	}
	return dir
}

// buildVoidSignatureModule exports memory and run, but run returns nothing:
// the classic mistake of forgetting the packed-i64 reply location.
func buildVoidSignatureModule(t *testing.T) []byte {
	t.Helper()
	mod := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	typ := []byte{0x01, 0x60, 0x02, 0x7F, 0x7F, 0x00} // (i32,i32) -> ()
	fun := []byte{0x01, 0x00}
	mem := []byte{0x01, 0x00, 0x01} // min 1 page
	exp := []byte{0x02}
	exp = append(exp, wasmName("memory")...)
	exp = append(exp, 0x02, 0x00)
	exp = append(exp, wasmName("run")...)
	exp = append(exp, 0x00, 0x00)
	body := []byte{0x00, 0x0B} // no locals, end
	code := append([]byte{0x01}, wasmULEB(uint32(len(body)))...)
	code = append(code, body...)
	for _, s := range []wasmSection{{1, typ}, {3, fun}, {5, mem}, {7, exp}, {10, code}} {
		mod = append(mod, s.id)
		mod = append(mod, wasmULEB(uint32(len(s.buf)))...)
		mod = append(mod, s.buf...)
	}
	return mod
}

// buildWasiImportModule imports wasi_snapshot_preview1.fd_write: the tier is
// deliberately WASI-free, so the checker must flag it before an install does.
func buildWasiImportModule(t *testing.T) []byte {
	t.Helper()
	mod := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	// types: 0 = wasi fd_write, 1 = ABI v1 run
	typ := []byte{0x02}
	typ = append(typ, 0x60, 0x04, 0x7F, 0x7F, 0x7F, 0x7F, 0x01, 0x7F)
	typ = append(typ, 0x60, 0x02, 0x7F, 0x7F, 0x01, 0x7E)
	// imports: wasi_snapshot_preview1.fd_write of type 0
	imp := []byte{0x01}
	imp = append(imp, wasmName("wasi_snapshot_preview1")...)
	imp = append(imp, wasmName("fd_write")...)
	imp = append(imp, 0x00, 0x00)
	// functions: one function of type 1
	fun := []byte{0x01, 0x01}
	mem := []byte{0x01, 0x00, 0x01}
	exp := []byte{0x02}
	exp = append(exp, wasmName("memory")...)
	exp = append(exp, 0x02, 0x00)
	exp = append(exp, wasmName("run")...)
	exp = append(exp, 0x00, 0x00)
	body := []byte{0x00, 0x42}
	body = append(body, wasmSLEB64(0)...)
	body = append(body, 0x0B)
	code := append([]byte{0x01}, wasmULEB(uint32(len(body)))...)
	code = append(code, body...)
	for _, s := range []wasmSection{{1, typ}, {2, imp}, {3, fun}, {5, mem}, {7, exp}, {10, code}} {
		mod = append(mod, s.id)
		mod = append(mod, wasmULEB(uint32(len(s.buf)))...)
		mod = append(mod, s.buf...)
	}
	return mod
}

// TestCheckPluginReportsMissingPackage covers the two ways a wrong path fails.
func TestCheckPluginReportsMissingPackage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	if rep := CheckPlugin(CheckOptions{Dir: missing}); !hasDiag(rep, DiagError, "package.missing_dir") || rep.OK() {
		t.Fatalf("missing directory must be an error, got %v", rep.Diagnostics)
	}
	empty := t.TempDir()
	if rep := CheckPlugin(CheckOptions{Dir: empty}); !hasDiag(rep, DiagError, "package.manifest_missing") || rep.OK() {
		t.Fatalf("directory without a manifest must be an error, got %v", rep.Diagnostics)
	}
}

// TestCheckPluginDetectsDigestMismatch proves the checker reuses the server's
// trust chain: a package that changed after signing is refused, with the
// digests in the message.
func TestCheckPluginDetectsDigestMismatch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh", Dir: dir}); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	digest, err := PackageDigest(dir, nil, dir)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	manifestPath := filepath.Join(dir, "manifest.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, append(raw, []byte("\ndigest: "+digest+"\n")...), 0o644); err != nil {
		t.Fatalf("stamp digest: %v", err)
	}
	// A stamped and untouched package must pass the static checks (the digest
	// matches; only the signature is absent, which is a warning).
	if rep := CheckPlugin(CheckOptions{Dir: dir}); !rep.OK() {
		t.Fatalf("a digest-only package must pass, got %v", rep.Diagnostics)
	}
	// Now change a digested file: the digest no longer describes the package.
	entry := filepath.Join(dir, "main.sh")
	if err := os.WriteFile(entry, []byte("#!/bin/sh\necho tampered\n"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	rep := CheckPlugin(CheckOptions{Dir: dir})
	if rep.OK() || !hasDiag(rep, DiagError, "trust.failed") {
		t.Fatalf("a tampered package must fail the trust check, got %v", rep.Diagnostics)
	}
	if !strings.Contains(diagText(rep, "trust.failed"), "digest") {
		t.Errorf("the trust failure must explain the digest mismatch, got %q", diagText(rep, "trust.failed"))
	}
}

// TestCheckPluginRequireSignedRefusesUnsigned mirrors plugins.require_signed:
// an unsigned package is an error under the strict policy, a warning without.
func TestCheckPluginRequireSignedRefusesUnsigned(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh", Dir: dir}); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if rep := CheckPlugin(CheckOptions{Dir: dir}); !rep.OK() || !hasDiag(rep, DiagWarning, "trust.unsigned") {
		t.Fatalf("unsigned package must warn (not fail) by default, got %v", rep.Diagnostics)
	}
	rep := CheckPlugin(CheckOptions{Dir: dir, RequireSigned: true})
	if rep.OK() || !hasDiag(rep, DiagError, "trust.failed") {
		t.Fatalf("require_signed must turn the missing digest into an error, got %v", rep.Diagnostics)
	}
}

// TestCheckPluginDetectsWasmAbiMistakes walks the ABI surface: the good module
// passes, and each broken one gets the diagnostic that names the fix.
func TestCheckPluginDetectsWasmAbiMistakes(t *testing.T) {
	t.Run("good module", func(t *testing.T) {
		dir := writeWasmCheckPackage(t, "wasm-good", buildResultModule(t, `{"success":true,"output":"ok"}`, testResultPages))
		rep := CheckPlugin(CheckOptions{Dir: dir})
		if !hasDiag(rep, DiagInfo, "wasm.exports_ok") {
			t.Fatalf("good module must report wasm.exports_ok, got %v", rep.Diagnostics)
		}
		for _, d := range rep.Diagnostics {
			if strings.HasPrefix(d.Code, "wasm.") && d.Code != "wasm.exports_ok" {
				t.Fatalf("good module produced %s: %s", d.Code, d.Message)
			}
		}
	})
	t.Run("wrong run signature", func(t *testing.T) {
		dir := writeWasmCheckPackage(t, "wasm-void", buildVoidSignatureModule(t))
		rep := CheckPlugin(CheckOptions{Dir: dir})
		if !hasDiag(rep, DiagError, "wasm.bad_abi") {
			t.Fatalf("run must be (i32,i32)->(i64); got %v", rep.Diagnostics)
		}
		if !strings.Contains(diagText(rep, "wasm.bad_abi"), "(i32,i32) -> ()") {
			t.Errorf("bad_abi must report the signature it found, got %q", diagText(rep, "wasm.bad_abi"))
		}
	})
	t.Run("missing memory", func(t *testing.T) {
		dir := writeWasmCheckPackage(t, "wasm-nomem", buildMemorylessModule(t))
		rep := CheckPlugin(CheckOptions{Dir: dir})
		if !hasDiag(rep, DiagError, "wasm.missing_memory") {
			t.Fatalf("a module without memory must fail closed, got %v", rep.Diagnostics)
		}
	})
	t.Run("wasi import", func(t *testing.T) {
		dir := writeWasmCheckPackage(t, "wasm-wasi", buildWasiImportModule(t))
		rep := CheckPlugin(CheckOptions{Dir: dir})
		if !hasDiag(rep, DiagError, "wasm.unexpected_import") {
			t.Fatalf("a WASI import must be flagged, got %v", rep.Diagnostics)
		}
		if !strings.Contains(diagText(rep, "wasm.unexpected_import"), "fd_write") {
			t.Errorf("the import diagnostic must name the import, got %q", diagText(rep, "wasm.unexpected_import"))
		}
	})
}

// TestCheckPluginReportsResultContractViolations runs a real plugin whose
// stdout is not the contract JSON: the run succeeds, the output does not.
func TestCheckPluginReportsResultContractViolations(t *testing.T) {
	if err := probeInterpreter("sh"); err != nil {
		t.Skip("no usable sh on this host")
	}
	dir := filepath.Join(t.TempDir(), "demo-bad-output")
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo-bad-output", Type: "command", Interpreter: "sh", Dir: dir}); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.sh"), []byte("#!/usr/bin/env sh\necho not-json\n"), 0o644); err != nil {
		t.Fatalf("rewrite entry: %v", err)
	}
	rep := CheckPlugin(CheckOptions{Dir: dir, Run: true})
	if rep.OK() || !hasDiag(rep, DiagError, "result.bad_json") {
		t.Fatalf("non-JSON output must be an error, got %v", rep.Diagnostics)
	}
	if !strings.Contains(diagText(rep, "result.bad_json"), "not-json") {
		t.Errorf("bad_json must quote the offending output, got %q", diagText(rep, "result.bad_json"))
	}
}

// TestCheckPluginRejectsBadRunInput keeps -input honest.
func TestCheckPluginRejectsBadRunInput(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	if _, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo", Type: "command", Interpreter: "sh", Dir: dir}); err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	rep := CheckPlugin(CheckOptions{Dir: dir, Run: true, Input: "[]"})
	if !hasDiag(rep, DiagError, "run.bad_input") {
		t.Fatalf("a JSON array is not a valid run input, got %v", rep.Diagnostics)
	}
}

// TestScaffoldedPluginsRunEndToEnd is the full loop for the native tiers the
// host can execute: scaffold, then run through the server's executor and check
// the output contract of each plugin type.
func TestScaffoldedPluginsRunEndToEnd(t *testing.T) {
	interpreter := scaffoldHostInterpreter(t)
	for _, typ := range ValidPluginTypes {
		t.Run(typ, func(t *testing.T) {
			name := "demo-" + typ
			dir := filepath.Join(t.TempDir(), name)
			if _, err := ScaffoldPlugin(ScaffoldOptions{Name: name, Type: typ, Interpreter: interpreter, Dir: dir}); err != nil {
				t.Fatalf("scaffold: %v", err)
			}
			rep := CheckPlugin(CheckOptions{Dir: dir, Run: true})
			if !rep.Ran {
				t.Fatalf("Run was requested but nothing ran: %v", rep.Diagnostics)
			}
			if !rep.OK() {
				t.Fatalf("scaffolded %s plugin failed its run: %v\nstdout=%s\nstderr=%s",
					typ, rep.Diagnostics, rep.Stdout, rep.Stderr)
			}
			if !hasDiag(rep, DiagInfo, "result.ok") {
				t.Errorf("want result.ok, got %v (stdout=%s)", rep.Diagnostics, rep.Stdout)
			}
		})
	}
}

// canCompileWasm probes each candidate toolchain with the scaffold's exact
// flags, so a host with a clang that cannot target wasm still finds zig (and
// CI images without a wasm-capable linker skip instead of failing).
func canCompileWasm(t *testing.T) (cc, target string, ok bool) {
	t.Helper()
	candidates := []struct{ cc, target string }{
		{"clang", "wasm32-unknown-unknown"},
		{"zig cc", "wasm32-freestanding"},
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "probe.c")
	body := `__attribute__((export_name("run"))) unsigned long long run(const unsigned char *p, unsigned len) { (void)p; (void)len; return 0; }`
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	for _, c := range candidates {
		fields := strings.Fields(c.cc)
		if _, err := exec.LookPath(fields[0]); err != nil {
			continue
		}
		args := append(fields[1:], "--target="+c.target, "-Os", "-nostdlib",
			"-Wl,--no-entry", "-Wl,--export=run", "-Wl,--export-memory",
			"-o", filepath.Join(dir, "probe.wasm"), src)
		if err := exec.Command(fields[0], args...).Run(); err == nil {
			return c.cc, c.target, true
		}
	}
	return "", "", false
}

// TestScaffoldedWasmPluginBuildsAndRuns drives the whole WASM loop: scaffold,
// build with build.sh, then check the compiled module — including one real
// execution through the sandboxed runtime.
func TestScaffoldedWasmPluginBuildsAndRuns(t *testing.T) {
	cc, target, ok := canCompileWasm(t)
	if !ok {
		t.Skip("no wasm-capable clang or zig on this host")
	}
	dir := filepath.Join(t.TempDir(), "demo-wasm")
	res, err := ScaffoldPlugin(ScaffoldOptions{Name: "demo-wasm", Type: "command", Interpreter: "wasm", Dir: dir})
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.sh")); err == nil {
		t.Fatalf("wasm scaffold must not write a shell entry")
	}
	if _, err := os.Stat(filepath.Join(dir, "main.sh")); err == nil {
		t.Fatalf("wasm scaffold must not write a shell entry")
	}
	build := exec.Command("sh", filepath.Join(dir, "build.sh"))
	build.Dir = dir
	build.Env = append(os.Environ(), "CC="+cc, "TARGET="+target)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build.sh failed: %v\n%s", err, out)
	}
	if res.BuildHint == "" {
		t.Fatalf("wasm scaffold must return a build hint")
	}
	rep := CheckPlugin(CheckOptions{Dir: dir, Run: true})
	if !rep.OK() {
		t.Fatalf("built wasm plugin failed the check: %v\nstderr=%s", rep.Diagnostics, rep.Stderr)
	}
	if !hasDiag(rep, DiagInfo, "result.ok") {
		t.Errorf("want result.ok, got %v (stdout=%s)", rep.Diagnostics, rep.Stdout)
	}
}

// TestBundledPluginsPassProjectCheck runs the authoring checker over the whole
// shipped corpus with the project key: the authoring path and the strict
// loader must agree on all 52 packages, or one of the two has drifted.
func TestBundledPluginsPassProjectCheck(t *testing.T) {
	root := filepath.Join("..", "..", "plugins")
	manifests, err := filepath.Glob(filepath.Join(root, "*", "*", "manifest.yaml"))
	if err != nil {
		t.Fatalf("glob manifests: %v", err)
	}
	if len(manifests) < 50 {
		t.Fatalf("expected the full bundled plugin set, found %d", len(manifests))
	}
	keys := bundledTrustedKey(t)
	for _, manifestPath := range manifests {
		dir := filepath.Dir(manifestPath)
		rep := CheckPlugin(CheckOptions{Dir: dir, Root: root, TrustedKeys: keys, RequireSigned: true})
		// tier.* findings depend on this host's interpreters (a Windows box
		// whose python is the WindowsApps alias fails the probe); this test is
		// about the packages themselves.
		for _, d := range rep.Diagnostics {
			if d.Level == DiagError && !strings.HasPrefix(d.Code, "tier.") {
				t.Errorf("%s: plugin-check found %s: %s", dir, d.Code, d.Message)
			}
		}
		if !hasDiag(rep, DiagInfo, "trust.verified") {
			t.Errorf("%s: want trust.verified under the project key, got %v", dir, rep.Diagnostics)
		}
	}
}
