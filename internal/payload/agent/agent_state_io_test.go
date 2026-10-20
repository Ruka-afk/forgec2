package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteStateFileAtomicReplacesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "beacon.seq")

	if err := writeStateFileAtomic(path, []byte("41"), 0o600, true); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := writeStateFileAtomic(path, []byte("42"), 0o600, true); err != nil {
		t.Fatalf("second write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "42" {
		t.Fatalf("content = %q, want %q", got, "42")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, stateTempPattern)); len(leftovers) != 0 {
		t.Fatalf("temp files left behind: %v", leftovers)
	}
}

func TestWriteStateFileAtomicMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not carry POSIX permission bits")
	}
	path := filepath.Join(t.TempDir(), "update.key")
	if err := writeStateFileAtomic(path, []byte("ab"), 0o600, false); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestCleanupStateTempFiles(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "beacon.seq")
	if err := os.WriteFile(keep, []byte("7"), 0o600); err != nil {
		t.Fatalf("seed keep: %v", err)
	}
	stateTmp := filepath.Join(dir, ".state-123.tmp")
	spoolTmp := filepath.Join(dir, ".results-456.tmp")
	for _, p := range []string{stateTmp, spoolTmp} {
		if err := os.WriteFile(p, []byte("partial"), 0o600); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}

	cleanupStateTempFiles(dir)

	for _, p := range []string{stateTmp, spoolTmp} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("temp file survived cleanup: %s (%v)", p, err)
		}
	}
	if got, err := os.ReadFile(keep); err != nil || string(got) != "7" {
		t.Fatalf("cleanup removed persisted state: %q, %v", got, err)
	}
}
