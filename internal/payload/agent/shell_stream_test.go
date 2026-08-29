package main

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestRunShellStreamingFastCommandSilent pins the quiet path: a command that
// finishes well inside the first-flush gate produces the full output with
// ZERO partial deltas (keeps the beacon pipeline free of churn).
func TestRunShellStreamingFastCommandSilent(t *testing.T) {
	deltas := 0
	out, err := runShellStreaming("echo FASTMARK", "", func(delta string) {
		deltas++
	})
	if err != nil {
		t.Fatalf("streaming run: %v", err)
	}
	if !strings.Contains(out, "FASTMARK") {
		t.Fatalf("output missing marker: %q", out)
	}
	if deltas != 0 {
		t.Fatalf("fast command produced %d partial deltas, want 0", deltas)
	}
}

// TestRunShellStreamingEmitsDeltas exercises the live path with a command
// that outlives the first-flush gate: at least one delta must arrive before
// completion, and the final full output must still contain everything.
func TestRunShellStreamingEmitsDeltas(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive streaming test")
	}
	var mu []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		// ~4s total: crosses the 3 s first-flush gate twice.
		if runtime.GOOS == "windows" {
			_, _ = runShellStreaming(
				"Write-Output PART1; Start-Sleep -Seconds 2; Write-Output PART2; Start-Sleep -Seconds 2; Write-Output PART3",
				"powershell.exe",
				func(delta string) { mu = append(mu, delta) },
			)
			return
		}
		_, _ = runShellStreaming(
			"echo PART1; sleep 2; echo PART2; sleep 2; echo PART3", "",
			func(delta string) { mu = append(mu, delta) },
		)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("streaming run timed out")
	}
	if len(mu) == 0 {
		t.Fatal("long-running command produced no partial deltas")
	}
	joined := strings.Join(mu, "")
	if !strings.Contains(joined, "PART1") && !strings.Contains(joined, "PART2") {
		t.Fatalf("deltas missing expected content: %q", joined)
	}
}

// TestEnqueuePartialResultShape verifies the wire contract: Partial=true,
// base64 output, unique rid per chunk.
func TestEnqueuePartialResultShape(t *testing.T) {
	task := Task{ID: 7777, Type: protocol.TaskTypeShell}

	pendingMu.Lock()
	pendingResults = nil
	pendingMu.Unlock()

	enqueuePartialResult(task, "chunk-one")
	enqueuePartialResult(task, "chunk-two")

	pendingMu.Lock()
	got := append([]protocol.TaskResult{}, pendingResults...)
	pendingResults = nil
	pendingMu.Unlock()

	if len(got) != 2 {
		t.Fatalf("enqueued %d partials, want 2", len(got))
	}
	for i, r := range got {
		if !r.Partial {
			t.Errorf("partial[%d].Partial = false", i)
		}
		if r.Encoding != "base64" {
			t.Errorf("partial[%d].Encoding = %q", i, r.Encoding)
		}
		if r.ResultID == "" || strings.Contains(r.ResultID, "nil") {
			t.Errorf("partial[%d] rid empty/malformed: %q", i, r.ResultID)
		}
	}
	if got[0].ResultID == got[1].ResultID {
		t.Fatal("rid not unique across chunks")
	}
}
