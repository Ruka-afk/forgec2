package plugin

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestWasmExamplePlugin keeps the checked-in example package honest: it loads
// the real manifest from examples/wasm/hello-report, runs the committed
// plugin.wasm through the real executor, and checks that the label travelled
// from params.label into the report title, the (base64-encoded) body and the
// forgec2.log diagnostics. Rebuilding the example without rebuilding the host
// side of the ABI still has to satisfy this test.
func TestWasmExamplePlugin(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "wasm", "hello-report")
	m, err := LoadManifest(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		t.Fatalf("load example manifest: %v", err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("example manifest is invalid: %v", err)
	}

	const label = "hello from the example"
	e := &executor{}
	res, err := e.run(context.Background(), dir, m, map[string]interface{}{
		"params": map[string]interface{}{"label": label},
		"config": map[string]interface{}{},
	}, 0)
	if err != nil {
		t.Fatalf("wasm run: %v", err)
	}

	report, err := parseReport(res.Stdout)
	if err != nil {
		t.Fatalf("parse example report: %v (stdout=%q)", err, res.Stdout)
	}
	if !strings.Contains(report.Title, label) {
		t.Fatalf("report title %q does not carry the label", report.Title)
	}
	if !strings.Contains(string(report.Content), label) {
		t.Fatalf("report body %q does not carry the label", report.Content)
	}
	if !strings.Contains(string(res.Stderr), label) {
		t.Fatalf("forgec2.log diagnostics missing from stderr: %q", res.Stderr)
	}
}
