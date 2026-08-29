package payload

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

// tinyShellcode is a harmless two-instruction stub (ret) encoded as x64.
var tinyShellcode = []byte("\xEB\x02\x90\x90\xC3")

func decodeResult(t *testing.T, exe []byte, encoded []byte, method ShellcodeEncode, key []byte, original []byte) {
	t.Helper()
	idx := bytes.Index(exe, encoded)
	if idx < 0 {
		t.Fatal("encoded shellcode blob not found inside the built loader")
	}
	decoded, err := DecodeShellcode(exe[idx:idx+len(encoded)], method, key)
	if err != nil {
		t.Fatalf("DecodeShellcode: %v", err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatal("loader-embedded blob does not decode back to the original shellcode")
	}
}

func assertPE(t *testing.T, data []byte, wantMachine uint16) {
	t.Helper()
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		t.Fatal("built artifact is not a PE (missing MZ header)")
	}
	peOff := int(data[0x3C]) | int(data[0x3D])<<8
	if peOff+24 >= len(data) || string(data[peOff:peOff+4]) != "PE\x00\x00" {
		t.Fatal("built artifact is missing the PE signature")
	}
	got := binary.LittleEndian.Uint16(data[peOff+4:])
	if got != wantMachine {
		t.Fatalf("unexpected PE machine 0x%04x, want 0x%04x", got, wantMachine)
	}
}

func peTimestamp(data []byte) uint32 {
	peOff := int(data[0x3C]) | int(data[0x3D])<<8
	return binary.LittleEndian.Uint32(data[peOff+8:])
}

func TestBuildArtifact_EXELoader(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a Go toolchain build")
	}
	key := []byte{0x5A, 0x3C}
	encoded, err := EncodeShellcode(tinyShellcode, EncodeXOR, key)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	req := BuildArtifactRequest{
		OutputType:     "exe",
		ShellcodeB64:   base64.StdEncoding.EncodeToString(tinyShellcode),
		EncodeType:     "xor",
		EncodeKeyHex:   "5a3c",
		EntryPoint:     "thread",
		Timestamp:      "custom",
		TimestampDate:  "2001-02-03",
		PESectionText:  "VECT0R",
		ImportDLLs:     "ws2_32.dll",
		OutputFilename: "staged.exe",
	}
	artifact, filename, err := BuildArtifact(req, "data")
	if err != nil {
		t.Fatalf("BuildArtifact: %v", err)
	}
	assertPE(t, artifact, 0x8664)

	wantTS := uint32(time.Date(2001, 2, 3, 0, 0, 0, 0, time.UTC).Unix())
	if got := peTimestamp(artifact); got != wantTS {
		t.Fatalf("PE timestamp = %d, want %d", got, wantTS)
	}
	if !bytes.Contains(artifact, []byte("VECT0R")) {
		t.Fatal("custom section name .text -> VECT0R not present in built PE")
	}
	if !bytes.Contains(bytes.ToLower(artifact), []byte("ws2_32.dll")) {
		t.Fatal("benign import ws2_32.dll not present in built PE")
	}
	if filename != "staged.exe" {
		t.Fatalf("filename = %q, want staged.exe", filename)
	}
	decodeResult(t, artifact, encoded, EncodeXOR, key, tinyShellcode)
}

func TestBuildArtifact_LoaderMethods(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a Go toolchain build")
	}
	cases := []struct {
		name   string
		method ShellcodeEncode
		key    string
	}{
		{"none", EncodeNone, "aa"},
		{"xor", EncodeXOR, "5a3c"},
		{"aes", EncodeAES, "5a3c"},
		{"sgn", EncodeSGN, "5a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The key travels as hex (encode_key_hex) and is hex-decoded by
			// BuildArtifact; the same decoding must be applied here so both
			// sides encode with the identical key.
			key, err := hex.DecodeString(tc.key)
			if err != nil {
				t.Fatalf("decode key hex: %v", err)
			}
			encoded, err := EncodeShellcode(tinyShellcode, tc.method, key)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			req := BuildArtifactRequest{
				OutputType:   "exe",
				ShellcodeB64: base64.StdEncoding.EncodeToString(tinyShellcode),
				EncodeType:   string(tc.method),
				EncodeKeyHex: tc.key,
				EntryPoint:   "callback",
			}
			artifact, _, err := BuildArtifact(req, "data")
			if err != nil {
				t.Fatalf("BuildArtifact: %v", err)
			}
			assertPE(t, artifact, 0x8664)
			decodeResult(t, artifact, encoded, tc.method, key, tinyShellcode)
			if tc.method == EncodeSGN {
				idx := bytes.Index(artifact, encoded)
				if idx < 0 {
					t.Fatal("sgn blob not found in loader")
				}
				if got := encoded[sgnKeyByteOffset]; got != key[0] {
					t.Fatalf("sgn stub key byte = 0x%02X, want 0x%02X", got, key[0])
				}
			}
		})
	}
}

func TestBuildArtifact_ServiceEXEAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a Go toolchain build")
	}
	req := BuildArtifactRequest{
		OutputType:   "service_exe",
		ShellcodeB64: base64.StdEncoding.EncodeToString(tinyShellcode),
		EntryPoint:   "direct",
	}
	artifact, filename, err := BuildArtifact(req, "data")
	if err != nil {
		t.Fatalf("BuildArtifact: %v", err)
	}
	assertPE(t, artifact, 0x8664)
	if !strings.HasSuffix(filename, ".exe") {
		t.Fatalf("filename = %q, want .exe suffix", filename)
	}
}

func TestBuildArtifact_PS1Loader(t *testing.T) {
	cases := []struct {
		name   string
		method ShellcodeEncode
		key    string
	}{
		{"none", EncodeNone, "aa"},
		{"xor", EncodeXOR, "5a3c"},
		{"aes", EncodeAES, "5a3c"},
		{"sgn", EncodeSGN, "5a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := BuildArtifactRequest{
				OutputType:   "ps1",
				ShellcodeB64: base64.StdEncoding.EncodeToString(tinyShellcode),
				EncodeType:   string(tc.method),
				EncodeKeyHex: tc.key,
			}
			script, filename, err := BuildArtifact(req, "data")
			if err != nil {
				t.Fatalf("BuildArtifact: %v", err)
			}
			s := string(script)
			if !strings.HasSuffix(filename, ".ps1") {
				t.Fatalf("filename = %q, want .ps1 suffix", filename)
			}
			for _, want := range []string{
				"FromBase64String",
				"VirtualAlloc",
				"VirtualProtect",
				"CreateThread",
				"WaitForSingleObject",
				string(tc.method),
			} {
				if !strings.Contains(s, want) {
					t.Fatalf("ps1 loader missing %q", want)
				}
			}
			if tc.method == EncodeXOR {
				if !strings.Contains(s, "-bxor") {
					t.Fatal("xor ps1 loader missing the decode loop")
				}
			}
			if tc.method == EncodeAES {
				if !strings.Contains(s, "CipherMode") {
					t.Fatal("aes ps1 loader missing the CTR decode loop")
				}
			}
		})
	}
}

func TestBuildArtifact_RawPassthrough(t *testing.T) {
	raw := []byte("\xEB\x10\x90\x90\xC3")
	req := BuildArtifactRequest{
		OutputType:   "raw",
		ShellcodeB64: base64.StdEncoding.EncodeToString(raw),
	}
	artifact, _, err := BuildArtifact(req, "data")
	if err != nil {
		t.Fatalf("BuildArtifact: %v", err)
	}
	if !bytes.Equal(artifact, raw) {
		t.Fatal("raw output must return the unencoded payload unchanged")
	}
}

func TestBuildArtifact_RawEXEPassthroughAndTransforms(t *testing.T) {
	// Use a loader-built EXE as the "uploaded" PE: it is a real image with an
	// import table, so section renames and benign imports must be applied.
	req := BuildArtifactRequest{
		OutputType:   "exe",
		ShellcodeB64: base64.StdEncoding.EncodeToString(tinyShellcode),
	}
	exe, _, err := BuildArtifact(req, "data")
	if err != nil {
		t.Fatalf("BuildArtifact(base): %v", err)
	}

	rawReq := BuildArtifactRequest{
		RawEXEB64:     base64.StdEncoding.EncodeToString(exe),
		PESectionText: "BENIGNX",
		ImportDLLs:    "ws2_32.dll",
	}
	artifact, _, err := BuildArtifact(rawReq, "data")
	if err != nil {
		t.Fatalf("BuildArtifact(raw): %v", err)
	}
	assertPE(t, artifact, 0x8664)
	if !bytes.Contains(artifact, []byte("BENIGNX")) {
		t.Fatal("custom section name not applied to raw exe passthrough")
	}
	if !bytes.Contains(bytes.ToLower(artifact), []byte("ws2_32.dll")) {
		t.Fatal("benign import not applied to raw exe passthrough")
	}
}

func TestBuildArtifact_ValidationErrors(t *testing.T) {
	tiny := base64.StdEncoding.EncodeToString(tinyShellcode)
	cases := []struct {
		name string
		req  BuildArtifactRequest
	}{
		{"no payload", BuildArtifactRequest{OutputType: "exe"}},
		{"bad base64", BuildArtifactRequest{ShellcodeB64: "!!!", OutputType: "exe"}},
		{"unknown encoder", BuildArtifactRequest{ShellcodeB64: tiny, EncodeType: "rc4", OutputType: "exe"}},
		{"dll output", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "dll"}},
		{"cert self_signed needs pe", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "raw", CertOption: "self_signed"}},
		{"cert unknown", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "raw", CertOption: "authenticode"}},
		{"tls entry", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "exe", EntryPoint: "tls"}},
		{"unknown entry", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "exe", EntryPoint: "moot"}},
		{"unknown output", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "elf"}},
		{"bad key hex", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "ps1", EncodeKeyHex: "zz"}},
		{"raw not pe", BuildArtifactRequest{RawEXEB64: base64.StdEncoding.EncodeToString([]byte("MZ-X-not-a-pe"))}},
		{"encode on raw", BuildArtifactRequest{RawEXEB64: base64.StdEncoding.EncodeToString([]byte("MZ-X-not-a-pe")), EncodeType: "xor"}},
		{"bad custom timestamp", BuildArtifactRequest{ShellcodeB64: tiny, OutputType: "exe", Timestamp: "custom", TimestampDate: "not-a-date"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := BuildArtifact(tc.req, "data")
			if err == nil {
				t.Fatal("expected validation error")
			}
			var verr *ArtifactValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected ArtifactValidationError, got %T: %v", err, err)
			}
		})
	}
}
