package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

func TestRedactFileScanResultForAuxiliary(t *testing.T) {
	raw := `{
		"root":"C:\\Users\\alice",
		"patterns":["*.txt"],
		"rules":["email","bank_card"],
		"files_scanned":2,
		"files_skipped":1,
		"skipped_binary":1,
		"bytes_scanned":2048,
		"findings":[
			{"path":"C:\\Users\\alice\\notes.txt","name":"notes.txt","rule":"email","value":"alice@example.com","line":4,"occurrences":2},
			{"path":"C:\\Users\\alice\\notes.txt","name":"notes.txt","rule":"bank_card","value":"4111 1111 1111 1111","line":9,"occurrences":1}
		],
		"counts":{"email":1,"bank_card":1},
		"errors":0,
		"truncated":false,
		"elapsed_ms":12
	}`
	got := redactSensitiveResultForAuxiliary(protocol.TaskTypeFileScan, raw)
	if strings.Contains(got, "alice@example.com") || strings.Contains(got, "4111 1111") {
		t.Fatalf("raw value leaked: %s", got)
	}
	var doc redactedFileScanResult
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatalf("decode redacted result: %v\n%s", err, got)
	}
	if !doc.AuxiliaryRedacted || doc.RedactionNote == "" {
		t.Errorf("redaction marker missing: %+v", doc)
	}
	if len(doc.Findings) != 2 {
		t.Fatalf("findings = %d, want 2", len(doc.Findings))
	}
	for _, finding := range doc.Findings {
		if finding.Value != "[redacted]" || !finding.Redacted {
			t.Errorf("finding not redacted: %+v", finding)
		}
		if finding.Path == "" || finding.Rule == "" {
			t.Errorf("finding lost useful metadata: %+v", finding)
		}
	}
	if doc.FilesScanned != 2 || doc.BytesScanned != 2048 || doc.SkippedBinary != 1 {
		t.Errorf("summary lost: %+v", doc)
	}
}

func TestRedactFileScanResultFailsClosed(t *testing.T) {
	for _, raw := range []string{
		`{"findings":[{"value":"secret@example.com"}`,
		`{"root":"C:\\Users","value":"secret@example.com"}`,
		`{"findings":null,"value":"secret@example.com"}`,
	} {
		got := redactSensitiveResultForAuxiliary(protocol.TaskTypeFileScan, raw)
		if strings.Contains(got, "secret@example.com") {
			t.Fatalf("malformed result leaked: %s", got)
		}
		if !strings.Contains(got, "auxiliary_redacted") {
			t.Errorf("fail-closed marker missing: %s", got)
		}
	}
}

func TestRedactSensitiveResultLeavesOtherTasksAlone(t *testing.T) {
	raw := `{"value":"not a scan"}`
	if got := redactSensitiveResultForAuxiliary("shell", raw); got != raw {
		t.Errorf("non-file_scan result changed: %q", got)
	}
}
