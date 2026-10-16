package payload

import (
	"os"
	"path/filepath"
	"testing"
)

// makePE builds a minimal but structurally valid PE header buffer for testing
// stripPEArtifacts / validatePE without invoking the Go toolchain.
func makePE(t *testing.T, machine uint16, pe32plus bool) []byte {
	t.Helper()
	eLfanew := 0x80
	buf := make([]byte, eLfanew+0x200)
	buf[0], buf[1] = 'M', 'Z'
	buf[0x3C] = byte(eLfanew)
	buf[0x3D], buf[0x3E], buf[0x3F] = 0, 0, 0
	buf[eLfanew], buf[eLfanew+1], buf[eLfanew+2], buf[eLfanew+3] = 'P', 'E', 0, 0
	buf[eLfanew+4] = byte(machine)
	buf[eLfanew+5] = byte(machine >> 8)
	magic := uint16(0x20b)
	if !pe32plus {
		magic = 0x10b
	}
	buf[eLfanew+0x18] = byte(magic)
	buf[eLfanew+0x19] = byte(magic >> 8)
	return buf
}

func TestValidatePEAcceptsValid(t *testing.T) {
	dir, _ := os.MkdirTemp("", "pe-*")
	defer os.RemoveAll(dir)
	for _, tc := range []struct {
		name    string
		machine uint16
		plus    bool
	}{
		{"amd64", 0x8664, true},
		{"386", 0x014c, false},
	} {
		p := filepath.Join(dir, tc.name+".exe")
		if err := os.WriteFile(p, makePE(t, tc.machine, tc.plus), 0644); err != nil {
			t.Fatal(err)
		}
		if err := validatePE(p, tc.machine); err != nil {
			t.Fatalf("%s PE rejected: %v", tc.name, err)
		}
	}
}

func TestValidatePERejectsWrongMachine(t *testing.T) {
	dir, _ := os.MkdirTemp("", "pe-*")
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "x.exe")
	if err := os.WriteFile(p, makePE(t, 0x014c, true), 0644); err != nil {
		t.Fatal(err)
	}
	// Expecting amd64 but the binary is 386 → must be rejected.
	if err := validatePE(p, 0x8664); err == nil {
		t.Fatal("expected wrong-machine rejection for 386 binary validated as amd64")
	}
}

func TestStripPEArtifactsKeepsPEValid(t *testing.T) {
	// PE32+ (amd64)
	dir, _ := os.MkdirTemp("", "pe-*")
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "a.exe")
	if err := os.WriteFile(p, makePE(t, 0x8664, true), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validatePE(p, 0x8664); err != nil {
		t.Fatalf("precondition failed: %v", err)
	}
	if err := stripPEArtifacts(p); err != nil {
		t.Fatalf("stripPEArtifacts failed: %v", err)
	}
	if err := validatePE(p, 0x8664); err != nil {
		t.Fatalf("stripPEArtifacts corrupted amd64 PE: %v", err)
	}

	// PE32 (386) — the old code zeroed the debug dir at the PE64 offset and
	// corrupted this layout. The hardened code must keep it valid.
	p2 := filepath.Join(dir, "b.exe")
	if err := os.WriteFile(p2, makePE(t, 0x014c, false), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stripPEArtifacts(p2); err != nil {
		t.Fatalf("stripPEArtifacts failed: %v", err)
	}
	if err := validatePE(p2, 0x014c); err != nil {
		t.Fatalf("stripPEArtifacts corrupted 386 PE: %v", err)
	}
}
