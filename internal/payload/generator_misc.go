package payload

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// psEscape escapes a value for safe inclusion inside a double-quoted PowerShell
// string. Without it, a crafted value (e.g. from an imported malleable profile)
// containing `"`, a backtick, or `$(...)` could break out of the string or
// execute arbitrary PowerShell on the victim machine.
func psEscape(s string) string {
	s = strings.ReplaceAll(s, "`", "`\"")
	s = strings.ReplaceAll(s, "\"", "`\"")
	s = strings.ReplaceAll(s, "$", "`$")
	return s
}

// buildLdflags constructs the full -ldflags content string for go build.
// Agent runtime configuration is carried in a single XOR-obfuscated blob
// injected via -X main.ConfigBlob (see buildConfigBlobKeyed); everything else is
// resolved from the blob at agent init(). Only build-level flags remain.
// safeBuildFileName strips any directory components from a user-supplied
// output filename so that build artifacts can never be written outside the
// requested output directory via ".." segments or an absolute path (A1).
func safeBuildFileName(name string) string {
	// Accept both "/" and "\" as separators regardless of GOOS: the caller
	// may receive a Windows-style path from a user on Linux CI, and stripping
	// only the native separator would leave directory components in place.
	return filepath.Base(strings.ReplaceAll(name, `\`, `/`))
}

// stripImportLine removes one tab-indented, single-line import of path and
// reports whether it was present.
//
// The closing bracket of the import block is optional because the import may
// be alone in its block (`"鈥redentials"]`) or sit among siblings
// (`"鈥redentials"`). Matching the exact current text is what let this guard
// drift out of sync with transport_utls.go: the matcher expected a bare line
// while the file ended the block on that same line, so stripUTLSCreds failed on
// EVERY slim build 鈥?blocking the artifact instead of catching a regression.
func stripImportLine(text, path string) (string, bool) {
	re := regexp.MustCompile(`(?m)^\t` + regexp.QuoteMeta(`"`+path+`"`) + `\]?\r?\n`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return text, false
	}
	return text[:loc[0]] + text[loc[1]:], true
}

// stripUTLSCreds removes the grpc-dependent tail and import from a copy of
// transport_utls.go source. It errors loudly (failing the build) when the
// marker or import moves, so upstream refactors cannot silently reintroduce
// the grpc dependency into slim builds.
func stripUTLSCreds(src []byte) ([]byte, error) {
	text := string(src)
	stripped, ok := stripImportLine(text, "google.golang.org/grpc/credentials")
	if !ok {
		return nil, fmt.Errorf("slim: grpc credentials import not found in transport_utls.go (refactor?)")
	}
	text = stripped
	idx := strings.Index(text, utlsCredsMarker)
	if idx < 0 {
		return nil, fmt.Errorf("slim: utlsCreds marker not found in transport_utls.go (refactor?)")
	}
	text = text[:idx]
	// The grpc tail was the only user of fmt in this file; drop the import
	// so the slim copy still compiles (verified by TestSmokeSlimWindowsEXE).
	if !strings.Contains(text, "fmt.") {
		if trimmed, ok := stripImportLine(text, "fmt"); ok {
			text = trimmed
		}
	}
	return []byte(text), nil
}
