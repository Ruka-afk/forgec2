package payload

import (
	"fmt"
	"path/filepath"
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
	return filepath.Base(name)
}

// stripUTLSCreds removes the grpc-dependent tail and import from a copy of
// transport_utls.go source. It errors loudly (failing the build) when the
// marker or import moves, so upstream refactors cannot silently reintroduce
// the grpc dependency into slim builds.
func stripUTLSCreds(src []byte) ([]byte, error) {
	text := string(src)
	imp := "\t\"google.golang.org/grpc/credentials\"\n"
	if !strings.Contains(text, imp) {
		return nil, fmt.Errorf("slim: grpc credentials import not found in transport_utls.go (refactor?)")
	}
	text = strings.Replace(text, imp, "", 1)
	idx := strings.Index(text, utlsCredsMarker)
	if idx < 0 {
		return nil, fmt.Errorf("slim: utlsCreds marker not found in transport_utls.go (refactor?)")
	}
	text = text[:idx]
	// The grpc tail was the only user of fmt in this file; drop the import
	// so the slim copy still compiles (verified by TestSmokeSlimWindowsEXE).
	if strings.Contains(text, "\t\"fmt\"\n") && !strings.Contains(text, "fmt.") {
		text = strings.Replace(text, "\t\"fmt\"\n", "", 1)
	}
	return []byte(text), nil
}
