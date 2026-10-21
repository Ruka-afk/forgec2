package payload

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSmokeSignedWindowsArtifacts builds real signed EXE and DLL artifacts.
// It is opt-in because it runs the full agent compile pipeline.
//
//	FORGEC2_SMOKE_BUILD=1 go test ./internal/payload -run TestSmokeSignedWindowsArtifacts -v
func TestSmokeSignedWindowsArtifacts(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") != "1" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run the real agent build")
	}
	outDir, err := os.MkdirTemp("", "forgec2-sign-smoke-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("output dir: %s", outDir)

	base := ImplantConfig{
		C2URL:           "http://127.0.0.1:8080",
		Protocol:        "http",
		Interval:        9,
		Jitter:          30,
		UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		BeaconKey:       "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
		ListenerID:      1,
		Filename:        "smoketest",
		BeaconTransport: "http",
		Architecture:    "amd64",
		FileDescription: "Invoice",
		CompanyName:     "Acme",
	}

	self := base
	self.SignMode = "self"
	self.SignOrg = "Acme Software"
	self.OriginalFilename = "invoice.pdf.exe"
	exePath, err := GenerateWindowsEXE(self, outDir)
	if err != nil {
		t.Fatalf("self-signed EXE build: %v", err)
	}
	assertSmokeSigned(t, exePath, "invoice.pdf.exe")

	cert, key := makeTestCodeSigningCert(t, testCertOptions{})
	pfx := encodeTestPFX(t, cert, key, "smoke-pw")
	dll := base
	dll.SignMode = "pfx"
	dll.SignPFX = pfx
	dll.SignPFXPassword = "smoke-pw"
	dll.OriginalFilename = "module.dll"
	dllPath, err := GenerateWindowsDLL(dll, outDir)
	if err != nil {
		t.Fatalf("PFX-signed DLL build: %v", err)
	}
	assertSmokeSigned(t, dllPath, "module.dll")

	if runtime.GOOS == "windows" {
		for _, path := range []string{exePath, dllPath} {
			out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
				"(Get-AuthenticodeSignature -LiteralPath '"+strings.ReplaceAll(path, "'", "''")+"').Status").CombinedOutput()
			if err != nil {
				t.Fatalf("Get-AuthenticodeSignature(%s): %v\n%s", filepath.Base(path), err, out)
			}
			status := strings.TrimSpace(string(out))
			switch status {
			case "Valid", "UnknownError", "NotTrusted":
				t.Logf("%s Authenticode status: %s", filepath.Base(path), status)
			default:
				t.Fatalf("%s Authenticode status = %s, want a structurally valid untrusted signature", filepath.Base(path), status)
			}
		}
	}
}

func assertSmokeSigned(t *testing.T, path, originalName string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := InspectPESignature(data)
	if err != nil {
		t.Fatalf("%s: inspect signature: %v", filepath.Base(path), err)
	}
	if info.Signer == nil || len(info.Digest) != 32 {
		t.Fatalf("%s: incomplete signature info", filepath.Base(path))
	}
	if err := VerifyPESignature(data); err != nil {
		t.Fatalf("%s: verify signature: %v", filepath.Base(path), err)
	}
	if !bytes.Contains(data, utf16LE(originalName)) {
		t.Fatalf("%s: OriginalFilename %q missing from VersionInfo", filepath.Base(path), originalName)
	}
	if bytes.Contains(data, []byte("forgec2_agent")) {
		t.Fatalf("%s: internal build name leaked into the binary", filepath.Base(path))
	}
}
