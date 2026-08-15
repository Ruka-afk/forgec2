//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestGetUUIDFilePathNotSystemFile ensures the agent UUID is persisted in a
// dedicated private path and never co-opts/corrupts a host system file such as
// /var/lib/dbus/machine-id or the cfprefs plist.
func TestGetUUIDFilePathNotSystemFile(t *testing.T) {
	p := getUUIDFilePath()
	if strings.Contains(p, "machine-id") {
		t.Fatalf("UUID path co-opts dbus machine-id: %s", p)
	}
	if strings.Contains(p, "cfprefsd") {
		t.Fatalf("UUID path co-opts cfprefs plist: %s", p)
	}
	if !filepath.IsAbs(p) {
		t.Fatalf("expected absolute UUID path, got %s", p)
	}
}
