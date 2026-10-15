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
	"time"
)

// linuxUserHZ is the kernel's fixed /proc tick rate: CPU times in
// /proc/<pid>/stat are expressed in USER_HZ (100 on every architecture),
// regardless of the architecture's internal clock tick.
const linuxUserHZ = 100

// cpuTime returns the CPU time consumed by every live process in the plugin's
// process group.
func (g *processGuard) cpuTime() (time.Duration, error) {
	if g == nil || g.pgid <= 0 {
		return 0, errGuardGone
	}
	return groupCPUTime("/proc", g.pgid)
}

// groupCPUTime sums utime+stime over every process whose process-group id is
// pgid under a procfs root. Processes that exit mid-scan (or whose stat file
// turns unreadable) are skipped; when no live member remains it reports zero,
// because a dead tree has nothing left to bound.
func groupCPUTime(procRoot string, pgid int) (time.Duration, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, err
	}
	var ticks int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "stat"))
		if err != nil {
			continue
		}
		entryPGID, utime, stime, err := parseProcStat(data)
		if err != nil || entryPGID != pgid {
			continue
		}
		ticks += utime + stime
	}
	return time.Duration(ticks) * (time.Second / linuxUserHZ), nil
}

// parseProcStat extracts the process-group id and the utime/stime tick counts
// from one /proc/<pid>/stat line. The comm field is the only field that may
// contain spaces or parentheses, so parsing resumes after its last ')'.
func parseProcStat(data []byte) (pgid int, utime, stime int64, err error) {
	open := bytes.IndexByte(data, '(')
	if open < 0 {
		return 0, 0, 0, errors.New("malformed stat: no comm field")
	}
	rest := data[open:]
	closing := bytes.LastIndexByte(rest, ')')
	if closing < 0 || closing+1 >= len(rest) {
		return 0, 0, 0, errors.New("malformed stat: unterminated comm field")
	}
	fields := strings.Fields(string(rest[closing+1:]))
	// fields[0]=state, [1]=ppid, [2]=pgrp … [11]=utime, [12]=stime.
	if len(fields) < 13 {
		return 0, 0, 0, errors.New("malformed stat: truncated")
	}
	p, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, 0, err
	}
	u, err := strconv.ParseInt(fields[11], 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	s, err := strconv.ParseInt(fields[12], 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return p, u, s, nil
}
