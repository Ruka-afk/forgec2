package plugin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// buildArgEchoModule returns a module whose run(ptr,len)->i64 reports its own
// arguments: whatever the host wrote at the pointer it passed comes back as
// the packed result (ptr | len<<32).
func buildArgEchoModule(t *testing.T, memoryPages uint32) []byte {
	t.Helper()
	mod := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	typ := []byte{0x01, 0x60, 0x02, 0x7F, 0x7F, 0x01, 0x7E}
	fun := []byte{0x01, 0x00}
	mem := append([]byte{0x01, 0x00}, wasmULEB(memoryPages)...)
	exp := []byte{0x02}
	exp = append(exp, wasmName("memory")...)
	exp = append(exp, 0x02, 0x00)
	exp = append(exp, wasmName("run")...)
	exp = append(exp, 0x00, 0x00)
	// ptr | len<<32: local.get 0; i64.extend_i32_u; local.get 1;
	// i64.extend_i32_u; i64.const 32; i64.shl; i64.or; end
	body := []byte{
		0x00,             // no locals
		0x20, 0x00, 0xAD, // local.get 0; i64.extend_i32_u
		0x20, 0x01, 0xAD, // local.get 1; i64.extend_i32_u
		0x42, 0x20, // i64.const 32
		0x86, // i64.shl
		0x84, // i64.or
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

// buildMemorylessModule returns a module that exports run but defines no
// linear memory at all.
func buildMemorylessModule(t *testing.T) []byte {
	t.Helper()
	mod := []byte{0x00, 0x61, 0x73, 0x6D, 0x01, 0x00, 0x00, 0x00}
	typ := []byte{0x01, 0x60, 0x02, 0x7F, 0x7F, 0x01, 0x7E}
	fun := []byte{0x01, 0x00}
	exp := []byte{0x01}
	exp = append(exp, wasmName("run")...)
	exp = append(exp, 0x00, 0x00)
	// i64.const 0; end
	body := []byte{0x00, 0x42, 0x00, 0x0B}
	code := append([]byte{0x01}, wasmULEB(uint32(len(body)))...)
	code = append(code, body...)
	for _, s := range []wasmSection{{1, typ}, {3, fun}, {7, exp}, {10, code}} {
		mod = append(mod, s.id)
		mod = append(mod, wasmULEB(uint32(len(s.buf)))...)
		mod = append(mod, s.buf...)
	}
	return mod
}

// TestWasmInputNeverOverwritesModuleData pins the fix for a real hazard: the
// host used to park the input JSON at a fixed 1024-byte offset — exactly where
// wasm-ld lays out the data segment of a real C/Rust module — so any module
// with static data had it overwritten before run() was even called. The module
// here keeps a sentinel at 1024 and returns it; a large input must not change
// a single byte of it.
func TestWasmInputNeverOverwritesModuleData(t *testing.T) {
	const sentinel = `{"title":"module data survived","format":"text","content":"b2s="}`
	dir, m := writeWasmPlugin(t, buildStaticDataModule(t, sentinel, 1024, testResultPages))

	// Much larger than the sentinel: the old fixed-offset write shredded it.
	big := strings.Repeat("A", 8*1024)
	e := &executor{}
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{"blob": big}, 0)
	if err != nil {
		t.Fatalf("wasm run: %v", err)
	}
	if got := string(res.Stdout); got != sentinel {
		t.Fatalf("module data was overwritten by the input:\n got %q\nwant %q", got, sentinel)
	}
}

// TestWasmInputReachesTheModuleAtThePassedPointer covers the other half of the
// contract: whatever offset the host chooses, the module reads the JSON
// through the pointer argument.
func TestWasmInputReachesTheModuleAtThePassedPointer(t *testing.T) {
	dir, m := writeWasmPlugin(t, buildArgEchoModule(t, 1))
	in := map[string]interface{}{"agent_id": "a2", "blob": strings.Repeat("B", 4096)}

	e := &executor{}
	res, err := e.run(context.Background(), dir, m, in, 0)
	if err != nil {
		t.Fatalf("wasm run: %v", err)
	}
	var round map[string]interface{}
	if err := json.Unmarshal(res.Stdout, &round); err != nil {
		t.Fatalf("module did not echo the input JSON: %v (%q)", err, res.Stdout)
	}
	if round["agent_id"] != "a2" || round["blob"] != in["blob"] {
		t.Fatalf("echoed input mismatch: %v", round)
	}
}

// TestWasmModuleWithoutMemoryFailsClosed proves a module that defines no
// linear memory is refused with an error instead of crashing the host on a nil
// memory.
func TestWasmModuleWithoutMemoryFailsClosed(t *testing.T) {
	dir, m := writeWasmPlugin(t, buildMemorylessModule(t))
	e := &executor{}
	if _, err := e.run(context.Background(), dir, m, map[string]interface{}{}, 0); err == nil {
		t.Fatal("a module without linear memory must be refused")
	}
}
