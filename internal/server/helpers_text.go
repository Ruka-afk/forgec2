package server

import (
	"os"
	"strings"
)

// splitAndTrim splits a comma-separated list, trimming whitespace and
// dropping empty entries.
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// joinNonEmpty joins parts with sep, skipping empty segments (used for
// DOMAIN\username rendering where either side may be blank).
func joinNonEmpty(a, b, sep string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + sep + b
	}
}

// listDirEntries is a thin wrapper kept separate from the timeline logic so
// tests can stub directory reads if needed.
func listDirEntries(dir string) ([]os.DirEntry, error) {
	return os.ReadDir(dir)
}

var _ = os.ReadDir
