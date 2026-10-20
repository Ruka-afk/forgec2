//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// file_search is the read-only metadata half of file intelligence. It never
// opens a matching file: hits carry path/name/category/size/mtime only. The
// result is a single JSON object (not the file_hunt TSV) so the media browser
// does not have to parse display formatting.
const (
	fileSearchDefaultMaxDepth   = 8
	fileSearchHardMaxDepth      = 16
	fileSearchDefaultMaxResults = 2000
	fileSearchHardMaxResults    = 5000
	fileSearchDefaultTimeout    = 20 * time.Second
	fileSearchHardTimeout       = 60 * time.Second
	// Keep the whole JSON result under the server's 1 MiB per-result cap. The
	// final marshal below still checks the real size after path escaping.
	fileSearchMaxResultBytes = 900 * 1024
)

var fileSearchCategories = map[string]bool{
	"image": true, "video": true, "audio": true, "document": true,
	"archive": true, "key": true, "config": true, "other": true,
}

// Extra directories skipped only by file_search. huntSkipDirNames already
// covers .git/node_modules/Windows and the Unix pseudo-filesystems; these are
// the remaining system trees that turn a drive-root media search into a
// multi-minute walk. Selecting one of them as the root still works (the root
// itself is never skipped).
var fileSearchExtraSkipDirNames = map[string]bool{
	"program files": true, "program files (x86)": true, "programdata": true,
	"recovery": true, "perflogs": true, "msocache": true, "config.msi": true,
	"$windows.~bt": true, "$windows.~ws": true, "run": true,
}

type fileSearchData struct {
	Patterns       []string `json:"patterns"`
	Pattern        string   `json:"pattern"`
	Categories     []string `json:"categories"`
	MinSize        int64    `json:"min_size"`
	MaxSize        int64    `json:"max_size"`
	ModifiedAfter  string   `json:"modified_after"`
	ModifiedBefore string   `json:"modified_before"`
	IncludeHidden  bool     `json:"include_hidden"`
	MaxDepth       int      `json:"max_depth"`
	MaxResults     int      `json:"max_results"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MaxResultBytes int      `json:"max_result_bytes"`
}

type fileSearchOpts struct {
	root           string
	patterns       []string
	categories     map[string]bool // empty means every category
	minSize        int64
	maxSize        int64
	modifiedAfter  time.Time
	modifiedBefore time.Time
	includeHidden  bool
	maxDepth       int
	maxResults     int
	timeout        time.Duration
	maxResultBytes int
}

type fileSearchHit struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Ext      string `json:"ext,omitempty"`
	Category string `json:"category"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`

	modTime time.Time
}

type fileSearchResult struct {
	Root            string          `json:"root"`
	Patterns        []string        `json:"patterns,omitempty"`
	Categories      []string        `json:"categories,omitempty"`
	Hits            []fileSearchHit `json:"hits"`
	Scanned         int             `json:"scanned"`
	Errors          int             `json:"errors,omitempty"`
	Truncated       bool            `json:"truncated"`
	TruncatedReason string          `json:"truncated_reason,omitempty"`
	ElapsedMS       int64           `json:"elapsed_ms"`
}

func handleFileSearch(task Task, res *TaskResult) {
	opts, err := parseFileSearchOpts(task.Path, task.Command, task.Data)
	if err != nil {
		res.Error = err.Error()
		return
	}
	out, err := runFileSearch(opts)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}

func parseFileSearchOpts(path, command, data string) (fileSearchOpts, error) {
	opts := fileSearchOpts{
		root:           strings.TrimSpace(path),
		patterns:       splitFileSearchPatterns(command),
		maxDepth:       fileSearchDefaultMaxDepth,
		maxResults:     fileSearchDefaultMaxResults,
		timeout:        fileSearchDefaultTimeout,
		maxResultBytes: fileSearchMaxResultBytes,
	}
	if opts.root == "" {
		opts.root = huntDefaultRoot()
	}

	raw := strings.TrimSpace(data)
	if strings.HasPrefix(raw, "{") {
		var d fileSearchData
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			return opts, fmt.Errorf("file_search: invalid options: %w", err)
		}
		if len(d.Patterns) > 0 {
			opts.patterns = normalizeFileSearchPatterns(d.Patterns)
		} else if strings.TrimSpace(d.Pattern) != "" {
			opts.patterns = splitFileSearchPatterns(d.Pattern)
		}
		if len(d.Categories) > 0 {
			cats, err := normalizeFileSearchCategories(d.Categories)
			if err != nil {
				return opts, err
			}
			opts.categories = cats
		}
		opts.minSize = d.MinSize
		opts.maxSize = d.MaxSize
		opts.includeHidden = d.IncludeHidden
		if d.MaxDepth > 0 {
			opts.maxDepth = d.MaxDepth
		}
		if d.MaxResults > 0 {
			opts.maxResults = d.MaxResults
		}
		if d.TimeoutSeconds > 0 {
			opts.timeout = time.Duration(d.TimeoutSeconds) * time.Second
		}
		if d.MaxResultBytes > 0 {
			opts.maxResultBytes = d.MaxResultBytes
		}
		var err error
		if opts.modifiedAfter, err = parseFileSearchTime(d.ModifiedAfter); err != nil {
			return opts, fmt.Errorf("file_search: invalid modified_after: %w", err)
		}
		if opts.modifiedBefore, err = parseFileSearchTime(d.ModifiedBefore); err != nil {
			return opts, fmt.Errorf("file_search: invalid modified_before: %w", err)
		}
	} else if raw != "" {
		// Backward-compatible key=value form for hand-issued tasks.
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(kv[0]))
			val := strings.TrimSpace(kv[1])
			switch key {
			case "patterns", "pattern":
				opts.patterns = splitFileSearchPatterns(val)
			case "categories", "category":
				cats, err := normalizeFileSearchCategories(strings.Split(val, ","))
				if err != nil {
					return opts, err
				}
				if opts.categories == nil {
					opts.categories = map[string]bool{}
				}
				for c := range cats {
					opts.categories[c] = true
				}
			case "min_size":
				opts.minSize, _ = strconv.ParseInt(val, 10, 64)
			case "max_size":
				opts.maxSize, _ = strconv.ParseInt(val, 10, 64)
			case "modified_after":
				t, err := parseFileSearchTime(val)
				if err != nil {
					return opts, fmt.Errorf("file_search: invalid modified_after: %w", err)
				}
				opts.modifiedAfter = t
			case "modified_before":
				t, err := parseFileSearchTime(val)
				if err != nil {
					return opts, fmt.Errorf("file_search: invalid modified_before: %w", err)
				}
				opts.modifiedBefore = t
			case "include_hidden":
				opts.includeHidden = val == "1" || strings.EqualFold(val, "true") || strings.EqualFold(val, "yes")
			case "max_depth":
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					opts.maxDepth = n
				}
			case "max_results":
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					opts.maxResults = n
				}
			case "timeout_seconds":
				if n, err := strconv.Atoi(val); err == nil && n > 0 {
					opts.timeout = time.Duration(n) * time.Second
				}
			}
		}
	}

	if len(opts.patterns) == 0 {
		opts.patterns = []string{"*"}
	}
	if opts.minSize < 0 {
		opts.minSize = 0
	}
	if opts.maxSize < 0 {
		opts.maxSize = 0
	}
	if opts.maxSize > 0 && opts.minSize > opts.maxSize {
		return opts, fmt.Errorf("file_search: min_size must not exceed max_size")
	}
	if opts.maxDepth > fileSearchHardMaxDepth {
		opts.maxDepth = fileSearchHardMaxDepth
	}
	if opts.maxResults > fileSearchHardMaxResults {
		opts.maxResults = fileSearchHardMaxResults
	}
	if opts.timeout > fileSearchHardTimeout {
		opts.timeout = fileSearchHardTimeout
	}
	if opts.maxResultBytes <= 0 || opts.maxResultBytes > fileSearchMaxResultBytes {
		opts.maxResultBytes = fileSearchMaxResultBytes
	}
	return opts, nil
}

func splitFileSearchPatterns(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return normalizeFileSearchPatterns(strings.Split(s, ","))
}

func normalizeFileSearchPatterns(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || len(p) > 256 {
			continue
		}
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
		if len(out) >= 32 {
			break
		}
	}
	return out
}

func normalizeFileSearchCategories(in []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, c := range in {
		c = strings.ToLower(strings.TrimSpace(c))
		if c == "" || c == "all" {
			continue
		}
		if !fileSearchCategories[c] {
			return nil, fmt.Errorf("file_search: unknown category %q", c)
		}
		out[c] = true
	}
	return out, nil
}

func parseFileSearchTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or YYYY-MM-DD")
}

func fileSearchCategoryForName(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff", "tif", "ico", "svg",
		"heic", "heif", "avif", "raw", "cr2", "nef", "arw", "dng":
		return "image"
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg",
		"ts", "m2ts", "3gp", "ogv":
		return "video"
	case "mp3", "wav", "flac", "aac", "ogg", "m4a", "wma", "opus", "aiff", "aif",
		"mid", "midi":
		return "audio"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "md", "rtf",
		"csv", "odt", "ods", "odp", "pages", "numbers":
		return "document"
	case "zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "zst", "iso", "cab",
		"jar", "war", "apk", "dmg":
		return "archive"
	case "pem", "key", "p12", "pfx", "ppk", "kdbx", "ovpn", "asc", "gpg", "pgp",
		"jks", "keystore", "der", "crt", "cer":
		return "key"
	case "ini", "conf", "config", "cfg", "yaml", "yml", "json", "xml", "toml",
		"env", "properties", "reg", "inf", "plist":
		return "config"
	default:
		return "other"
	}
}

func skipFileSearchDir(name string) bool {
	n := strings.ToLower(name)
	return huntSkipDirNames[n] || fileSearchExtraSkipDirNames[n]
}

func sortedFileSearchCategories(cats map[string]bool) []string {
	if len(cats) == 0 {
		return nil
	}
	out := make([]string, 0, len(cats))
	for c := range cats {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func runFileSearch(opts fileSearchOpts) (string, error) {
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return "", fmt.Errorf("file_search: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("file_search: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("file_search: %s is not a directory", root)
	}

	start := time.Now()
	result := fileSearchResult{
		Root:       root,
		Patterns:   append([]string{}, opts.patterns...),
		Categories: sortedFileSearchCategories(opts.categories),
		Hits:       []fileSearchHit{},
	}
	estimated := 0
	stopReason := ""

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if currentExecCtx().Err() != nil {
			stopReason = "cancelled"
			return fs.SkipAll
		}
		if time.Since(start) > opts.timeout {
			stopReason = "timeout"
			return fs.SkipAll
		}
		if walkErr != nil {
			result.Errors++
			return nil
		}
		if d == nil {
			return nil
		}
		if d.IsDir() {
			if p != root {
				if skipFileSearchDir(d.Name()) {
					return filepath.SkipDir
				}
				if !opts.includeHidden && fileSearchIsHidden(p, d.Name()) {
					return filepath.SkipDir
				}
				if huntDepth(root, p) >= opts.maxDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || !d.Type().IsRegular() {
			return nil
		}
		if !opts.includeHidden && fileSearchIsHidden(p, d.Name()) {
			return nil
		}
		if huntDepth(root, p) > opts.maxDepth {
			return nil
		}
		result.Scanned++

		category := fileSearchCategoryForName(d.Name())
		if len(opts.categories) > 0 && !opts.categories[category] {
			return nil
		}
		if !matchHuntName(d.Name(), opts.patterns) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			result.Errors++
			return nil
		}
		if fi.Size() < opts.minSize {
			return nil
		}
		if opts.maxSize > 0 && fi.Size() > opts.maxSize {
			return nil
		}
		mod := fi.ModTime()
		if !opts.modifiedAfter.IsZero() && mod.Before(opts.modifiedAfter) {
			return nil
		}
		if !opts.modifiedBefore.IsZero() && !mod.Before(opts.modifiedBefore) {
			return nil
		}
		if len(result.Hits) >= opts.maxResults {
			stopReason = "max_results"
			return fs.SkipAll
		}

		hit := fileSearchHit{
			Path:     p,
			Name:     d.Name(),
			Ext:      strings.ToLower(strings.TrimPrefix(filepath.Ext(d.Name()), ".")),
			Category: category,
			Size:     fi.Size(),
			Modified: mod.Format(time.RFC3339),
			modTime:  mod,
		}
		encoded, err := json.Marshal(hit)
		if err != nil {
			result.Errors++
			return nil
		}
		// Leave room for the envelope and for escaping invalid UTF-8 paths.
		if estimated+len(encoded)+256 > opts.maxResultBytes {
			stopReason = "max_bytes"
			return fs.SkipAll
		}
		estimated += len(encoded) + 1
		result.Hits = append(result.Hits, hit)
		return nil
	})
	if walkErr != nil && walkErr != fs.SkipAll && !errors.Is(walkErr, context.Canceled) {
		// WalkDir normally surfaces per-entry failures through the callback's
		// error argument (counted above). Only a failure of the walk itself
		// reaches here.
		result.Errors++
	}
	if stopReason != "" {
		result.Truncated = true
		result.TruncatedReason = stopReason
	}

	sort.Slice(result.Hits, func(i, j int) bool {
		if !result.Hits[i].modTime.Equal(result.Hits[j].modTime) {
			return result.Hits[i].modTime.After(result.Hits[j].modTime)
		}
		return result.Hits[i].Path < result.Hits[j].Path
	})
	result.ElapsedMS = time.Since(start).Milliseconds()

	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("file_search: encode result: %w", err)
	}
	// Path escaping can make the real document larger than the estimate. Drop
	// the oldest (tail) hits until the result fits the server's cap.
	for len(payload) > opts.maxResultBytes && len(result.Hits) > 0 {
		result.Hits = result.Hits[:len(result.Hits)-1]
		result.Truncated = true
		result.TruncatedReason = "max_bytes"
		payload, err = json.Marshal(result)
		if err != nil {
			return "", fmt.Errorf("file_search: encode result: %w", err)
		}
	}
	return string(payload), nil
}
