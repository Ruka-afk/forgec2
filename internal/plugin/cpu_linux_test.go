//go:build linux
// +build linux

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestParseProcStat covers the parsing rule that matters for /proc/<pid>/stat:
// the comm field may contain spaces and parentheses, so parsing may only
// resume after its LAST ')'.
func TestParseProcStat(t *testing.T) {
	line := "4242 (my ) weird ) prog) S 1 4242 4242 0 -1 4194304 100 0 0 0 7 3 0 0 20 0 1 0 123456 789 0 0 0 0 0 0 0 0 0 0 0 0 17 0 0\n"
	pgid, utime, stime, err := parseProcStat([]byte(line))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pgid != 4242 || utime != 7 || stime != 3 {
		t.Fatalf("got pgid=%d utime=%d stime=%d, want 4242/7/3", pgid, utime, stime)
	}

	for _, bad := range []string{"", "4242 S 1 4242", "4242 (unterminated", "4242 (x) S 1"} {
		if _, _, _, err := parseProcStat([]byte(bad)); err == nil {
			t.Errorf("parseProcStat(%q) must fail", bad)
		}
	}
}

// TestGroupCPUTimeSumsProcessGroup exercises the /proc walk against a fixture
// tree: only numeric pid directories whose stat belongs to the group count,
// and exited or malformed entries are skipped rather than fatal.
func TestGroupCPUTimeSumsProcessGroup(t *testing.T) {
	root := t.TempDir()
	member := func(pid, pgid int, comm string, utime, stime int64) string {
		return fmt.Sprintf("%d (%s) S 1 %d %d 0 -1 4194304 10 0 0 0 %d %d 0 0 20 0 1 0 0 0 0 0 0 0 0 0 0 0 0 0 17 0 0\n",
			pid, comm, pgid, pgid, utime, stime)
	}
	writeStat := func(entry string, body string) {
		t.Helper()
		dir := filepath.Join(root, entry)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeStat("101", member(101, 900, "worker", 30, 5))   // +35 ticks
	writeStat("107", member(107, 900, "kid) name", 5, 0)) // +5 ticks, tricky comm
	writeStat("202", member(202, 200, "other", 900, 0))   // different group
	writeStat("notapid", member(1, 900, "junk", 4, 4))    // non-numeric dir name
	writeStat("303", "not a stat line")                   // malformed, skipped

	got, err := groupCPUTime(root, 900)
	if err != nil {
		t.Fatalf("groupCPUTime: %v", err)
	}
	if want := 40 * (time.Second / linuxUserHZ); got != want {
		t.Fatalf("cpu time = %s, want %s", got, want)
	}

	// A dead group reports zero, not an error: there is nothing left to bound.
	if got, err := groupCPUTime(root, 12345); err != nil || got != 0 {
		t.Fatalf("empty group = (%s, %v), want (0, nil)", got, err)
	}
	if _, err := groupCPUTime(filepath.Join(root, "missing"), 900); err == nil {
		t.Fatal("unreadable procfs root must error")
	}
}
