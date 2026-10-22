package payload

import (
	"debug/pe"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testPEOptions lets a test tune the minimal PE fixture without duplicating
// header layout math in every case.
type testPEOptions struct {
	DLL           bool
	Subsystem     uint16
	Entry         uint32
	Timestamp     uint32
	DebugDir      bool
	ExceptionDir  bool
	SizeOfImage   uint32
	SizeOfHeaders uint32
}

const (
	testPEElfanew      = 0x80
	testPEOptSize32    = 0xE0
	testPEOptSize64    = 0xF0
	testPESectionAlign = 0x1000
	testPEFileAlign    = 0x200
	testPEHeadersSize  = 0x200
	testPETextVA       = 0x1000
	testPETextRawSize  = 0x200
	testPETextRawOff   = 0x200
	testPEFileSize     = testPEHeadersSize + testPETextRawSize
)

// makePE builds a structurally valid single-section PE that debug/pe can parse
// and validatePEWith can inspect: DOS header, COFF header, complete optional
// header with 16 data directories, and a .text section mapped into the image.
func makePE(t *testing.T, machine uint16, pe32plus bool) []byte {
	t.Helper()
	return makePEWith(t, machine, pe32plus, testPEOptions{})
}

func makePEWith(t *testing.T, machine uint16, pe32plus bool, opts testPEOptions) []byte {
	t.Helper()
	optSize := testPEOptSize32
	if pe32plus {
		optSize = testPEOptSize64
	}
	data := make([]byte, testPEFileSize)
	data[0], data[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(data[0x3C:], testPEElfanew)
	copy(data[testPEElfanew:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[testPEElfanew+4:], machine)
	binary.LittleEndian.PutUint16(data[testPEElfanew+6:], 1) // NumberOfSections
	binary.LittleEndian.PutUint16(data[testPEElfanew+20:], uint16(optSize))
	chars := uint16(pe.IMAGE_FILE_EXECUTABLE_IMAGE)
	if !pe32plus {
		chars |= 0x0100 // IMAGE_FILE_32BIT_MACHINE
	}
	if opts.DLL {
		chars |= pe.IMAGE_FILE_DLL
	}
	binary.LittleEndian.PutUint16(data[testPEElfanew+22:], chars)

	opt := testPEElfanew + 24
	magic := uint16(0x20b)
	if !pe32plus {
		magic = 0x10b
	}
	binary.LittleEndian.PutUint16(data[opt:], magic)
	entry := opts.Entry
	if entry == 0 && !opts.DLL {
		entry = testPETextVA
	}
	binary.LittleEndian.PutUint32(data[opt+0x10:], entry)
	binary.LittleEndian.PutUint32(data[opt+0x20:], testPESectionAlign)
	binary.LittleEndian.PutUint32(data[opt+0x24:], testPEFileAlign)
	sizeOfImage := opts.SizeOfImage
	if sizeOfImage == 0 {
		sizeOfImage = 0x2000
	}
	binary.LittleEndian.PutUint32(data[opt+0x38:], sizeOfImage)
	sizeOfHeaders := opts.SizeOfHeaders
	if sizeOfHeaders == 0 {
		sizeOfHeaders = testPEHeadersSize
	}
	binary.LittleEndian.PutUint32(data[opt+0x3C:], sizeOfHeaders)
	subsystem := opts.Subsystem
	if subsystem == 0 {
		subsystem = pe.IMAGE_SUBSYSTEM_WINDOWS_CUI
	}
	binary.LittleEndian.PutUint16(data[opt+0x44:], subsystem)
	if pe32plus {
		binary.LittleEndian.PutUint32(data[opt+0x6C:], 16) // NumberOfRvaAndSizes
	} else {
		binary.LittleEndian.PutUint32(data[opt+0x5C:], 16)
	}
	section := opt + optSize
	copy(data[section:], ".text")
	binary.LittleEndian.PutUint32(data[section+8:], testPETextRawSize)  // VirtualSize
	binary.LittleEndian.PutUint32(data[section+12:], testPETextVA)      // VirtualAddress
	binary.LittleEndian.PutUint32(data[section+16:], testPETextRawSize) // SizeOfRawData
	binary.LittleEndian.PutUint32(data[section+20:], testPETextRawOff)  // PointerToRawData
	binary.LittleEndian.PutUint32(data[section+36:], 0x60000020)        // code|execute|read
	if opts.Timestamp != 0 {
		binary.LittleEndian.PutUint32(data[testPEElfanew+8:], opts.Timestamp)
	}
	ddBase := opt + 0x70
	if !pe32plus {
		ddBase = opt + 0x60
	}
	if opts.DebugDir {
		binary.LittleEndian.PutUint32(data[ddBase+6*8:], 0x1100)
		binary.LittleEndian.PutUint32(data[ddBase+6*8+4:], 28)
	}
	if opts.ExceptionDir {
		binary.LittleEndian.PutUint32(data[ddBase+3*8:], 0x1200)
		binary.LittleEndian.PutUint32(data[ddBase+3*8+4:], 12)
	}
	return data
}

// peHeaderOffsets recomputes the header offsets the way the implementation
// does, so tests assert the real data-directory base instead of a copy of a
// previous bug.
func peHeaderOffsets(t *testing.T, data []byte) (opt, ddBase, sectionTable int) {
	t.Helper()
	eLfanew := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	opt = eLfanew + 24
	optSize := int(binary.LittleEndian.Uint16(data[eLfanew+20 : eLfanew+22]))
	sectionTable = opt + optSize
	magic := binary.LittleEndian.Uint16(data[opt:])
	ddBase = opt + 0x60
	if magic == 0x20b {
		ddBase = opt + 0x70
	}
	return opt, ddBase, sectionTable
}

func writeTestPE(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "artifact.exe")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidatePEAcceptsValid(t *testing.T) {
	for _, tc := range []struct {
		name    string
		machine uint16
		plus    bool
	}{
		{"amd64", 0x8664, true},
		{"386", 0x014c, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTestPE(t, makePE(t, tc.machine, tc.plus))
			if err := validatePE(path, tc.machine); err != nil {
				t.Fatalf("valid PE rejected: %v", err)
			}
			if err := validatePEWith(path, peExpect{
				Machine:             tc.machine,
				RequireDebugCleared: true,
				Subsystem:           pe.IMAGE_SUBSYSTEM_WINDOWS_CUI,
			}); err != nil {
				t.Fatalf("valid PE rejected by extended validation: %v", err)
			}
		})
	}
}

func TestValidatePERejectsOverlappingSections(t *testing.T) {
	data := makePE(t, 0x8664, true)
	_, _, sectionTable := peHeaderOffsets(t, data)
	binary.LittleEndian.PutUint16(data[testPEElfanew+6:], 2) // NumberOfSections
	second := sectionTable + 40
	copy(data[second:second+8], ".data\x00\x00\x00")
	binary.LittleEndian.PutUint32(data[second+8:], 0x200)         // VirtualSize
	binary.LittleEndian.PutUint32(data[second+12:], testPETextVA) // same VA as .text
	binary.LittleEndian.PutUint32(data[second+16:], 0)            // SizeOfRawData
	binary.LittleEndian.PutUint32(data[second+20:], 0)            // PointerToRawData
	binary.LittleEndian.PutUint32(data[second+36:], 0xC0000040)   // data|read|write

	path := writeTestPE(t, data)
	err := validatePE(path, 0x8664)
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("overlapping sections: err = %v, want an overlap error", err)
	}
}

func TestGenerateTimestampRandomIsPlausible(t *testing.T) {
	floor := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	ceiling := time.Now().Unix()
	for i := 0; i < 32; i++ {
		ts, err := GenerateTimestamp(TSRandom, "")
		if err != nil {
			t.Fatalf("GenerateTimestamp(random): %v", err)
		}
		if int64(ts) < floor || int64(ts) > ceiling {
			t.Fatalf("random timestamp %d (%s) outside [%d, %d)",
				ts, time.Unix(int64(ts), 0).UTC().Format(time.RFC3339), floor, ceiling)
		}
	}
}

func TestValidatePEArtifactInvariants(t *testing.T) {
	// Each mutator runs on a fresh amd64 fixture.
	tests := []struct {
		name   string
		mutate func(t *testing.T, data []byte) peExpect
	}{
		{
			name: "wrong machine",
			mutate: func(t *testing.T, data []byte) peExpect {
				return peExpect{Machine: 0x014c}
			},
		},
		{
			name: "dll flag missing",
			mutate: func(t *testing.T, data []byte) peExpect {
				return peExpect{Machine: 0x8664, DLL: true}
			},
		},
		{
			name: "dll flag unexpected",
			mutate: func(t *testing.T, data []byte) peExpect {
				chars := binary.LittleEndian.Uint16(data[testPEElfanew+22:])
				binary.LittleEndian.PutUint16(data[testPEElfanew+22:], chars|pe.IMAGE_FILE_DLL)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "entry point outside sections",
			mutate: func(t *testing.T, data []byte) peExpect {
				opt, _, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[opt+0x10:], 0x9000)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "exe without entry point",
			mutate: func(t *testing.T, data []byte) peExpect {
				opt, _, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[opt+0x10:], 0)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "raw section outside file",
			mutate: func(t *testing.T, data []byte) peExpect {
				_, _, section := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[section+20:], 0x800)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "virtual range exceeds image",
			mutate: func(t *testing.T, data []byte) peExpect {
				opt, _, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[opt+0x38:], 0x1000)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "subsystem mismatch",
			mutate: func(t *testing.T, data []byte) peExpect {
				return peExpect{Machine: 0x8664, Subsystem: pe.IMAGE_SUBSYSTEM_WINDOWS_GUI}
			},
		},
		{
			name: "debug directory not cleared",
			mutate: func(t *testing.T, data []byte) peExpect {
				_, ddBase, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[ddBase+6*8:], 0x1100)
				binary.LittleEndian.PutUint32(data[ddBase+6*8+4:], 28)
				return peExpect{Machine: 0x8664, RequireDebugCleared: true}
			},
		},
		{
			name: "unsigned image carries certificate table",
			mutate: func(t *testing.T, data []byte) peExpect {
				_, ddBase, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[ddBase+4*8:], testPEFileSize)
				binary.LittleEndian.PutUint32(data[ddBase+4*8+4:], 8)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "size of image misaligned",
			mutate: func(t *testing.T, data []byte) peExpect {
				opt, _, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[opt+0x38:], 0x1234)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "size of headers zero",
			mutate: func(t *testing.T, data []byte) peExpect {
				opt, _, _ := peHeaderOffsets(t, data)
				binary.LittleEndian.PutUint32(data[opt+0x3C:], 0)
				return peExpect{Machine: 0x8664}
			},
		},
		{
			name: "truncated file",
			mutate: func(t *testing.T, data []byte) peExpect {
				return peExpect{Machine: 0x8664}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := makePEWith(t, 0x8664, true, testPEOptions{})
			exp := tc.mutate(t, data)
			if tc.name == "truncated file" {
				data = data[:0x100]
			}
			path := writeTestPE(t, data)
			if err := validatePEWith(path, exp); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidatePEAcceptsSignedImage(t *testing.T) {
	data := makePEWith(t, 0x8664, true, testPEOptions{})
	signed, err := SignPESelfSigned(data, "Validation Test")
	if err != nil {
		t.Fatalf("sign fixture: %v", err)
	}
	path := writeTestPE(t, signed)
	if err := validatePEWith(path, peExpect{
		Machine:             0x8664,
		Signed:              true,
		RequireDebugCleared: true,
	}); err != nil {
		t.Fatalf("signed PE rejected: %v", err)
	}
	if err := validatePEWith(path, peExpect{Machine: 0x8664}); err == nil {
		t.Fatal("signed PE accepted as unsigned")
	}

	// A checksum mismatch must fail even though the certificate table is
	// structurally valid.
	tampered := append([]byte(nil), signed...)
	tampered[testPETextRawOff] ^= 0xFF
	tamperedPath := writeTestPE(t, tampered)
	if err := validatePEWith(tamperedPath, peExpect{Machine: 0x8664, Signed: true}); err == nil {
		t.Fatal("signed PE with a bad checksum was accepted")
	}
}

func TestStripPEArtifactsFixesDataDirectories(t *testing.T) {
	data := makePEWith(t, 0x8664, true, testPEOptions{
		Timestamp:    0x11223344,
		DebugDir:     true,
		ExceptionDir: true,
	})
	_, ddBase, _ := peHeaderOffsets(t, data)

	origTS, err := stripPEArtifactsInPlace(data)
	if err != nil {
		t.Fatalf("strip: %v", err)
	}
	if origTS != 0x11223344 {
		t.Fatalf("original timestamp = 0x%x, want 0x11223344", origTS)
	}
	if ts := binary.LittleEndian.Uint32(data[testPEElfanew+8:]); ts != 0 {
		t.Fatalf("timestamp = 0x%x, want 0", ts)
	}
	// Index 3 is the exception directory. The old bug computed the data
	// directory base 0x18 bytes early and zeroed this instead of index 6.
	excVA := binary.LittleEndian.Uint32(data[ddBase+3*8:])
	excSize := binary.LittleEndian.Uint32(data[ddBase+3*8+4:])
	if excVA != 0x1200 || excSize != 12 {
		t.Fatalf("exception directory was modified: offset=0x%x size=%d", excVA, excSize)
	}
	// Index 4 is the certificate table and must never be zeroed by stripping.
	if secVA := binary.LittleEndian.Uint32(data[ddBase+4*8:]); secVA != 0 {
		t.Fatalf("security directory was modified: offset=0x%x", secVA)
	}
	if dbgVA := binary.LittleEndian.Uint32(data[ddBase+6*8:]); dbgVA != 0 {
		t.Fatalf("debug directory was not cleared: offset=0x%x", dbgVA)
	}
	if dbgSize := binary.LittleEndian.Uint32(data[ddBase+6*8+4:]); dbgSize != 0 {
		t.Fatalf("debug directory size was not cleared: %d", dbgSize)
	}

	path := writeTestPE(t, data)
	if err := validatePEWith(path, peExpect{
		Machine:             0x8664,
		RequireDebugCleared: true,
	}); err != nil {
		t.Fatalf("stripped PE failed validation: %v", err)
	}
}

func TestFinalizePETimestampModes(t *testing.T) {
	const origTS = 0x5F112233
	for _, tc := range []struct {
		mode  string
		check func(t *testing.T, ts uint32)
	}{
		{"keep", func(t *testing.T, ts uint32) {
			if ts != origTS {
				t.Fatalf("keep timestamp = 0x%x, want 0x%x", ts, origTS)
			}
		}},
		{"zero", func(t *testing.T, ts uint32) {
			if ts != 0 {
				t.Fatalf("zero timestamp = 0x%x, want 0", ts)
			}
		}},
		{"random", func(t *testing.T, ts uint32) {
			if ts == 0 || ts == origTS {
				t.Fatalf("random timestamp = 0x%x, want a fresh non-zero value", ts)
			}
		}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			path := writeTestPE(t, makePEWith(t, 0x8664, true, testPEOptions{
				Timestamp:    origTS,
				DebugDir:     true,
				ExceptionDir: true,
			}))
			if err := finalizePE(path, ImplantConfig{PETimestampMode: tc.mode}, "amd64"); err != nil {
				t.Fatalf("finalizePE: %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, binary.LittleEndian.Uint32(data[testPEElfanew+8:]))
			_, ddBase, _ := peHeaderOffsets(t, data)
			if dbg := binary.LittleEndian.Uint32(data[ddBase+6*8:]); dbg != 0 {
				t.Fatalf("debug directory survived finalize: 0x%x", dbg)
			}
			if exc := binary.LittleEndian.Uint32(data[ddBase+3*8:]); exc != 0x1200 {
				t.Fatalf("exception directory was modified by finalize: 0x%x", exc)
			}
			if err := validatePEWith(path, peExpect{
				Machine:             0x8664,
				RequireDebugCleared: true,
			}); err != nil {
				t.Fatalf("finalized PE failed validation: %v", err)
			}
		})
	}
}

func TestFinalizePERandomizesSectionNames(t *testing.T) {
	path := writeTestPE(t, makePE(t, 0x8664, true))
	if err := finalizePE(path, ImplantConfig{PESectionMode: "random"}, "amd64"); err != nil {
		t.Fatalf("finalizePE: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, section := peHeaderOffsets(t, data)
	name := string(data[section : section+8])
	if name == ".text\x00\x00\x00" {
		t.Fatal("section name was not randomized")
	}
	if err := validatePE(path, 0x8664); err != nil {
		t.Fatalf("finalized PE failed validation: %v", err)
	}
}

func TestStripPEArtifactsRejectsNonPE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-pe.bin")
	if err := os.WriteFile(path, []byte("MZ not really a PE"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stripPEArtifacts(path); err == nil {
		t.Fatal("expected non-PE input to fail stripping")
	}
	if err := finalizePE(path, ImplantConfig{}, "amd64"); err == nil {
		t.Fatal("expected finalizePE to reject a non-PE")
	}
}
