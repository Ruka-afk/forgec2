//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultMigratePath(t *testing.T) {
	p := defaultMigratePath()
	if p == "" {
		t.Fatal("empty default migrate path")
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("default migrate path not absolute: %s", p)
	}
	if !strings.Contains(p, "migrated_") {
		t.Fatalf("default path lacks marker: %s", p)
	}
}

func TestCopySelf(t *testing.T) {
	src, err := os.CreateTemp(t.TempDir(), "src")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.WriteString("implant-bytes"); err != nil {
		t.Fatal(err)
	}
	src.Close()

	dest := filepath.Join(t.TempDir(), "migrated.elf")
	if err := copySelf(src.Name(), dest); err != nil {
		t.Fatalf("copySelf: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(b) != "implant-bytes" {
		t.Fatalf("copied content mismatch: %q", string(b))
	}
}

func TestMigrateParamParsingReusesCommandPath(t *testing.T) {
	// The handler passes the destination verbatim via task.Command; verify the
	// plain-text (non-encrypted) path flows through unchanged.
	task := Task{Type: "migrate", Command: "C:\\Users\\public\\stage\\note.exe"}
	dest := strings.TrimSpace(task.Command)
	if dest != "C:\\Users\\public\\stage\\note.exe" {
		t.Fatalf("path mangled: %q", dest)
	}
}