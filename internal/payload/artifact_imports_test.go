package payload

import (
	"bytes"
	"debug/pe"
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
	binary.LittleEndian.PutUint32(data[0xB8:], 0x1000) // SectionAlignment
	binary.LittleEndian.PutUint32(data[0xBC:], 0x200)  // FileAlignment
	binary.LittleEndian.PutUint32(data[0xD0:], 0x2000) // SizeOfImage
	binary.LittleEndian.PutUint32(data[0xD4:], 0x200)  // SizeOfHeaders
	binary.LittleEndian.PutUint16(data[0xDC:], 3)      // Subsystem
	binary.LittleEndian.PutUint32(data[0x104:], 16)    // NumberOfRvaAndSizes
	// Data directory[1] (import) at opt+0x70+8 = 0x110.
	binary.LittleEndian.PutUint32(data[0x110:], importRVA)
	binary.LittleEndian.PutUint32(data[0x114:], 0x40)

	// Section header at 0x188 (pe+24+0xF0).
	sec := 0x188
	copy(data[sec:], ".idata")
	binary.LittleEndian.PutUint32(data[sec+8:], 0x100)   // VirtualSize
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

// buildTestPE32 is the PE32 counterpart of buildTestPE64. Its ILT/IAT entries
// are four bytes wide, so it exercises the 32-bit thunk path that windows/386
// artifacts use.
func buildTestPE32(withSlack bool) []byte {
	data := make([]byte, 0x300)
	data[0], data[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(data[0x3C:], 0x80)

	data[0x80], data[0x81], data[0x82], data[0x83] = 'P', 'E', 0, 0
	binary.LittleEndian.PutUint16(data[0x84:], 0x014c) // Machine (x86)
	binary.LittleEndian.PutUint16(data[0x86:], 1)      // NumberOfSections
	binary.LittleEndian.PutUint16(data[0x94:], 0xE0)   // SizeOfOptionalHeader

	opt := 0x98
	binary.LittleEndian.PutUint16(data[opt:], 0x10B) // PE32 magic
	binary.LittleEndian.PutUint32(data[opt+0x20:], 0x1000)
	binary.LittleEndian.PutUint32(data[opt+0x24:], 0x200)
	binary.LittleEndian.PutUint32(data[opt+0x38:], 0x2000)
	binary.LittleEndian.PutUint32(data[opt+0x3C:], 0x200)
	binary.LittleEndian.PutUint16(data[opt+0x44:], 3)
	binary.LittleEndian.PutUint32(data[opt+0x5C:], 16)
	dd := opt + 0x60
	binary.LittleEndian.PutUint32(data[dd+8:], 0x1000) // import RVA
	binary.LittleEndian.PutUint32(data[dd+12:], 0x40)  // import size

	sec := opt + 0xE0
	copy(data[sec:], ".idata")
	binary.LittleEndian.PutUint32(data[sec+8:], 0x100)
	binary.LittleEndian.PutUint32(data[sec+12:], 0x1000)
	binary.LittleEndian.PutUint32(data[sec+16:], 0x100)
	binary.LittleEndian.PutUint32(data[sec+20:], 0x200)

	put32 := func(off int, v uint32) { binary.LittleEndian.PutUint32(data[off:], v) }
	put32(0x200, 0x10A0) // OriginalFirstThunk
	put32(0x20C, 0x10D0) // Name
	put32(0x210, 0x10B0) // FirstThunk
	put32(0x2A0, 0x10C0) // ILT[0]
	put32(0x2A4, 0)      // ILT null
	put32(0x2B0, 0x10C0) // IAT[0]
	put32(0x2B4, 0)      // IAT null
	binary.LittleEndian.PutUint16(data[0x2C0:], 0)
	copy(data[0x2C2:], "GetModuleHandleA\x00")
	copy(data[0x2D0:], "kernel32.dll\x00")

	if !withSlack {
		for i := 0x228; i < 0x2A0; i++ {
			data[i] = 0xFF
		}
	}
	return data
}

func TestAddBenignImportsPE32AppendsSection(t *testing.T) {
	data := buildTestPE32(false)
	if err := validatePEImports(data); err != nil {
		t.Fatalf("valid PE32 fixture rejected: %v", err)
	}
	out, err := AddBenignImports(data, []string{"user32.dll"})
	if err != nil {
		t.Fatalf("PE32 new-section fallback failed: %v", err)
	}
	if err := validatePEImports(out); err != nil {
		t.Fatalf("PE32 imports do not validate after injection: %v", err)
	}
	f, err := pe.NewFile(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("extended PE32 no longer parses: %v", err)
	}
	defer f.Close()
	if len(f.Sections) != 2 {
		t.Fatalf("PE32 section count = %d, want 2", len(f.Sections))
	}
	names := parseImportViewForTest(t, out).importedDLLs()
	if !strings.Contains(strings.Join(names, ","), "user32.dll") {
		t.Fatalf("user32.dll missing after PE32 injection; imports = %v", names)
	}
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
	if err := validatePEImports(out); err != nil {
		t.Fatalf("injected imports do not validate: %v", err)
	}
}

// TestAddBenignImportsAppendsSectionWhenPacked verifies the fallback used when
// the import section has no slack: a new section is appended and the import
// directory is repointed, without shifting any existing bytes. The old growth
// fallback corrupted PointerToSymbolTable and could overlap the next section.
// TestAddBenignImportsRelocatedPreservesILTNullAtVirtualSize is the regression
// test for the corruption that motivated the relocated guard: the Go linker
// puts the ILT's terminating null thunk exactly at VirtualSize, and the old
// relocated injector started its new descriptor array there. Overwriting that
// null made the original descriptor walk off into the new descriptors and
// produce out-of-range thunk RVAs.
func TestAddBenignImportsRelocatedPreservesILTNullAtVirtualSize(t *testing.T) {
	data := buildTestPE64(false, 0x1000)
	data = append(data, make([]byte, 0x100)...)        // room past VirtualSize
	binary.LittleEndian.PutUint32(data[0x198:], 0x400) // SizeOfRawData
	// The fixture's ILT is at 0x2A0 and its null at 0x2A8. Move the descriptor
	// at the ILT that ends at VirtualSize (0x100): entry at 0xF8, null at 0x100.
	binary.LittleEndian.PutUint32(data[0x200:], 0x10F8) // OriginalFirstThunk
	binary.LittleEndian.PutUint32(data[0x210:], 0x10F8) // FirstThunk
	binary.LittleEndian.PutUint32(data[0x2F8:], 0x10C0)
	// Make in-place injection fail so the relocated strategy is selected.
	for i := 0x228; i < 0x2C0; i++ {
		data[i] = 0xFF
	}
	if err := validatePEImports(data); err != nil {
		t.Fatalf("fixture with ILT null at VirtualSize is invalid: %v", err)
	}

	out, err := AddBenignImports(data, []string{"user32.dll"})
	if err != nil {
		t.Fatalf("relocated injection failed: %v", err)
	}
	if len(out) != len(data) {
		t.Fatalf("relocated injection resized the file: %d -> %d bytes", len(data), len(out))
	}
	for i := 0x300; i < 0x308; i++ {
		if out[i] != 0 {
			t.Fatalf("relocated injection overwrote the ILT null at file offset 0x%x", i)
		}
	}
	if err := validatePEImports(out); err != nil {
		t.Fatalf("relocated imports do not validate: %v", err)
	}
	view := parseImportViewForTest(t, out)
	if view.descCount != 2 {
		t.Fatalf("descCount = %d, want 2", view.descCount)
	}
	f, err := pe.NewFile(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("relocated PE no longer parses: %v", err)
	}
	defer f.Close()
	if len(f.Sections) != 1 {
		t.Fatalf("relocated injection added a section (%d sections); want in-place relocation", len(f.Sections))
	}
}

func TestAddBenignImportsAppendsSectionWhenPacked(t *testing.T) {
	data := buildTestPE64(false, 0x1000)
	out, err := AddBenignImports(data, []string{"user32.dll"})
	if err != nil {
		t.Fatalf("new-section fallback failed: %v", err)
	}
	if len(out) <= len(data) {
		t.Fatal("expected the new section to extend the image")
	}
	view, err := parseImportView(out)
	if err != nil {
		t.Fatalf("re-parse extended image: %v", err)
	}
	found := false
	for _, n := range view.importedDLLs() {
		if n == "user32.dll" {
			found = true
		}
	}
	if !found {
		t.Fatalf("user32.dll missing after extension; imports = %v", view.importedDLLs())
	}
	if err := validatePEImports(out); err != nil {
		t.Fatalf("extended imports do not validate: %v", err)
	}
	f, err := pe.NewFile(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("extended PE no longer parses with debug/pe: %v", err)
	}
	defer f.Close()
	if len(f.Sections) != 2 {
		t.Fatalf("section count = %d, want 2", len(f.Sections))
	}
	var newSection *pe.Section
	for _, s := range f.Sections {
		if s.Name == importSectionName {
			newSection = s
		}
	}
	if newSection == nil {
		t.Fatalf("new section %q not found in %v", importSectionName, f.Sections)
	}
	if newSection.VirtualAddress != 0x2000 || newSection.Size != 0x200 {
		t.Fatalf("new section VA/size = 0x%x/0x%x, want 0x2000/0x200",
			newSection.VirtualAddress, newSection.Size)
	}
	// Existing section data must not move or change when a section is appended
	// (only header fields are updated).
	if !bytes.Equal(out[0x200:len(data)], data[0x200:]) {
		t.Fatal("new-section fallback modified existing section data")
	}
}

func TestValidatePEImportsRejectsBrokenTables(t *testing.T) {
	valid := buildTestPE64(true, 0x1000)
	if err := validatePEImports(valid); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}

	t.Run("dll name rva", func(t *testing.T) {
		data := buildTestPE64(true, 0x1000)
		binary.LittleEndian.PutUint32(data[0x20C:], 0xFFFFFFF0)
		if err := validatePEImports(data); err == nil {
			t.Fatal("expected an out-of-range DLL name RVA to fail validation")
		}
	})
	t.Run("thunk rva", func(t *testing.T) {
		data := buildTestPE64(true, 0x1000)
		binary.LittleEndian.PutUint64(data[0x2A0:], 0xFFFFFFFF)
		if err := validatePEImports(data); err == nil {
			t.Fatal("expected an out-of-range thunk RVA to fail validation")
		}
	})
	t.Run("unterminated descriptors", func(t *testing.T) {
		data := buildTestPE64(true, 0x1000)
		// Make descriptor 1 non-null and end the section right after it, so
		// no null descriptor can follow inside the import section.
		const sec = 0x188
		binary.LittleEndian.PutUint32(data[sec+16:], 0x228) // SizeOfRawData
		binary.LittleEndian.PutUint32(data[0x214:], 0x10A0)
		if err := validatePEImports(data); err == nil {
			t.Fatal("expected an unterminated descriptor table to fail validation")
		}
	})
}

func TestAddBenignImportsRejectsSignedPE(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	signed, err := SignPESelfSigned(data, "Import Test")
	if err != nil {
		t.Fatalf("sign fixture: %v", err)
	}
	if _, err := AddBenignImports(signed, []string{"user32.dll"}); err == nil ||
		!strings.Contains(err.Error(), "signed") {
		t.Fatalf("signed PE was modified: err = %v, want a signed-PE refusal", err)
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

func TestApplyPESectionNamesMatchesByName(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	cfg := PESectionConfig{Text: ".t1", Data: ".d1", Rdata: ".r1", Reloc: ".x1"}
	sec := 0x188
	// A section called .idata must not be renamed just because it is first in
	// the table (the old index-based implementation did exactly that).
	ApplyPESectionNames(data, cfg)
	if name := strings.TrimRight(string(data[sec:sec+8]), "\x00"); name != ".idata" {
		t.Fatalf(".idata section was rewritten to %q", name)
	}

	copy(data[sec:sec+8], ".text\x00\x00\x00")
	ApplyPESectionNames(data, cfg)
	if name := strings.TrimRight(string(data[sec:sec+8]), "\x00"); name != ".t1" {
		t.Errorf(".text section name = %q, want %q", name, ".t1")
	}
}

func TestApplyPESectionNamesResolvesCOFFLongName(t *testing.T) {
	data := buildTestPE64(true, 0x1000)
	// Put a COFF string table after the image and point section 0 at entry 4
	// using the /nnn long-name form.
	const symOff = 0x300
	table := make([]byte, 0x40)
	binary.LittleEndian.PutUint32(table, uint32(len(table)))
	copy(table[4:], ".text\x00")
	data = append(data, table...)
	binary.LittleEndian.PutUint32(data[0x80+12:], symOff) // PointerToSymbolTable
	binary.LittleEndian.PutUint32(data[0x80+16:], 0)      // NumberOfSymbols
	copy(data[0x188:0x190], "/4\x00\x00\x00\x00\x00\x00")

	ApplyPESectionNames(data, PESectionConfig{Text: ".t1"})
	if name := strings.TrimRight(string(data[0x188:0x190]), "\x00"); name != ".t1" {
		t.Errorf("long-name section resolved to %q, want %q", name, ".t1")
	}
}
