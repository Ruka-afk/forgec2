package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ── minimal wasm module builder (no toolchain needed in tests) ──────────────

type wasmSection struct {
	id  byte
	buf []byte
}

func wasmULEB(n uint32) []byte {
	var out []byte
	for {
		b := byte(n & 0x7F)
		n >>= 7
		if n != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			return out
		}
	}
}

func wasmName(s string) []byte {
	out := wasmULEB(uint32(len(s)))
	return append(out, s...)
}

// testResultOffset is where the synthetic test modules park their result. The
// real ABI lets a module choose any address; a fixed value keeps the
// hand-encoded module trivial.
// testResultOffset is where the synthetic test modules park their result, well
// above the input region so a large input cannot clobber it. The real ABI lets
// a module choose any address in its own memory.
const testResultOffset = 512 * 1024

// testResultPages is the minimum page count for modules that park a result at
// testResultOffset (9 pages = 576 KiB).
const testResultPages = 9

// buildResultModule produces a module that exports memory and
// run(ptr,len)->(ptr,len) and returns a preloaded JSON result. That is exactly
// the ForgeC2 WASM ABI contract.
func buildResultModule(t *testing.T, resultJSON string, memoryPages uint32) []byte {
	t.Helper()
	mod := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}

	// type: (i32,i32)->(i32,i32)
	typ := []byte{0x01, 0x60, 0x02, 0x7F, 0x7F, 0x02, 0x7F, 0x7F}
	// function: one function of type 0
	fun := []byte{0x01, 0x00}
	// memory: min pages
	mem := append([]byte{0x01, 0x00}, wasmULEB(memoryPages)...)
	// exports: memory(0), run(0)
	exp := []byte{0x02}
	exp = append(exp, wasmName("memory")...)
	exp = append(exp, 0x02, 0x00)
	exp = append(exp, wasmName("run")...)
	exp = append(exp, 0x00, 0x00)
	// data: resultJSON at testResultOffset (well past any small input)
	data := []byte{0x01, 0x00}
	offsetExpr := []byte{0x41}
	offsetExpr = append(offsetExpr, wasmULEB(testResultOffset)...)
	offsetExpr = append(offsetExpr, 0x0B)
	data = append(data, offsetExpr...)
	data = append(data, wasmULEB(uint32(len(resultJSON)))...)
	data = append(data, resultJSON...)
	// code: push (offset, len) then end
	body := []byte{0x00}
	body = append(body, 0x41)
	body = append(body, wasmULEB(testResultOffset)...)
	body = append(body, 0x41)
	body = append(body, wasmULEB(uint32(len(resultJSON)))...)
	body = append(body, 0x0B)
	code := append([]byte{0x01}, wasmULEB(uint32(len(body)))...)
	code = append(code, body...)

	// Canonical section order: type(1), function(3), memory(5), export(7),
	// code(10), data(11).
	for _, s := range []wasmSection{
		{1, typ}, {3, fun}, {5, mem}, {7, exp}, {10, code}, {11, data},
	} {
		mod = append(mod, s.id)
		mod = append(mod, wasmULEB(uint32(len(s.buf)))...)
		mod = append(mod, s.buf...)
	}
	return mod
}

// buildSpinModule returns a module whose run() never returns, for timeout tests.
func buildSpinModule(t *testing.T) []byte {
	t.Helper()
	mod := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	typ := []byte{0x01, 0x60, 0x02, 0x7F, 0x7F, 0x02, 0x7F, 0x7F}
	fun := []byte{0x01, 0x00}
	mem := []byte{0x01, 0x00, 0x01}
	exp := []byte{0x02}
	exp = append(exp, wasmName("memory")...)
	exp = append(exp, 0x02, 0x00)
	exp = append(exp, wasmName("run")...)
	exp = append(exp, 0x00, 0x00)
	// loop { br 0 } ; then (0,0)
	body := []byte{
		0x00,       // no locals
		0x03, 0x40, // loop void
		0x0C, 0x00, // br 0
		0x0B,       // end loop
		0x41, 0x00, // i32.const 0
		0x41, 0x00, // i32.const 0
		0x0B, // end
	}
	code := append([]byte{0x01}, wasmULEB(uint32(len(body)))...)
	code = append(code, body...)

	for _, s := range []wasmSection{{1, typ}, {3, fun}, {5, mem}, {7, exp}, {10, code}} {
		mod = append(mod, s.id)
		mod = append(mod, wasmULEB(uint32(len(s.buf)))...)
		mod = append(mod, s.buf...)
	}
	return mod
}

func writeWasmPlugin(t *testing.T, module []byte) (string, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.wasm"), module, 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}
	return dir, &Manifest{
		Name: "wasm-demo", Version: "1.0.0", Type: "report",
		Entry: "plugin.wasm", Interpreter: "wasm", Timeout: 5,
	}
}

// TestWasmPluginReturnsResult proves the ABI works end to end: host writes the
// input, module returns JSON, and the standard result parser accepts it.
// (Report.Content is a []byte, so JSON carries it base64-encoded.)
func TestWasmPluginReturnsResult(t *testing.T) {
	result := `{"title":"wasm report","format":"markdown","content":"b2s="}`
	dir, m := writeWasmPlugin(t, buildResultModule(t, result, testResultPages))

	e := &executor{}
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{"agent_id": "a1"}, 0)
	if err != nil {
		t.Fatalf("wasm run: %v", err)
	}
	report, err := parseReport(res.Stdout)
	if err != nil {
		t.Fatalf("parse report: %v (stdout=%q)", err, res.Stdout)
	}
	if report.Title != "wasm report" {
		t.Fatalf("title = %q", report.Title)
	}
}

// TestWasmPluginTimeout proves an infinite loop is stopped by the per-run
// deadline instead of pinning a worker.
func TestWasmPluginTimeout(t *testing.T) {
	dir, m := writeWasmPlugin(t, buildSpinModule(t))
	m.Timeout = 1

	e := &executor{}
	start := time.Now()
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 0)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("spinning module was not stopped")
	}
	if res == nil || !res.TimedOut {
		t.Fatalf("expected TimedOut result, got %+v (err=%v)", res, err)
	}
	if elapsed > 30*time.Second {
		t.Fatalf("timeout enforcement took %s", elapsed)
	}
}

// TestWasmPluginRejectsBadModules proves malformed or ABI-incompatible modules
// fail with a clear error instead of being executed.
func TestWasmPluginRejectsBadModules(t *testing.T) {
	cases := []struct {
		name   string
		module []byte
		want   string
	}{
		{"not wasm", []byte("this is not a wasm module"), "invalid wasm module"},
		{"empty", nil, "invalid wasm module"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, m := writeWasmPlugin(t, tc.module)
			if _, err := (&executor{}).run(context.Background(), dir, m, map[string]interface{}{}, 0); err == nil ||
				!strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

// TestWasmManifestRequiresModuleExtension proves the manifest contract: the
// WASM tier only accepts .wasm entry points.
func TestWasmManifestRequiresModuleExtension(t *testing.T) {
	m := &Manifest{Name: "x", Version: "1", Type: "report", Entry: "plugin.py", Interpreter: "wasm"}
	if err := m.Validate(); err == nil {
		t.Fatal("wasm manifest with a .py entry was accepted")
	}
	m.Entry = "plugin.wasm"
	if err := m.Validate(); err != nil {
		t.Fatalf("valid wasm manifest rejected: %v", err)
	}
	if !isWasmInterpreter("WASM") || isWasmInterpreter("python3") {
		t.Fatal("isWasmInterpreter misclassifies interpreters")
	}
}

// TestWasmInputFitsInMemory proves the host grows linear memory for a large
// input instead of failing when the module starts with two pages.
func TestWasmInputFitsInMemory(t *testing.T) {
	result := `{"ok":true}`
	dir, m := writeWasmPlugin(t, buildResultModule(t, result, testResultPages))
	payload := map[string]interface{}{"agent_id": strings.Repeat("a", 200_000)}
	res, err := (&executor{}).run(context.Background(), dir, m, payload, 0)
	if err != nil {
		t.Fatalf("large input rejected: %v", err)
	}
	if string(res.Stdout) != result {
		t.Fatalf("result = %q", res.Stdout)
	}
}

// TestWasmModuleSizeCap proves an oversized module is refused before compile.
func TestWasmModuleSizeCap(t *testing.T) {
	dir := t.TempDir()
	big := make([]byte, maxWasmBinaryBytes+1)
	if err := os.WriteFile(filepath.Join(dir, "plugin.wasm"), big, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	m := &Manifest{Name: "big", Version: "1", Type: "report", Entry: "plugin.wasm", Interpreter: "wasm"}
	_, err := (&executor{}).run(context.Background(), dir, m, map[string]interface{}{}, 0)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized module error = %v", err)
	}
}
