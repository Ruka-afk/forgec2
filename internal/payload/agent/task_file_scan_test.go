//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func writeScanTestFile(t *testing.T, root, rel string, data []byte) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return p
}

func decodeFileScanResult(t *testing.T, raw string) fileScanResult {
	t.Helper()
	var got fileScanResult
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	return got
}

func TestParseFileScanOptsDefaultsAndClamp(t *testing.T) {
	opts, err := parseFileScanOpts("/tmp", "*.txt", "")
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if opts.root != "/tmp" || len(opts.patterns) != 1 || opts.patterns[0] != "*.txt" {
		t.Fatalf("root/patterns = %q / %#v", opts.root, opts.patterns)
	}
	if len(opts.ruleList) != 3 || opts.ruleList[0] != "email" || opts.ruleList[1] != "phone" || opts.ruleList[2] != "bank_card" {
		t.Fatalf("default rules = %#v", opts.ruleList)
	}
	if opts.maxFiles != fileScanDefaultMaxFiles || opts.maxFileBytes != fileScanDefaultMaxFileBytes ||
		opts.maxTotalBytes != fileScanDefaultMaxTotalBytes || opts.maxFindings != fileScanDefaultMaxFindings {
		t.Fatalf("default limits = %+v", opts)
	}

	opts, err = parseFileScanOpts("/tmp", "", `{"rules":["all","email"],"max_depth":99,"max_files":99999,"max_file_bytes":99999999,"max_total_bytes":99999999,"max_findings":99999,"timeout_seconds":999,"max_result_bytes":99999999}`)
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}
	if len(opts.ruleList) != len(fileScanRuleOrder) {
		t.Fatalf("all rules = %#v", opts.ruleList)
	}
	if opts.maxDepth != fileScanHardMaxDepth || opts.maxFiles != fileScanHardMaxFiles ||
		opts.maxFileBytes != fileScanHardMaxFileBytes || opts.maxTotalBytes != fileScanHardMaxTotalBytes ||
		opts.maxFindings != fileScanHardMaxFindings || opts.timeout != fileScanHardTimeout {
		t.Fatalf("limits not clamped: %+v", opts)
	}
	if opts.maxResultBytes != fileScanMaxResultBytes {
		t.Fatalf("maxResultBytes = %d", opts.maxResultBytes)
	}
	if _, err := parseFileScanOpts("/tmp", "", `{"rules":["nope"]}`); err == nil {
		t.Fatal("unknown rule must be rejected")
	}
	if _, err := parseFileScanOpts("/tmp", "", `rules=email`); err == nil {
		t.Fatal("non-JSON options must be rejected")
	}
}

func TestRunFileScanDefaultsAndOccurrenceDedup(t *testing.T) {
	root := t.TempDir()
	writeScanTestFile(t, root, "notes.txt", []byte(strings.Join([]string{
		"alice@example.com and alice@example.com",
		"call +1 (415) 555-2671",
		"card 4111 1111 1111 1111",
		"invalid card 4111 1111 1111 1112",
		"date 2026-10-20 and IP 192.168.1.1",
		"ssn 123-45-6789",
		"aws AKIAIOSFODNN7EXAMPLE",
		"-----BEGIN RSA PRIVATE KEY-----",
		"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature1234567890",
		`password = "hunter2!"`,
	}, "\n")))
	writeScanTestFile(t, root, ".hidden.txt", []byte("hidden@example.com"))
	writeScanTestFile(t, root, "binary.dat", []byte{0x00, 0x01, 0x02})
	writeScanTestFile(t, root, "image.png", []byte("alice@example.com"))

	raw, err := runFileScan(fileScanOpts{
		root:           root,
		patterns:       []string{"*"},
		rules:          map[string]bool{"email": true, "phone": true, "bank_card": true},
		ruleList:       []string{"email", "phone", "bank_card"},
		maxDepth:       8,
		maxFiles:       100,
		maxFileBytes:   fileScanDefaultMaxFileBytes,
		maxTotalBytes:  fileScanDefaultMaxTotalBytes,
		maxFindings:    100,
		timeout:        fileScanDefaultTimeout,
		maxResultBytes: fileScanMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("runFileScan: %v", err)
	}
	got := decodeFileScanResult(t, raw)
	if len(got.Findings) != 3 {
		t.Fatalf("findings = %d, want 3: %#v", len(got.Findings), got.Findings)
	}
	byRule := map[string]fileScanFinding{}
	for _, finding := range got.Findings {
		byRule[finding.Rule] = finding
	}
	email := byRule["email"]
	if email.Value != "alice@example.com" || email.Occurrences != 2 || email.Line != 1 {
		t.Errorf("email = %+v", email)
	}
	if byRule["phone"].Value != "+1 (415) 555-2671" {
		t.Errorf("phone = %+v", byRule["phone"])
	}
	if byRule["bank_card"].Value != "4111 1111 1111 1111" {
		t.Errorf("card = %+v", byRule["bank_card"])
	}
	for rule := range byRule {
		if rule != "email" && rule != "phone" && rule != "bank_card" {
			t.Errorf("opt-in rule %q ran by default", rule)
		}
	}
	if got.SkippedBinary != 2 {
		t.Errorf("skipped_binary = %d, want 2", got.SkippedBinary)
	}
	if got.FilesScanned != 1 {
		t.Errorf("files_scanned = %d, want 1", got.FilesScanned)
	}
	if got.Truncated {
		t.Errorf("unexpected truncation: %s", got.TruncatedReason)
	}
}

func TestRunFileScanOptInDetectors(t *testing.T) {
	root := t.TempDir()
	writeScanTestFile(t, root, "secrets.txt", []byte(strings.Join([]string{
		"ssn 123-45-6789",
		"aws AKIAIOSFODNN7EXAMPLE",
		"-----BEGIN RSA PRIVATE KEY-----",
		"jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature1234567890",
		`password = "hunter2!"`,
	}, "\n")))

	raw, err := runFileScan(fileScanOpts{
		root:           root,
		patterns:       []string{"*"},
		rules:          map[string]bool{"ssn": true, "aws_key": true, "private_key": true, "jwt": true, "password": true},
		ruleList:       []string{"ssn", "aws_key", "private_key", "jwt", "password"},
		maxDepth:       8,
		maxFiles:       100,
		maxFileBytes:   fileScanDefaultMaxFileBytes,
		maxTotalBytes:  fileScanDefaultMaxTotalBytes,
		maxFindings:    100,
		timeout:        fileScanDefaultTimeout,
		maxResultBytes: fileScanMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("runFileScan: %v", err)
	}
	got := decodeFileScanResult(t, raw)
	values := map[string]string{}
	for _, finding := range got.Findings {
		values[finding.Rule] = finding.Value
	}
	want := map[string]string{
		"ssn":         "123-45-6789",
		"aws_key":     "AKIAIOSFODNN7EXAMPLE",
		"private_key": "-----BEGIN RSA PRIVATE KEY-----",
		"password":    "hunter2!",
	}
	for rule, value := range want {
		if values[rule] != value {
			t.Errorf("%s = %q, want %q (all=%#v)", rule, values[rule], value, got.Findings)
		}
	}
	if !strings.HasPrefix(values["jwt"], "eyJ") {
		t.Errorf("jwt = %q", values["jwt"])
	}
}

func TestRunFileScanUTF16AndLimits(t *testing.T) {
	root := t.TempDir()
	units := utf16.Encode([]rune("contact utf16@example.com\n"))
	data := []byte{0xff, 0xfe}
	for _, unit := range units {
		data = append(data, byte(unit), byte(unit>>8))
	}
	writeScanTestFile(t, root, "0-utf16.txt", data)
	writeScanTestFile(t, root, "a.txt", []byte("a@example.com\nb@example.com\nc@example.com\n"))

	raw, err := runFileScan(fileScanOpts{
		root:           root,
		patterns:       []string{"*"},
		rules:          map[string]bool{"email": true},
		ruleList:       []string{"email"},
		maxDepth:       8,
		maxFiles:       100,
		maxFileBytes:   fileScanDefaultMaxFileBytes,
		maxTotalBytes:  fileScanDefaultMaxTotalBytes,
		maxFindings:    2,
		timeout:        fileScanDefaultTimeout,
		maxResultBytes: fileScanMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("runFileScan: %v", err)
	}
	got := decodeFileScanResult(t, raw)
	if len(got.Findings) != 2 {
		t.Fatalf("findings = %d, want 2: %#v", len(got.Findings), got.Findings)
	}
	if !got.Truncated || got.TruncatedReason != "max_findings" {
		t.Errorf("truncation = %v / %q", got.Truncated, got.TruncatedReason)
	}
	foundUTF16 := false
	for _, finding := range got.Findings {
		if finding.Value == "utf16@example.com" {
			foundUTF16 = true
		}
	}
	if !foundUTF16 {
		t.Errorf("UTF-16 finding missing: %#v", got.Findings)
	}
}

func TestRunFileScanMaxBytesAndTotalBudget(t *testing.T) {
	root := t.TempDir()
	var lines []string
	for i := 0; i < 80; i++ {
		lines = append(lines, "user"+strings.Repeat("x", 20)+string(rune('a'+i%26))+"@example.com")
	}
	writeScanTestFile(t, root, "many.txt", []byte(strings.Join(lines, "\n")))
	writeScanTestFile(t, root, "big.txt", []byte(strings.Repeat("a@example.com\n", 200)))

	raw, err := runFileScan(fileScanOpts{
		root:           root,
		patterns:       []string{"*"},
		rules:          map[string]bool{"email": true},
		ruleList:       []string{"email"},
		maxDepth:       8,
		maxFiles:       100,
		maxFileBytes:   fileScanDefaultMaxFileBytes,
		maxTotalBytes:  512,
		maxFindings:    100,
		timeout:        fileScanDefaultTimeout,
		maxResultBytes: 1024,
	})
	if err != nil {
		t.Fatalf("runFileScan: %v", err)
	}
	if len(raw) > 1024 {
		t.Fatalf("result bytes = %d, want <= 1024", len(raw))
	}
	got := decodeFileScanResult(t, raw)
	if !got.Truncated {
		t.Errorf("expected truncation, got %+v", got)
	}
	if got.TruncatedReason != "max_bytes" && got.TruncatedReason != "max_total_bytes" {
		t.Errorf("truncated_reason = %q", got.TruncatedReason)
	}
}
