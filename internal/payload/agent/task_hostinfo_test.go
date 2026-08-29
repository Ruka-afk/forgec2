package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// parseHostInfo runs the handler and decodes the JSON report.
func parseHostInfo(t *testing.T, category, filter string) hostInfoReport {
	t.Helper()
	res := TaskResult{Type: "hostinfo"}
	handleHostInfo(Task{ID: 1, Type: "hostinfo", Command: category, Data: filter}, &res)
	if res.Error != "" {
		t.Fatalf("hostinfo %q: %s", category, res.Error)
	}
	var report hostInfoReport
	if err := json.Unmarshal([]byte(res.Output), &report); err != nil {
		t.Fatalf("decode report: %v (output %.200s)", err, res.Output)
	}
	if len(report.Sections) == 0 {
		t.Fatal("report has no sections")
	}
	return report
}

// TestHostInfoCategoryDispatch pins that a single category returns exactly
// one section and "all" returns every section.
func TestHostInfoCategoryDispatch(t *testing.T) {
	rep := parseHostInfo(t, "security", "")
	if _, ok := rep.Sections["security"]; !ok {
		t.Fatalf("security section missing: %v", keysOf(rep.Sections))
	}
	if len(rep.Sections) != 1 {
		t.Fatalf("single category returned %d sections", len(rep.Sections))
	}

	all := parseHostInfo(t, "all", "")
	for _, want := range hostInfoCategories {
		if _, ok := all.Sections[want]; !ok {
			t.Fatalf("all-report missing section %q (got %v)", want, keysOf(all.Sections))
		}
	}
}

// TestHostInfoInvalidCategory ensures garbage categories are rejected with a
// helpful message instead of an empty sweep.
func TestHostInfoInvalidCategory(t *testing.T) {
	res := TaskResult{}
	handleHostInfo(Task{ID: 2, Type: "hostinfo", Command: "nonsense"}, &res)
	if res.Error == "" || !strings.Contains(res.Error, "unknown category") {
		t.Fatalf("expected unknown-category error, got %q", res.Error)
	}
}

// TestHostInfoSoftwareFilter verifies the keyword filter narrows results and
// is echoed in the report for auditability. On non-Windows the software
// section reports available=false, so the assertion adapts to platform.
func TestHostInfoSoftwareFilter(t *testing.T) {
	unfiltered := parseHostInfo(t, "software", "")
	sec := unfiltered.Sections["software"].(map[string]any)
	if avail, ok := sec["available"].(bool); ok && !avail {
		t.Skip("software enumeration not available on this platform")
	}
	filtered := parseHostInfo(t, "software", "zz-no-such-product-zz")
	fsec := filtered.Sections["software"].(map[string]any)
	count := toInt(fsec["count"])
	if count != 0 {
		t.Fatalf("filter should match nothing, got %d entries", count)
	}
	if fsec["filter"] != "zz-no-such-product-zz" {
		t.Fatalf("filter not echoed: %v", fsec["filter"])
	}
}

// TestHostInfoReportShape pins the wire contract the frontend renders:
// category echo, RFC3339 timestamp, platform tag.
func TestHostInfoReportShape(t *testing.T) {
	rep := parseHostInfo(t, "system", "")
	if rep.Category != "system" {
		t.Fatalf("category = %q", rep.Category)
	}
	if rep.CollectedAt == "" || !strings.Contains(rep.CollectedAt, "T") {
		t.Fatalf("collected_at missing/invalid: %q", rep.CollectedAt)
	}
	if rep.Platform == "" {
		t.Fatal("platform missing")
	}
	sys := rep.Sections["system"].(map[string]any)
	if sys["platform"] == "" && sys["available"] == false {
		t.Skip("no system data on this platform")
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return -1
	}
}
