package main

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// setupResultsSpoolTest isolates the queue globals and redirects the spool to a
// temp file. It deliberately leaves pendingResultsSpoolPath pointing at the
// temp directory after cleanup: if a debounced flush ever fired late, it must
// not write test data into the real agent state directory.
func setupResultsSpoolTest(t *testing.T) string {
	t.Helper()
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate identity key: %v", err)
	}

	pendingMu.Lock()
	oldResults, oldInflight, oldAcks := pendingResults, inflightResults, pendingTaskAcks
	pendingResults, inflightResults, pendingTaskAcks = nil, nil, nil
	pendingMu.Unlock()

	oldPriv, oldUUID := identityPriv, agentUUID
	identityPriv, agentUUID = priv, "spool-test-agent"

	path := filepath.Join(t.TempDir(), "results.spool")
	pendingResultsSpoolPath = func() string { return path }

	t.Cleanup(func() {
		pendingMu.Lock()
		pendingResults, inflightResults, pendingTaskAcks = oldResults, oldInflight, oldAcks
		pendingMu.Unlock()
		identityPriv, agentUUID = oldPriv, oldUUID
	})
	return path
}

func TestResultsSpoolEncryptedRoundTrip(t *testing.T) {
	path := setupResultsSpoolTest(t)
	in := []TaskResult{
		{TaskID: 7, Type: "shell", Output: "TOPSECRET-shell-output", ResultID: "rid-7"},
		{TaskID: 8, Type: "download", Output: "ZmlsZQ==", Encoding: "base64", Filename: "a.bin", Size: 4, Offset: 2, ResultID: "rid-8"},
		{TaskID: 9, Type: "stream", Output: "partial", Partial: true, ResultID: "rid-9"},
	}
	if err := writeResultsSpool(path, in); err != nil {
		t.Fatalf("writeResultsSpool: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw spool: %v", err)
	}
	if bytes.Contains(raw, []byte("TOPSECRET-shell-output")) {
		t.Fatal("spool contains plaintext task output")
	}
	if !bytes.HasPrefix(raw, []byte(resultsSpoolMagic)) {
		t.Fatalf("spool missing magic prefix: %q", raw[:min(len(raw), len(resultsSpoolMagic))])
	}

	got, err := readResultsSpool(path)
	if err != nil {
		t.Fatalf("readResultsSpool: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, in)
	}
}

func TestResultsSpoolRejectsTamperingAndOtherIdentity(t *testing.T) {
	path := setupResultsSpoolTest(t)
	in := []TaskResult{{TaskID: 1, Type: "shell", Output: "out", ResultID: "rid-1"}}
	if err := writeResultsSpool(path, in); err != nil {
		t.Fatalf("writeResultsSpool: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw spool: %v", err)
	}
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("tamper spool: %v", err)
	}
	if _, err := readResultsSpool(path); err == nil {
		t.Fatal("tampered spool decrypted successfully")
	}

	if err := writeResultsSpool(path, in); err != nil {
		t.Fatalf("rewrite spool: %v", err)
	}
	agentUUID = "a-different-agent"
	if _, err := readResultsSpool(path); err == nil {
		t.Fatal("spool decrypted under a different agent UUID")
	}
}

func TestResultsSpoolEmptySnapshotRemovesFile(t *testing.T) {
	path := setupResultsSpoolTest(t)
	if err := writeResultsSpool(path, []TaskResult{{TaskID: 1, Type: "shell", Output: "x"}}); err != nil {
		t.Fatalf("writeResultsSpool: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("spool not written: %v", err)
	}
	if err := writeResultsSpool(path, nil); err != nil {
		t.Fatalf("write empty spool: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("empty snapshot left a spool file: %v", err)
	}
}

func TestLoadPendingResultsSpoolRestoresQueue(t *testing.T) {
	path := setupResultsSpoolTest(t)
	stored := []TaskResult{
		{TaskID: 11, Type: "shell", Output: "first", ResultID: "rid-11"},
		{TaskID: 12, Type: "shell", Output: "second", ResultID: "rid-12"},
	}
	if err := writeResultsSpool(path, stored); err != nil {
		t.Fatalf("writeResultsSpool: %v", err)
	}
	loadPendingResultsSpool()

	pendingMu.Lock()
	got := append([]TaskResult(nil), pendingResults...)
	pendingMu.Unlock()
	if !reflect.DeepEqual(got, stored) {
		t.Fatalf("restored queue mismatch:\n got %+v\nwant %+v", got, stored)
	}
}

func TestLoadPendingResultsSpoolBackfillsResultID(t *testing.T) {
	path := setupResultsSpoolTest(t)
	stored := []TaskResult{{TaskID: 13, Type: "shell", Output: "legacy"}}
	if err := writeResultsSpool(path, stored); err != nil {
		t.Fatalf("writeResultsSpool: %v", err)
	}
	loadPendingResultsSpool()

	pendingMu.Lock()
	got := append([]TaskResult(nil), pendingResults...)
	pendingMu.Unlock()
	if len(got) != 1 {
		t.Fatalf("restored %d results, want 1", len(got))
	}
	if got[0].ResultID == "" {
		t.Fatal("restored result without a ResultID was not backfilled")
	}
	if got[0].TaskID != 13 || got[0].Output != "legacy" {
		t.Fatalf("backfill changed the result: %+v", got[0])
	}
}

func TestLoadPendingResultsSpoolDiscardsCorruptFile(t *testing.T) {
	path := setupResultsSpoolTest(t)
	if err := os.WriteFile(path, []byte(resultsSpoolMagic+"not-really-ciphertext"), 0o600); err != nil {
		t.Fatalf("write corrupt spool: %v", err)
	}
	loadPendingResultsSpool()

	pendingMu.Lock()
	n := len(pendingResults)
	pendingMu.Unlock()
	if n != 0 {
		t.Fatalf("corrupt spool restored %d results", n)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt spool was not removed: %v", err)
	}
}

func TestFlushPendingResultsSpoolIncludesInflight(t *testing.T) {
	path := setupResultsSpoolTest(t)
	inflight := TaskResult{TaskID: 21, Type: "shell", Output: "on-the-wire", ResultID: "rid-21"}
	pending := TaskResult{TaskID: 22, Type: "shell", Output: "queued", ResultID: "rid-22"}

	pendingMu.Lock()
	inflightResults = []TaskResult{inflight}
	pendingResults = []TaskResult{pending}
	pendingMu.Unlock()

	flushPendingResultsSpool()

	got, err := readResultsSpool(path)
	if err != nil {
		t.Fatalf("readResultsSpool: %v", err)
	}
	want := []TaskResult{inflight, pending}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestRequeuePendingBeaconClearsInflight(t *testing.T) {
	path := setupResultsSpoolTest(t)
	first := TaskResult{TaskID: 31, Type: "shell", Output: "first", ResultID: "rid-31"}
	later := TaskResult{TaskID: 32, Type: "shell", Output: "later", ResultID: "rid-32"}

	pendingMu.Lock()
	inflightResults = []TaskResult{first}
	pendingResults = []TaskResult{later}
	pendingMu.Unlock()

	requeuePendingBeacon([]TaskResult{first}, []uint{7})

	pendingMu.Lock()
	got := append([]TaskResult(nil), pendingResults...)
	inflight := inflightResults
	acks := append([]uint(nil), pendingTaskAcks...)
	pendingMu.Unlock()

	if !reflect.DeepEqual(got, []TaskResult{first, later}) {
		t.Fatalf("requeued queue mismatch: %+v", got)
	}
	if inflight != nil {
		t.Fatalf("inflight results not cleared: %+v", inflight)
	}
	if !reflect.DeepEqual(acks, []uint{7}) {
		t.Fatalf("acks mismatch: %v", acks)
	}

	flushPendingResultsSpool()
	stored, err := readResultsSpool(path)
	if err != nil {
		t.Fatalf("readResultsSpool: %v", err)
	}
	if !reflect.DeepEqual(stored, []TaskResult{first, later}) {
		t.Fatalf("spool mismatch after requeue: %+v", stored)
	}
}

func TestReenforcePendingBoundsDropsOldest(t *testing.T) {
	setupResultsSpoolTest(t)
	pendingMu.Lock()
	defer pendingMu.Unlock()
	pendingResults = make([]TaskResult, maxPendingResults+3)
	for i := range pendingResults {
		pendingResults[i] = TaskResult{TaskID: uint(i), Type: "shell", Output: "x", ResultID: "rid"}
	}
	dropped := reenforcePendingBounds()
	if dropped != 3 {
		t.Fatalf("dropped = %d, want 3", dropped)
	}
	if len(pendingResults) != maxPendingResults {
		t.Fatalf("queue length = %d, want %d", len(pendingResults), maxPendingResults)
	}
	if pendingResults[0].TaskID != 3 {
		t.Fatalf("oldest results were not dropped: first TaskID = %d", pendingResults[0].TaskID)
	}
}
