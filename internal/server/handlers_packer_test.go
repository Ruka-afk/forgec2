package server

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// tinyShellcode mirrors the payload package's stub so the handler test can
// prove the artifact round-trips through the real loader without reaching into
// unexported package state.
var tinyShellcode = []byte("\xEB\x02\x90\x90\xC3")

func packerTestServer(t *testing.T) (*Server, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	d := testutil.SetupTestDB(t)
	cfg := &config.Config{}
	cfg.Server.DataDir = t.TempDir()
	s := &Server{db: d, cfg: cfg}
	w := httptest.NewRecorder()
	return s, w
}

func setPostJSON(t *testing.T, c *gin.Context, body map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
}

func respondJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("handler did not return JSON: %v; body=%s", err, w.Body.String())
	}
	return m
}

func TestHandlePackerTemplates_OnlyImplementedTemplates(t *testing.T) {
	s, w := packerTestServer(t)
	c, _ := gin.CreateTestContext(w)
	c.Set("user", "admin")
	s.handleAPIPackerTemplates(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	m := respondJSON(t, w)
	list, _ := m["templates"].([]any)
	if len(list) == 0 {
		t.Fatal("no templates returned")
	}
	for _, item := range list {
		tmpl, _ := item.(map[string]any)
		out, _ := tmpl["output_type"].(string)
		if out != "exe" {
			t.Fatalf("template %v advertises output_type %q; only exe loaders are built", tmpl["name"], out)
		}
	}
}

func TestHandlePackerInfo_AdvertisesOnlyRealOptions(t *testing.T) {
	s, w := packerTestServer(t)
	c, _ := gin.CreateTestContext(w)
	c.Set("user", "admin")
	s.handleAPIPackerInfo(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	m := respondJSON(t, w)

	encTypes := strSlice(t, m["encode_types"])
	if strings.Join(encTypes, ",") != "none,xor,aes,sgn" {
		t.Fatalf("encode_types = %v", encTypes)
	}
	entryPts := strSlice(t, m["entry_points"])
	if strings.Join(entryPts, ",") != "direct,thread,callback" {
		t.Fatalf("entry_points = %v", entryPts)
	}
	certs := strSlice(t, m["cert_options"])
	if strings.Join(certs, ",") != "none" {
		t.Fatalf("cert_options = %v; unsupported certificate options must not be advertised", certs)
	}
	outputs := strSlice(t, m["output_types"])
	if containsString(outputs, "dll") {
		t.Fatal("output_types advertises dll, which has no working implementation")
	}
}

func strSlice(t *testing.T, v any) []string {
	t.Helper()
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("expected array, got %T", v)
	}
	out := make([]string, len(arr))
	for i, x := range arr {
		out[i], _ = x.(string)
	}
	return out
}

func containsString(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

// buildTestPEFixture compiles a tiny windows/amd64 Go binary so the bundle and
// artifact handlers exercise the real load path instead of a hand-crafted PE.
func buildTestPEFixture(t *testing.T) []byte {
	t.Helper()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(src, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmp, "probe.exe"), src)
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "probe.exe"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHandlePackerBundle_AppliesTransformations(t *testing.T) {
	s, w := packerTestServer(t)
	pe := buildTestPEFixture(t)

	c, _ := gin.CreateTestContext(w)
	c.Set("user", "admin")
	setPostJSON(t, c, map[string]any{
		"agent_exe":       base64.StdEncoding.EncodeToString(pe),
		"pe_section_text": "CODE",
		"timestamp":       "random",
		"import_dlls":     "ws2_32.dll",
	})
	c.Request.Method = http.MethodPost
	s.handlePackerBundle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	m := respondJSON(t, w)
	data, _ := m["data"].(string)
	got, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(got) < 0x40 || got[0] != 'M' || got[1] != 'Z' {
		t.Fatalf("bundle did not decode to a PE image: err=%v len=%d", err, len(got))
	}
	if !bytes.Contains(got, []byte("ws2_32.dll\x00")) {
		t.Fatal("bundle output missing ws2_32.dll import")
	}
	if !sectionNameApplied(got, "CODE") {
		t.Fatal("bundle output did not rename the .text section to CODE")
	}
}

// The import section of Go-built binaries (the worst realistic case) has
// limited trailing zero slack: one new import fits, two do not. The endpoint
// must fail loudly instead of silently dropping the request.
func TestHandlePackerBundle_OverflowFailsLoudly(t *testing.T) {
	s, _ := packerTestServer(t)
	pe := buildTestPEFixture(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user", "admin")
	setPostJSON(t, c, map[string]any{
		"agent_exe":   base64.StdEncoding.EncodeToString(pe),
		"import_dlls": "advapi32.dll, user32.dll",
	})
	s.handlePackerBundle(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "no zero slack space") {
		t.Fatalf("error body %q does not explain the capacity failure", w.Body.String())
	}
}

func sectionNameApplied(data []byte, want string) bool {
	if len(data) < 0x100 || data[0] != 'M' || data[1] != 'Z' {
		return false
	}
	peOff := int(data[0x3C]) | int(data[0x3D])<<8
	if data[peOff] != 'P' || data[peOff+1] != 'E' {
		return false
	}
	numSec := int(data[peOff+6]) | int(data[peOff+7])<<8
	optSize := int(data[peOff+20]) | int(data[peOff+21])<<8
	secOff := peOff + 4 + 20 + optSize
	for i := 0; i < numSec; i++ {
		name := strings.TrimRight(string(data[secOff+i*40:secOff+i*40+8]), "\x00")
		if name == want {
			return true
		}
	}
	return false
}

func TestHandlePackerBundle_RejectsNonPE(t *testing.T) {
	s, w := packerTestServer(t)
	c, _ := gin.CreateTestContext(w)
	c.Set("user", "admin")
	setPostJSON(t, c, map[string]any{
		"agent_exe": base64.StdEncoding.EncodeToString([]byte("not a pe")),
	})
	s.handlePackerBundle(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandlePackerArtifact_ValidationErrors(t *testing.T) {
	s, _ := packerTestServer(t)
	cases := []struct {
		name string
		body map[string]any
		want string
	}{
		{"no_payload", map[string]any{}, "either shellcode_b64 or raw_exe_b64 is required"},
		{"dll_output", map[string]any{"shellcode_b64": b64(t, tinyShellcode), "output_type": "dll", "encode_type": "xor"}, "dll output is not supported"},
		{"fake_cert", map[string]any{"shellcode_b64": b64(t, tinyShellcode), "cert_option": "self_signed"}, "certificate option"},
		{"fake_entry", map[string]any{"shellcode_b64": b64(t, tinyShellcode), "entry_point": "tls"}, "tls entry point is not implemented"},
		{"bad_key", map[string]any{"shellcode_b64": b64(t, tinyShellcode), "encode_key_hex": "zz"}, "invalid encode key hex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w2 := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w2)
			c.Set("user", "admin")
			setPostJSON(t, c, tc.body)
			s.handlePackerArtifact(c)
			if w2.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d; body=%s", w2.Code, w2.Body.String())
			}
			if !strings.Contains(w2.Body.String(), tc.want) {
				t.Fatalf("error body %q does not mention %q", w2.Body.String(), tc.want)
			}
		})
	}
}

func b64(t *testing.T, b []byte) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(b)
}

func TestHandlePackerArtifact_BuildsXORLoader(t *testing.T) {
	s, w := packerTestServer(t)

	key := []byte{0x5A, 0x3C}
	encoded, err := payload.EncodeShellcode(tinyShellcode, payload.EncodeXOR, key)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	c, _ := gin.CreateTestContext(w)
	c.Set("user", "admin")
	setPostJSON(t, c, map[string]any{
		"shellcode_b64":  b64(t, tinyShellcode),
		"encode_type":    "xor",
		"encode_key_hex": hex.EncodeToString(key),
		"output_type":    "exe",
	})
	c.Request.Method = http.MethodPost
	s.handlePackerArtifact(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	m := respondJSON(t, w)
	data, _ := m["data"].(string)
	if filename, _ := m["filename"].(string); !strings.HasSuffix(filename, ".exe") {
		t.Fatalf("filename %q does not end with .exe", filename)
	}
	exe, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(exe) < 0x40 || exe[0] != 'M' || exe[1] != 'Z' {
		t.Fatalf("artifact is not a PE image: err=%v len=%d", err, len(exe))
	}

	idx := bytes.Index(exe, encoded)
	if idx < 0 {
		t.Fatal("encoded shellcode blob not found inside the built loader")
	}
	decoded, err := payload.DecodeShellcode(exe[idx:idx+len(encoded)], payload.EncodeXOR, key)
	if err != nil {
		t.Fatalf("decode embedded blob back failed: %v", err)
	}
	if !bytes.Equal(decoded, tinyShellcode) {
		t.Fatalf("decoded blob mismatches: got %d bytes want %d", len(decoded), len(tinyShellcode))
	}
}
