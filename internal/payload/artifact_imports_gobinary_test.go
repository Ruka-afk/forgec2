package payload

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func parseImportViewForTest(t *testing.T, data []byte) *peImportView {
	t.Helper()
	view, err := parseImportView(data)
	if err != nil {
		t.Fatalf("parseImportView: %v", err)
	}
	return view
}

func (v *peImportView) descriptorAt(i int) peImportDescriptor {
	base := v.importOff + i*20
	return peImportDescriptor{
		OriginalFirstThunk: binary.LittleEndian.Uint32(v.data[base:]),
		TimeDateStamp:      binary.LittleEndian.Uint32(v.data[base+4:]),
		ForwarderChain:     binary.LittleEndian.Uint32(v.data[base+8:]),
		Name:               binary.LittleEndian.Uint32(v.data[base+12:]),
		FirstThunk:         binary.LittleEndian.Uint32(v.data[base+16:]),
	}
}

func (v *peImportView) rvaToOffsetSafe(rva uint32) (int, bool) {
	return rvaToOffset(v.sections, rva)
}

// buildDebugGoEXE compiles a tiny windows/amd64 Go binary and returns its
// bytes. This exercises the same .idata layout the real loader module uses.
func buildDebugGoEXE(t *testing.T) []byte {
	t.Helper()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(src, []byte(`package main

import "os"

func main() {
	os.Exit(0)
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmp, "probe.exe"), src)
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "probe.exe"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// artifact_imports_gobinary_test.go validates AddBenignImports against
// binaries produced by the Go toolchain (the same .idata layout the real
// loader module uses): the in-place strategy cannot apply there, so the
// relocation strategy is exercised for real, and the rewritten import table
// is checked against the invariants the Windows loader enforces.

func TestDebugGoBinaryImportSectionPadding(t *testing.T) {
	data := buildDebugGoEXE(t)
	view, err := parseImportView(data)
	if err != nil {
		t.Fatalf("parseImportView: %v", err)
	}
	// Find the import section's trailing zero region.
	end := view.sectionRawEnd
	trailStart := end
	for trailStart > view.importOff && data[trailStart-1] == 0 {
		trailStart--
	}
	slack := end - trailStart
	t.Logf("import section: importOff=0x%x descCount=%d sectionRawEnd=0x%x trailStart=0x%x slack=%d bytes",
		view.importOff, view.descCount, end, trailStart, slack)
	if slack == 0 {
		t.Fatal("Go binary import section has NO trailing zero padding")
	}
}

func TestDebugAddBenignImportsGoBinary(t *testing.T) {
	// The Go toolchain's .idata trailing padding is ~200 bytes: enough for
	// the imported descriptor + one additional benign import. Excess demand
	// must produce an explicit error, not silent silence.
	data := buildDebugGoEXE(t)
	before := parseImportViewForTest(t, data)
	descBefore := before.descCount
	out2, err := AddBenignImports(data, []string{"user32.dll"})
	if err != nil {
		t.Fatalf("AddBenignImports: %v", err)
	}
	after := parseImportViewForTest(t, out2)
	names := after.importedDLLs()
	found := false
	for _, n := range names {
		if n == "user32.dll" {
			found = true
		}
	}
	if !found {
		t.Fatalf("user32.dll not present after injection; imports = %v", names)
	}
	if after.descCount != descBefore+1 {
		t.Fatalf("descCount = %d, want %d", after.descCount, descBefore+1)
	}

	excess := buildDebugGoEXE(t)
	// Ten benign imports exceed any realistic padding, but the growth
	// fallback satisfies even that demand — assert success AND that key DLLs
	// landed in the import table.
	out3, err := AddBenignImports(excess, []string{"user32.dll", "ws2_32.dll", "bcrypt.dll", "shell32.dll", "wininet.dll", "advapi32.dll", "ole32.dll", "crypt32.dll", "winmm.dll", "version.dll"})
	if err != nil {
		t.Fatalf("growth fallback should satisfy heavy demand: %v", err)
	}
	grown := parseImportViewForTest(t, out3)
	for _, want := range []string{"user32.dll", "ws2_32.dll", "bcrypt.dll", "winmm.dll"} {
		found := false
		for _, n := range grown.importedDLLs() {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s missing after growth injection; imports = %v", want, grown.importedDLLs())
		}
	}
}

// TestDebugInjectedImportsPreservedByGo verifies the Windows-loader invariants
// over the rewritten import table: every descriptor resolves, every ILT/IAT
// ends in a null thunk, and a null descriptor terminates the array.
func TestDebugInjectedImportsPreservedByGo(t *testing.T) {
	data := buildDebugGoEXE(t)
	data, err := func() ([]byte, error) {
		return AddBenignImports(data, []string{"user32.dll"})
	}()
	if err != nil {
		t.Fatalf("AddBenignImports: %v", err)
	}
	view := parseImportViewForTest(t, data)
	for i := 0; i < view.descCount; i++ {
		d := view.descriptorAt(i)
		iltRVA, iatRVA := d.OriginalFirstThunk, d.FirstThunk
		if iltRVA == 0 {
			iltRVA = iatRVA // OFT may be omitted; IAT doubles as ILT
		}
		ilt, ok := view.rvaToOffsetSafe(iltRVA)
		if !ok || ilt >= len(data) {
			t.Fatalf("descriptor %d ILT unresolvable", i)
		}
		iat, ok := view.rvaToOffsetSafe(iatRVA)
		if !ok || iat >= len(data) {
			t.Fatalf("descriptor %d IAT unresolvable", i)
		}
		// Walk thunks until a null entry; must be all-zero-free before it
		// (a valid chain) and the terminator must appear before hitting
		// unreadable memory.
		walk := func(base int, label string) {
			for j := 0; j < 256; j++ {
				off := base + j*8
				if off+8 > len(data) {
					t.Fatalf("descriptor %d %s never terminates before EOF", i, label)
				}
				if binary.LittleEndian.Uint64(data[off:]) == 0 {
					return
				}
			}
			t.Fatalf("descriptor %d %s lacks null terminator thunk", i, label)
		}
		walk(ilt, "ILT")
		walk(iat, "IAT")
		// The first thunk of a real chain points at a hint/name pair: hint
		// (2 bytes) followed by a NUL-terminated export name.
		nameOff, ok := view.rvaToOffsetSafe(binary.LittleEndian.Uint32(data[ilt:]))
		if !ok || nameOff+2 >= len(data) {
			t.Fatalf("descriptor %d first thunk does not reference hint/name", i)
		}
		if data[nameOff+2] == 0 {
			t.Fatalf("descriptor %d has empty export name", i)
		}
		// DLL name string must be readable ASCII terminating in ".dll".
		dnOff, ok := view.rvaToOffsetSafe(d.Name)
		if !ok || dnOff >= len(data) {
			t.Fatalf("descriptor %d DLL name unresolvable", i)
		}
		dnEnd := dnOff
		for dnEnd < len(data) && dnEnd-dnOff < 260 && data[dnEnd] != 0 {
			if data[dnEnd] < 0x20 || data[dnEnd] > 0x7E {
				t.Fatalf("descriptor %d DLL name contains non-ASCII bytes", i)
			}
			dnEnd++
		}
		if !strings.HasSuffix(strings.ToLower(string(data[dnOff:dnEnd])), ".dll") {
			t.Fatalf("descriptor %d DLL name %q does not end in .dll", i, string(data[dnOff:dnEnd]))
		}
	}
	d0 := view.descriptorAt(view.descCount)
	if !d0.isNull() {
		t.Fatalf("no null descriptor after table")
	}
	names := view.importedDLLs()
	if !strings.Contains(strings.Join(names, ","), "user32.dll") {
		t.Fatalf("user32.dll lost after verification pass: %v", names)
	}
}

// (The multi-import path through the relocation strategy is not separately
// tested here: the Go-binary tests already force relocation by construction,
// and the fixed-size crafted PEs bound how many imports a hand-packed slack
// region can hold.)
