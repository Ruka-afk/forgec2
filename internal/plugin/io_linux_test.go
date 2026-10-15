//go:build linux
// +build linux

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestParseProcIOWChar covers the /proc/<pid>/io field the write watchdog
// sums: only the wchar line counts, and a dump without a parsable one is an
// error rather than a silent zero (a silent zero would leave a runaway writer
// unbounded).
func TestParseProcIOWChar(t *testing.T) {
	dump := "rchar: 4096\nwchar: 123456\nsyscr: 10\nsyscw: 7\nread_bytes: 0\nwrite_bytes: 0\ncancelled_write_bytes: 0\n"
	got, err := parseProcIOWChar([]byte(dump))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 123456 {
		t.Fatalf("wchar = %d, want 123456", got)
	}

	for _, bad := range []string{"", "rchar: 1\n", "wchar: lots\n"} {
		if _, err := parseProcIOWChar([]byte(bad)); err == nil {
			t.Errorf("parseProcIOWChar(%q) must fail", bad)
		}
	}
}

// TestGroupWriteBytesSumsProcessGroup exercises the /proc walk against a
// fixture tree: only numeric pid directories whose stat belongs to the group
// and whose io dump is readable count; exited, unreadable or malformed
// entries are skipped rather than fatal.
func TestGroupWriteBytesSumsProcessGroup(t *testing.T) {
	root := t.TempDir()
	member := func(pid, pgid int) string {
		return fmt.Sprintf("%d (writer) S 1 %d %d 0 -1 4194304 10 0 0 0 5 5 0 0 20 0 1 0 0 0 0 0 0 0 0 0 0 0 0 0 17 0 0\n",
			pid, pgid, pgid)
	}
	ioDump := func(n int64) string {
		return fmt.Sprintf("rchar: 1\nwchar: %d\nsyscr: 1\nsyscw: 1\n", n)
	}
	writeMember := func(entry, stat, io string) {
		t.Helper()
		dir := filepath.Join(root, entry)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if stat != "" {
			if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if io != "" {
			if err := os.WriteFile(filepath.Join(dir, "io"), []byte(io), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeMember("101", member(101, 900), ioDump(1<<20))   // counts
	writeMember("107", member(107, 900), ioDump(512))     // counts too
	writeMember("202", member(202, 200), ioDump(1<<30))   // different group
	writeMember("303", member(303, 900), "")              // io unreadable: skipped
	writeMember("notapid", member(1, 900), ioDump(1<<20)) // non-numeric dir name
	writeMember("404", "not a stat line", ioDump(1<<20))  // malformed stat: skipped

	got, err := groupWriteBytes(root, 900)
	if err != nil {
		t.Fatalf("groupWriteBytes: %v", err)
	}
	if want := int64(1<<20 + 512); got != want {
		t.Fatalf("written = %d, want %d", got, want)
	}

	// A dead group reports zero, not an error: there is nothing left to bound.
	if got, err := groupWriteBytes(root, 12345); err != nil || got != 0 {
		t.Fatalf("empty group = (%d, %v), want (0, nil)", got, err)
	}
	if _, err := groupWriteBytes(filepath.Join(root, "missing"), 900); err == nil {
		t.Fatal("unreadable procfs root must error")
	}
}
