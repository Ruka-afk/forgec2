package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeLsResult(t *testing.T) {
	sample := "Type\tName\tSize\tModified\n" +
		strings.Repeat("-", 80) + "\n" +
		"DIR\tWindows\t-\t2024-01-01 12:00\n" +
		"FILE\tsecret.txt\t1234\t2024-02-02 09:30\n"

	out := normalizeLsResult(sample)
	if !strings.HasPrefix(out, "[") {
		t.Fatalf("expected JSON array, got: %q", out)
	}

	var entries []struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %s", len(entries), out)
	}
	if entries[0].Name != "Windows" || !entries[0].IsDir {
		t.Errorf("entry[0] wrong: %+v", entries[0])
	}
	if entries[1].Name != "secret.txt" || entries[1].IsDir || entries[1].Size != 1234 {
		t.Errorf("entry[1] wrong: %+v", entries[1])
	}
	if entries[1].ModTime != "2024-02-02 09:30" {
		t.Errorf("entry[1].ModTime wrong: %q", entries[1].ModTime)
	}
}

func TestNormalizeLsResultEmpty(t *testing.T) {
	// Header/separator only -> no entries -> returns input unchanged.
	in := "Type\tName\tSize\tModified\n" + strings.Repeat("-", 80) + "\n"
	if got := normalizeLsResult(in); got != in {
		t.Fatalf("expected unchanged input, got %q", got)
	}
}
