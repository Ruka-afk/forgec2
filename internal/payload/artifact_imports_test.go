package payload

import (
	"encoding/binary"
	"strings"
	"testing"
)

// buildTestPE64 builds a minimal in-memory PE32+ image with a single
// ".idata" section (VA 0x1000, raw 0x200..0x300). The descriptor table at
// 0x200 holds one import (kernel32.dll) plus a null terminator; the kernel32
// ILT/IAT/hint-name data sits at the top of the section (0x2A0+). The region
// 0x228..0x2A0 is zero padding — slack available for in-place injection.
// When withSlack is false that region is filled with 0xFF so injection
// cannot fit.
func buildTestPE64(withSlack bool, importRVA uint32) []byte {
	data := make([]byte, 0x300)
	data[0] = 'M'
	data[1] = 'Z'
	binary.LittleEndian.PutUint32(data[0x3C:], 0x80)

	// PE signature + file header.
	data[0x80], data[0x81], data[0x82], data[0x83] = 'P', 'E', 0, 0
	binary.LittleEndian.PutUint16(data[0x84:], 0x8664) // Machine (x64)
	binary.LittleEndian.PutUint16(data[0x86:], 1)      // NumberOfSections
	binary.LittleEndian.PutUint16(data[0x94:], 0xF0)   // SizeOfOptionalHeader

	// Optional header: PE32+ magic.
	binary.LittleEndian.PutUint16(data[0x98:], 0x20B)
	// Data directory[1] (import) at opt+0x70+8 = 0x110.
	binary.LittleEndian.PutUint32(data[0x110:], importRVA)
	binary.LittleEndian.PutUint32(data[0x114:], 0x40)

	// Section header at 0x188 (pe+24+0xF0).
	sec := 0x188
	copy(data[sec:], ".idata")
	binary.LittleEndian.PutUint32(data[sec+8:], 0x100)  // VirtualSize
	binary.LittleEndian.PutUint32(data[sec+12:], 0x1000) // VirtualAddress
	binary.LittleEndian.PutUint32(data[sec+16:], 0x100)  // SizeOfRawData
	binary.LittleEndian.PutUint32(data[sec+20:], 0x200)  // PointerToRawData

	if importRVA == 0 {
		return data
	}

	// Import descriptor[0] at 0x200 (RVA 0x1000).
	put32 := func(off int, v uint32) { binary.LittleEndian.PutUint32(data[off:], v) }
	put64 := func(off int, v uint64) { binary.LittleEndian.PutUint64(data[off:], v) }
	put32(0x200, 0x10A0) // OriginalFirstThunk -> ILT
	put32(0x20C, 0x10D0) // Name -> "kernel32.dll"
	put32(0x210, 0x10B0) // FirstThunk -> IAT
	// Descriptor[1] = null terminator at 0x214..0x227.

	// ILT at 0x2A0, IAT at 0x2B0, hint/name at 0x2C0, DLL name at 0x2D0.
	put64(0x2A0, 0x10C0) // ILT thunk -> hint/name
	put64(0x2A8, 0)      // ILT null terminator
	put64(0x2B0, 0x10C0) // IAT thunk
	put64(0x2B8, 0)      // IAT null terminator
	put16 := func(off int, v uint16) { binary.LittleEndian.PutUint16(data[off:], v) }
	put16(0x2C0, 0) // hint
	copy(data[0x2C2:], "GetModuleHandleA\x00")
	copy(data[0x2D0:], "kernel32.dll\x00")

	if !withSlack {
		// Destroy the zero padding region after the descriptor table.
		for i := 0x228; i < 0x2A0; i++ {
			data[i] = 0xFF
		}
	}
	return data
}

func TestAddBenignImportsAlreadyPresent(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	// "kernel32" (no extension) normalizes to kernel32.dll, already present.
	if _, err := AddBenignImports(data, []string{"kernel32"}); err != nil {
		t.Fatalf("already-imported DLL should be a no-op: %v", err)
	}
	if _, err := AddBenignImports(data, nil); err != nil {
		t.Fatalf("empty request should be a no-op: %v", err)
	}
}

func TestAddBenignImportsInjectsIntoSlack(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	out, err := AddBenignImports(data, []string{"user32.dll"})
	if err != nil {
		t.Fatalf("injection into slack failed: %v", err)
	}
	view, err := parseImportView(out)
	if err != nil {
		t.Fatalf("re-parse after injection: %v", err)
	}
	names := view.importedDLLs()
	found := false
	for _, n := range names {
		if n == "user32.dll" {
			found = true
		}
	}
	if !found {
		t.Fatalf("user32.dll not present after injection; imports = %v", names)
	}
}

// TestAddBenignImportsGrowsWhenPacked upgrades the old "no slack" contract:
// a packed terminal section is extended (raw+virtual) with aligned zeros so
// injection always succeeds instead of failing the artifact.
func TestAddBenignImportsGrowsWhenPacked(t *testing.T) {
	data := buildTestPE64(false, 0x1000)
	out, err := AddBenignImports(data, []string{"user32.dll"})
	if err != nil {
		t.Fatalf("growth fallback failed: %v", err)
	}
	if len(out) <= len(data) {
		t.Fatal("expected section growth to extend the image")
	}
	view, err := parseImportView(out)
	if err != nil {
		t.Fatalf("re-parse grown image: %v", err)
	}
	found := false
	for _, n := range view.importedDLLs() {
		if n == "user32.dll" {
			found = true
		}
	}
	if !found {
		t.Fatalf("user32.dll missing after growth; imports = %v", view.importedDLLs())
	}
}

func TestAddBenignImportsUnknownDLL(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	if _, err := AddBenignImports(data, []string{"totally_unknown.dll"}); err == nil {
		t.Fatal("expected error for unknown DLL")
	}
}

func TestAddBenignImportsRejectsNonPE(t *testing.T) {
	if _, err := AddBenignImports([]byte("hello world"), []string{"user32.dll"}); err == nil {
		t.Fatal("expected error for non-PE input")
	}
}

func TestAddBenignImportsNoImportDirectory(t *testing.T) {
	data := buildTestPE64(true, 0)
	if _, err := AddBenignImports(data, []string{"user32.dll"}); err == nil {
		t.Fatal("expected error for PE without import directory")
	}
}

func TestApplyPESectionNamesPE32Plus(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	ApplyPESectionNames(data, PESectionConfig{Text: ".t1", Data: ".d1", Rdata: ".r1", Reloc: ".x1"})
	// The test PE declares a single section; only its 8-byte NUL-padded
	// header name may be rewritten.
	sec := 0x188
	name := strings.TrimRight(string(data[sec:sec+8]), "\x00")
	if name != ".t1" {
		t.Errorf("section 0 name = %q, want %q", name, ".t1")
	}
}
