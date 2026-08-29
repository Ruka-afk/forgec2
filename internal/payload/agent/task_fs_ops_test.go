package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestHandleMkdir covers single and nested directory creation plus the
// empty-path error path.
func TestHandleMkdir(t *testing.T) {
	base := t.TempDir()

	nested := filepath.Join(base, "a", "b", "c")
	res := TaskResult{}
	handleMkdir(Task{ID: 1, Type: "mkdir", Command: nested}, &res)
	if res.Error != "" {
		t.Fatalf("mkdir nested: %s", res.Error)
	}
	if fi, err := os.Stat(nested); err != nil || !fi.IsDir() {
		t.Fatalf("nested dir not created: %v", err)
	}

	res = TaskResult{}
	handleMkdir(Task{ID: 2, Type: "mkdir", Command: ""}, &res)
	if res.Error == "" {
		t.Fatal("empty path must error")
	}
}

// TestHandleRename covers file rename, directory move, missing-source error
// and the missing-parameter error path.
func TestHandleRename(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "old.txt")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(base, "new.txt")
	res := TaskResult{}
	handleRename(Task{ID: 3, Type: "rename", Command: src, Data: dst}, &res)
	if res.Error != "" {
		t.Fatalf("rename file: %s", res.Error)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("renamed file missing: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source still exists after rename")
	}

	// Directory move.
	dirA := filepath.Join(base, "dirA")
	dirB := filepath.Join(base, "dirB")
	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	res = TaskResult{}
	handleRename(Task{ID: 4, Type: "rename", Command: dirA, Data: dirB}, &res)
	if res.Error != "" {
		t.Fatalf("move dir: %s", res.Error)
	}
	if fi, err := os.Stat(dirB); err != nil || !fi.IsDir() {
		t.Fatalf("moved dir missing: %v", err)
	}

	res = TaskResult{}
	handleRename(Task{ID: 5, Type: "rename", Command: filepath.Join(base, "nope"), Data: dst}, &res)
	if res.Error == "" {
		t.Fatal("missing source must error")
	}

	res = TaskResult{}
	handleRename(Task{ID: 6, Type: "rename", Command: dst, Data: ""}, &res)
	if res.Error == "" {
		t.Fatal("missing new path must error")
	}
}

// TestHandleChmod verifies octal parsing and mode application on POSIX. On
// Windows Go maps chmod to the read-only bit only, so full-mode assertions
// are skipped there; the parse-error paths are checked on every platform.
func TestHandleChmod(t *testing.T) {
	base := t.TempDir()
	f := filepath.Join(base, "script.sh")
	if err := os.WriteFile(f, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := TaskResult{}
	handleChmod(Task{ID: 7, Type: "chmod", Command: f, Data: "0755"}, &res)
	if res.Error != "" {
		t.Fatalf("chmod: %s", res.Error)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != 0o755 {
			t.Fatalf("mode = %o, want 755", got)
		}
	}

	for name, tc := range map[string]struct{ cmd, data string }{
		"bad mode":   {f, "999"},
		"empty mode": {f, ""},
		"empty path": {"", "0755"},
	} {
		res := TaskResult{}
		handleChmod(Task{ID: 8, Type: "chmod", Command: tc.cmd, Data: tc.data}, &res)
		if res.Error == "" {
			t.Fatalf("%s: expected error", name)
		}
	}
}

// TestFSTaskTypesRegistered pins the registry wiring so a refactor that drops
// the handlers fails loudly instead of leaving tasks stuck as unknown types.
func TestFSTaskTypesRegistered(t *testing.T) {
	for _, typ := range []string{"mkdir", "rename", "chmod"} {
		if _, ok := taskHandlers[typ]; !ok {
			t.Errorf("task type %q not registered in taskHandlers", typ)
		}
	}
}
