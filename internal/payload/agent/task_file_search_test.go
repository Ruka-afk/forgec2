//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseFileSearchOptsJSON(t *testing.T) {
	opts, err := parseFileSearchOpts("/tmp", "*.gif,*.webp", `{
		"patterns":["*.jpg","*.png"],
		"categories":["image","video"],
		"min_size":10,
		"max_size":1000,
		"modified_after":"2026-01-01",
		"modified_before":"2026-12-31",
		"include_hidden":true,
		"max_depth":99,
		"max_results":99999,
		"timeout_seconds":999,
		"max_result_bytes":99999999
	}`)
	if err != nil {
		t.Fatalf("parseFileSearchOpts: %v", err)
	}
	if opts.root != "/tmp" {
		t.Fatalf("root = %q", opts.root)
	}
	if len(opts.patterns) != 2 || opts.patterns[0] != "*.jpg" || opts.patterns[1] != "*.png" {
		t.Fatalf("patterns = %#v, want JSON patterns to override the command", opts.patterns)
	}
	if !opts.categories["image"] || !opts.categories["video"] || len(opts.categories) != 2 {
		t.Fatalf("categories = %#v", opts.categories)
	}
	if opts.minSize != 10 || opts.maxSize != 1000 || !opts.includeHidden {
		t.Fatalf("filters = min %d max %d hidden %v", opts.minSize, opts.maxSize, opts.includeHidden)
	}
	if opts.maxDepth != fileSearchHardMaxDepth || opts.maxResults != fileSearchHardMaxResults {
		t.Fatalf("limits = depth %d results %d", opts.maxDepth, opts.maxResults)
	}
	if opts.timeout != fileSearchHardTimeout {
		t.Fatalf("timeout = %v", opts.timeout)
	}
	if opts.maxResultBytes != fileSearchMaxResultBytes {
		t.Fatalf("maxResultBytes = %d", opts.maxResultBytes)
	}
	if opts.modifiedAfter.IsZero() || opts.modifiedBefore.IsZero() {
		t.Fatal("date filters were not parsed")
	}
}

func TestParseFileSearchOptsDefaultsAndLegacyData(t *testing.T) {
	opts, err := parseFileSearchOpts("", "", "")
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if opts.root == "" || len(opts.patterns) != 1 || opts.patterns[0] != "*" {
		t.Fatalf("defaults root=%q patterns=%#v", opts.root, opts.patterns)
	}
	if opts.maxDepth != fileSearchDefaultMaxDepth || opts.maxResults != fileSearchDefaultMaxResults {
		t.Fatalf("defaults depth=%d results=%d", opts.maxDepth, opts.maxResults)
	}
	if opts.timeout != fileSearchDefaultTimeout {
		t.Fatalf("default timeout = %v", opts.timeout)
	}

	opts, err = parseFileSearchOpts("/tmp", "*.jpg", "category=image,category=video,max_depth=3,max_results=7,include_hidden=1")
	if err != nil {
		t.Fatalf("legacy data: %v", err)
	}
	if !opts.includeHidden || opts.maxDepth != 3 || opts.maxResults != 7 {
		t.Fatalf("legacy options = hidden %v depth %d results %d", opts.includeHidden, opts.maxDepth, opts.maxResults)
	}
	if !opts.categories["image"] || !opts.categories["video"] {
		t.Fatalf("legacy categories = %#v", opts.categories)
	}

	if _, err := parseFileSearchOpts("/tmp", "", `{"categories":["nope"]}`); err == nil {
		t.Fatal("unknown category must be rejected")
	}
	if _, err := parseFileSearchOpts("/tmp", "", `{"min_size":10,"max_size":1}`); err == nil {
		t.Fatal("min_size > max_size must be rejected")
	}
	if _, err := parseFileSearchOpts("/tmp", "", `{"modified_after":"not-a-date"}`); err == nil {
		t.Fatal("bad date must be rejected")
	}
}

func writeSearchTestFile(t *testing.T, root, rel string, size int, mod time.Time) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", rel, err)
	}
	return p
}

func decodeFileSearchResult(t *testing.T, raw string) fileSearchResult {
	t.Helper()
	var got fileSearchResult
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	return got
}

func TestRunFileSearchFiltersMetadataAndSkipsTrees(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	recent := time.Now().Add(-1 * time.Hour).Truncate(time.Second)

	writeSearchTestFile(t, root, "pic-old.jpg", 100, old)
	writeSearchTestFile(t, root, "pic-new.JPG", 100, recent)
	writeSearchTestFile(t, root, "clip.mp4", 200, recent)
	writeSearchTestFile(t, root, "notes.txt", 50, recent)
	writeSearchTestFile(t, root, "keys/id_rsa.pem", 60, recent)
	writeSearchTestFile(t, root, ".hidden.png", 70, recent)
	writeSearchTestFile(t, root, "deep/one/two.png", 80, recent.Add(-time.Minute))
	writeSearchTestFile(t, root, ".git/ignored.png", 90, recent)
	writeSearchTestFile(t, root, "node_modules/ignored.png", 90, recent)

	raw, err := runFileSearch(fileSearchOpts{
		root:           root,
		patterns:       []string{"*"},
		categories:     map[string]bool{"image": true},
		maxDepth:       8,
		maxResults:     100,
		timeout:        5 * time.Second,
		maxResultBytes: fileSearchMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("runFileSearch: %v", err)
	}
	got := decodeFileSearchResult(t, raw)
	if len(got.Hits) != 3 {
		t.Fatalf("hits = %d, want 3: %#v", len(got.Hits), got.Hits)
	}
	byName := map[string]fileSearchHit{}
	for _, h := range got.Hits {
		byName[h.Name] = h
		if h.Category != "image" {
			t.Errorf("%s category = %q", h.Name, h.Category)
		}
		if h.Size <= 0 || h.Modified == "" || h.Path == "" {
			t.Errorf("%s incomplete hit: %#v", h.Name, h)
		}
	}
	for _, name := range []string{"pic-old.jpg", "pic-new.JPG", "two.png"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("missing %s in %#v", name, byName)
		}
	}
	if _, ok := byName["ignored.png"]; ok {
		t.Error("a skipped directory leaked into the result")
	}
	if _, ok := byName[".hidden.png"]; ok {
		t.Error("hidden file leaked without include_hidden")
	}
	if got.Scanned != 6 {
		t.Errorf("scanned = %d, want 6", got.Scanned)
	}
	if got.Root != root {
		t.Errorf("root = %q, want %q", got.Root, root)
	}
	if len(got.Categories) != 1 || got.Categories[0] != "image" {
		t.Errorf("categories = %#v", got.Categories)
	}
	if got.Truncated {
		t.Errorf("unexpected truncation: %s", got.TruncatedReason)
	}
	// Newest first, then path.
	if got.Hits[0].Name != "pic-new.JPG" {
		t.Errorf("first hit = %q, want newest pic-new.JPG", got.Hits[0].Name)
	}
}

func TestRunFileSearchHiddenSizeDateAndLimits(t *testing.T) {
	root := t.TempDir()
	now := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	old := now.Add(-72 * time.Hour)
	writeSearchTestFile(t, root, ".hidden.jpg", 10, now)
	writeSearchTestFile(t, root, "small.jpg", 10, now)
	writeSearchTestFile(t, root, "large.jpg", 1000, now)
	writeSearchTestFile(t, root, "old.jpg", 500, old)

	raw, err := runFileSearch(fileSearchOpts{
		root:           root,
		patterns:       []string{"*.jpg"},
		categories:     map[string]bool{"image": true},
		minSize:        100,
		maxSize:        2000,
		modifiedAfter:  now.Add(-24 * time.Hour),
		maxDepth:       8,
		maxResults:     100,
		timeout:        5 * time.Second,
		maxResultBytes: fileSearchMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("runFileSearch: %v", err)
	}
	got := decodeFileSearchResult(t, raw)
	if len(got.Hits) != 1 || got.Hits[0].Name != "large.jpg" {
		t.Fatalf("filtered hits = %#v, want large.jpg only", got.Hits)
	}

	raw, err = runFileSearch(fileSearchOpts{
		root:           root,
		patterns:       []string{"*.jpg"},
		includeHidden:  true,
		maxDepth:       8,
		maxResults:     100,
		timeout:        5 * time.Second,
		maxResultBytes: fileSearchMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("hidden run: %v", err)
	}
	got = decodeFileSearchResult(t, raw)
	if len(got.Hits) != 4 {
		t.Fatalf("hidden hits = %d, want 4", len(got.Hits))
	}

	raw, err = runFileSearch(fileSearchOpts{
		root:           root,
		patterns:       []string{"*.jpg"},
		maxDepth:       8,
		maxResults:     2,
		timeout:        5 * time.Second,
		maxResultBytes: fileSearchMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("limit run: %v", err)
	}
	got = decodeFileSearchResult(t, raw)
	if len(got.Hits) != 2 || !got.Truncated || got.TruncatedReason != "max_results" {
		t.Fatalf("limited result = hits %d truncated %v reason %q", len(got.Hits), got.Truncated, got.TruncatedReason)
	}
}

func TestRunFileSearchDepthAndBadRoot(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeSearchTestFile(t, root, "top.jpg", 10, now)
	writeSearchTestFile(t, root, "one/shallow.jpg", 10, now)
	writeSearchTestFile(t, root, "one/two/three/deep.jpg", 10, now)

	raw, err := runFileSearch(fileSearchOpts{
		root:           root,
		patterns:       []string{"*.jpg"},
		maxDepth:       1,
		maxResults:     100,
		timeout:        5 * time.Second,
		maxResultBytes: fileSearchMaxResultBytes,
	})
	if err != nil {
		t.Fatalf("runFileSearch: %v", err)
	}
	got := decodeFileSearchResult(t, raw)
	if len(got.Hits) != 1 || got.Hits[0].Name != "top.jpg" {
		t.Fatalf("depth-limited hits = %#v", got.Hits)
	}

	if _, err := runFileSearch(fileSearchOpts{
		root:           filepath.Join(root, "missing"),
		patterns:       []string{"*"},
		maxDepth:       8,
		maxResults:     10,
		timeout:        time.Second,
		maxResultBytes: fileSearchMaxResultBytes,
	}); err == nil || !strings.Contains(err.Error(), "file_search") {
		t.Fatalf("missing root error = %v", err)
	}
}
