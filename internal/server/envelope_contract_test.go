package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestSuccessEnvelope is the ratchet for the Option-A response contract:
//
//	success -> {"success": true, "data": <payload>}   (respond)
//	failure -> {"success": false, "error": "<msg>"}   (respondError)
//
// Two things are enforced:
//
//  1. No handler may hand-build the envelope. respond() already wraps exactly
//     once, so `respond(c, gin.H{"success": ...})` would emit a nested
//     {"data": {"success": ...}} body.
//  2. The number of raw `c.JSON(http.StatusOK, ...)` sites per file may only
//     go down. The migration runs in batches; envelope_baseline.txt pins the
//     remainder so a new raw success cannot slip in, and a migrated file must
//     lower its number in the same commit (otherwise the list goes stale and
//     the gate stops meaning anything).
func TestSuccessEnvelope(t *testing.T) {
	const marker = "c.JSON(http.StatusOK"
	const nestedMarker = `respond(c, gin.H{"success"`

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	sort.Strings(files)

	actual := make(map[string]int)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(src)
		if n := strings.Count(text, nestedMarker); n > 0 {
			t.Fatalf("%s hand-builds the success envelope (%d site(s)); call respond(c, payload) "+
				"with the bare payload instead — respond wraps it exactly once", path, n)
		}
		if strings.Contains(text, "func respondSuccess(") {
			t.Fatalf("%s re-declares respondSuccess; it was folded into respond", path)
		}
		if n := strings.Count(text, marker); n > 0 {
			actual[path] = n
		}
	}

	baseline, err := readEnvelopeBaseline("envelope_baseline.txt")
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}

	var grew, shrank []string
	for path, n := range actual {
		want, ok := baseline[path]
		if !ok {
			grew = append(grew, fmt.Sprintf("%s: %d raw site(s), not in envelope_baseline.txt", path, n))
			continue
		}
		if n > want {
			grew = append(grew, fmt.Sprintf("%s: %d raw site(s), baseline says %d", path, n, want))
		}
		if n < want {
			shrank = append(shrank, fmt.Sprintf("%s: %d raw site(s), baseline says %d", path, n, want))
		}
	}
	for path, want := range baseline {
		if _, ok := actual[path]; !ok {
			shrank = append(shrank, fmt.Sprintf("%s: 0 raw site(s), baseline says %d", path, want))
		}
	}
	sort.Strings(grew)
	sort.Strings(shrank)

	if len(grew) > 0 {
		t.Fatalf("new raw JSON 200 responses (use respond(c, payload)):\n  %s", strings.Join(grew, "\n  "))
	}
	if len(shrank) > 0 {
		t.Fatalf("envelope_baseline.txt is stale — lower or remove these counts in the same commit:\n  %s",
			strings.Join(shrank, "\n  "))
	}
}

// readEnvelopeBaseline parses "<file.go> <count>" lines, ignoring blanks and
// '#' comments.
func readEnvelopeBaseline(path string) (map[string]int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int)
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("%s:%d: want `<file> <count>`, got %q", path, i+1, line)
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: bad count %q", path, i+1, fields[1])
		}
		out[fields[0]] = n
	}
	return out, nil
}
