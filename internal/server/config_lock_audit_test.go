package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestConfigReadsAreLocked is the gate for the config read discipline.
//
// `s.cfg` is a *config.Config that the reload watcher rewrites in place under
// s.configMu's write lock (CopyFrom copies every section, not only the changed
// one). A reader that does not hold the lock therefore races any successful hot
// reload — the class fixed for the settings surface, DNS, RoE and the
// settings-writable fields. This test walks the package sources and fails on a
// function that reads s.cfg without ever acquiring configMu.
//
// Boot-only code (runs before the HTTP server accepts requests, so no reload
// can overlap) opts out with a marker on the function line:
//
//	func (s *Server) initThing() { // config-lock: boot-only, no server serving yet
//
// The marker is intentionally per-function and carries a reason: an exemption
// without one is the bug this gate exists to catch.
func TestConfigReadsAreLocked(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	type violation struct {
		file string
		line int
		fn   string
		refs []int
	}
	var found []violation

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		lines := strings.Split(string(src), "\n")

		type fnRange struct {
			start int
			sig   string
		}
		var fns []fnRange
		for i, line := range lines {
			if strings.HasPrefix(line, "func ") {
				fns = append(fns, fnRange{start: i, sig: line})
			}
		}

		for idx, fn := range fns {
			end := len(lines)
			if idx+1 < len(fns) {
				end = fns[idx+1].start
			}
			var refs []int
			locked := false
			for i := fn.start; i < end; i++ {
				line := lines[i]
				if strings.Contains(line, "configMu.Lock()") || strings.Contains(line, "configMu.RLock()") {
					locked = true
				}
				if strings.Contains(line, "s.cfg.") {
					refs = append(refs, i+1)
				}
			}
			if len(refs) == 0 || locked {
				continue
			}
			if strings.Contains(fn.sig, "// config-lock:") {
				continue
			}
			found = append(found, violation{file: path, line: fn.start + 1, fn: strings.TrimSpace(fn.sig), refs: refs})
		}
	}

	if len(found) == 0 {
		return
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].file != found[j].file {
			return found[i].file < found[j].file
		}
		return found[i].line < found[j].line
	})
	var b strings.Builder
	for _, v := range found {
		fmt.Fprintf(&b, "  %s:%d  %s  (refs %v)\n", v.file, v.line, v.fn, v.refs)
	}
	t.Fatalf("functions read s.cfg without holding configMu (%d); take RLock around the\n"+
		"read (or annotate boot-only code with `// config-lock: <reason>`):\n%s", len(found), b.String())
}
