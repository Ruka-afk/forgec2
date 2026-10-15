//go:build linux
// +build linux

package plugin

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// writeBytes returns the bytes every live process in the plugin's process
// group has written. The kernel counts bytes passed to write(2) (field wchar
// in /proc/<pid>/io) regardless of destination, which is exactly the file /
// pipe / console scope the budget documents.
func (g *processGuard) writeBytes() (int64, error) {
	if g == nil || g.pgid <= 0 {
		return 0, errGuardGone
	}
	return groupWriteBytes("/proc", g.pgid)
}

// groupWriteBytes sums wchar over every process whose process-group id is pgid
// under a procfs root. Processes that exit mid-scan are skipped: a dead writer
// has nothing left to bound. /proc/<pid>/io is refused for processes the
// caller may not inspect (strictly more restrictive than /proc/<pid>/stat), so
// such members are skipped too — like the CPU walk, this is best effort.
func groupWriteBytes(procRoot string, pgid int) (int64, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, err
	}
	var written int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		pidDir := filepath.Join(procRoot, e.Name())
		stat, err := os.ReadFile(filepath.Join(pidDir, "stat"))
		if err != nil {
			continue
		}
		entryPGID, _, _, err := parseProcStat(stat)
		if err != nil || entryPGID != pgid {
			continue
		}
		ioDump, err := os.ReadFile(filepath.Join(pidDir, "io"))
		if err != nil {
			continue
		}
		n, err := parseProcIOWChar(ioDump)
		if err != nil {
			continue
		}
		written += n
	}
	return written, nil
}

// parseProcIOWChar extracts the wchar line ("wchar: 12345") from one
// /proc/<pid>/io dump: the number of bytes the process caused to be written.
func parseProcIOWChar(data []byte) (int64, error) {
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		rest, ok := bytes.CutPrefix(line, []byte("wchar:"))
		if !ok {
			continue
		}
		return strconv.ParseInt(strings.TrimSpace(string(rest)), 10, 64)
	}
	return 0, errors.New("malformed /proc/<pid>/io: no wchar line")
}
