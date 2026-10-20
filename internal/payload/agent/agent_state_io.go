package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// stateTempPattern is the prefix shared by in-progress state writes. The spool
// has its own (.results-*.tmp) because it can be much larger, but both are
// swept at startup: a crash mid-write must not leave the identity private key
// or task output in a stray temp file.
const stateTempPattern = ".state-*.tmp"

// writeStateFileAtomic replaces path with data via a temp file in the same
// directory and an atomic rename. A crash or power loss can leave the previous
// value or the new one, never a truncated file — the failure mode of writing
// the destination in place. sync flushes file data before the rename, which is
// required for values whose regression is protocol-visible (beacon.seq, the
// identity key, the persisted network config).
func writeStateFileAtomic(path string, data []byte, mode os.FileMode, sync bool) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), stateTempPattern)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if runtime.GOOS != "windows" {
		if err := tmp.Chmod(mode); err != nil {
			return fail(err)
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if sync {
		if err := tmp.Sync(); err != nil {
			return fail(err)
		}
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	// Make the rename itself durable on Unix. Windows cannot fsync a directory
	// handle; MoveFileEx is already ordered by the filesystem.
	if sync && runtime.GOOS != "windows" {
		if d, err := os.Open(filepath.Dir(path)); err == nil {
			_ = d.Sync()
			_ = d.Close()
		}
	}
	if runtime.GOOS == "windows" {
		setHidden(path)
	}
	return nil
}

// cleanupStateTempFiles removes half-written state files left by a crash.
// Callers run it before reading persisted state; the files are disposable.
func cleanupStateTempFiles(dir string) {
	patterns := []string{
		filepath.Join(dir, stateTempPattern),
		filepath.Join(dir, ".results-*.tmp"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range matches {
			_ = os.Remove(path)
		}
	}
}
