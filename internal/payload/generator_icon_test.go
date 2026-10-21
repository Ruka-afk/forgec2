package payload

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func utf16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(u)*2)
	for _, r := range u {
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

func TestInjectIconResourceUsesUnprefixedOriginalFilename(t *testing.T) {
	dir := t.TempDir()
	cfg := ImplantConfig{
		Architecture:     "amd64",
		Filename:         "deadbeef_report.pdf.exe",
		OriginalFilename: "report.pdf.exe",
		FileDescription:  "PDF Document",
		CompanyName:      "Acme",
	}
	if err := injectIconResource(dir, cfg, cfg.OriginalFilename); err != nil {
		t.Fatalf("injectIconResource: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "rsrc.syso"))
	if err != nil {
		t.Fatalf("read rsrc.syso: %v", err)
	}
	if !bytes.Contains(data, utf16LE("report.pdf.exe")) {
		t.Fatal("OriginalFilename was not embedded without the server prefix")
	}
	if bytes.Contains(data, utf16LE("deadbeef_report.pdf.exe")) {
		t.Fatal("OriginalFilename leaked the server's uniqueness prefix")
	}
	if bytes.Contains(data, utf16LE("forgec2_agent.exe")) {
		t.Fatal("OriginalFilename leaked the internal default build name")
	}
}

func TestInjectIconResourceRejectsRequestedButUnavailableResources(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  ImplantConfig
	}{
		{"invalid custom icon", ImplantConfig{IconB64: base64.StdEncoding.EncodeToString([]byte("not an icon"))}},
		{"unknown preset", ImplantConfig{IconPreset: "does-not-exist"}},
		{"oversized custom icon", ImplantConfig{IconB64: base64.StdEncoding.EncodeToString(make([]byte, 300*1024))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := injectIconResource(t.TempDir(), tc.cfg, "agent.exe"); err == nil {
				t.Fatal("expected resource injection to fail")
			}
		})
	}

	// A known preset still works and produces a resource object.
	dir := t.TempDir()
	if err := injectIconResource(dir, ImplantConfig{IconPreset: "pdf", Architecture: "amd64"}, "agent.exe"); err != nil {
		t.Fatalf("valid preset rejected: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "rsrc.syso"))
	if err != nil || info.Size() == 0 {
		t.Fatalf("preset did not produce rsrc.syso: %v", err)
	}
}
