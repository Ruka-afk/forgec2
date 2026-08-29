package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Filesystem mutation handlers: mkdir / rename / chmod.
//
// These complement the existing ls/read/delete/download/upload set so common
// file operations no longer require shelling out. All paths are used exactly
// as supplied by the operator (the server-side task registry documents the
// parameters); errors are reported verbatim in res.Error.

// handleMkdir creates a directory, creating parent components as needed
// (os.MkdirAll semantics). command = directory path.
func handleMkdir(task Task, res *TaskResult) {
	dir := strings.TrimSpace(task.Command)
	if dir == "" {
		res.Error = "mkdir: directory path required"
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		res.Error = fmt.Sprintf("mkdir %s: %v", dir, err)
		return
	}
	res.Output = fmt.Sprintf("created directory %s", dir)
}

// handleRename renames or moves a file/directory. command = current path,
// data = new path. Rename across directories moves the entry; across
// filesystem boundaries Go returns EXDEV which is surfaced as an error.
func handleRename(task Task, res *TaskResult) {
	oldPath := strings.TrimSpace(task.Command)
	newPath := strings.TrimSpace(task.Data)
	if oldPath == "" || newPath == "" {
		res.Error = "rename: both current path (command) and new path (data) are required"
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		res.Error = fmt.Sprintf("rename %s -> %s: %v", oldPath, newPath, err)
		return
	}
	res.Output = fmt.Sprintf("renamed %s to %s", oldPath, newPath)
}

// handleChmod changes file mode bits. command = path, data = octal mode.
// On POSIX the full mode applies; on Windows Go maps it to the read-only bit
// only — the task-type description states this so operators are not misled.
func handleChmod(task Task, res *TaskResult) {
	path := strings.TrimSpace(task.Command)
	modeStr := strings.TrimSpace(task.Data)
	if path == "" || modeStr == "" {
		res.Error = "chmod: path (command) and octal mode (data) are required"
		return
	}
	mode, err := strconv.ParseUint(strings.TrimPrefix(modeStr, "0o"), 8, 32)
	if err != nil {
		res.Error = fmt.Sprintf("chmod: invalid octal mode %q", modeStr)
		return
	}
	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		res.Error = fmt.Sprintf("chmod %s: %v", path, err)
		return
	}
	res.Output = fmt.Sprintf("set mode %s on %s", modeStr, path)
}
