//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"context"
	"testing"
)

// TestCurrentExecCtxBackgroundFallback verifies that goroutines outside the
// task worker scope observe context.Background() rather than a stale or nil
// context, so fire-and-forget commands are never bound to beacon teardown.
func TestCurrentExecCtxBackgroundFallback(t *testing.T) {
	done := make(chan bool, 1)
	go func() {
		ctx := currentExecCtx()
		if ctx == nil {
			t.Error("currentExecCtx() returned nil on unbound goroutine")
		}
		if ctx != context.Background() {
			t.Error("currentExecCtx() did not fall back to context.Background()")
		}
		done <- true
	}()
	if !<-done {
		t.Fatal("goroutine exited early")
	}
}

// TestCurrentExecCtxGoroutineLocal verifies the per-goroutine scoping: the ctx
// set on one goroutine is visible only to that goroutine, is cleared by
// setExecCtx(nil), and another goroutine keeps seeing Background.
func TestCurrentExecCtxGoroutineLocal(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ready := make(chan struct{})
	release := make(chan struct{})
	ownerResult := make(chan bool, 1)

	go func() {
		setExecCtx(ctx1)
		defer setExecCtx(nil)
		close(ready)
		<-release
		ownerResult <- currentExecCtx() == ctx1
	}()
	<-ready

	other := make(chan context.Context, 1)
	for i := 0; i < 3; i++ {
		go func() { other <- currentExecCtx() }()
	}
	for i := 0; i < 3; i++ {
		got := <-other
		if got == ctx1 {
			t.Fatalf("goroutine-local ctx leaked across goroutines (round %d)", i)
		}
		if got == nil || got != context.Background() {
			t.Fatalf("unbound goroutine saw unexpected ctx (round %d)", i)
		}
	}

	// The owner goroutine must still resolve its own ctx while bound.
	close(release)
	if !<-ownerResult {
		t.Error("owner goroutine lost its own ctx")
	}

	// Clearing the binding restores Background on the same goroutine.
	setExecCtx(ctx1)
	setExecCtx(nil)
	if got := currentExecCtx(); got != context.Background() {
		t.Error("setExecCtx(nil) did not restore Background")
	}

	// A cancelled owner ctx is delivered as-is so abort signalling works via
	// Done() on the scoped ctx.
	ctx2, cancel2 := context.WithCancel(context.Background())
	scoped := make(chan context.Context, 1)
	ready2 := make(chan struct{})
	go func() {
		setExecCtx(ctx2)
		defer setExecCtx(nil)
		close(ready2)
		<-ctx2.Done()
		scoped <- currentExecCtx()
	}()
	<-ready2
	cancel2()
	if got := <-scoped; got != ctx2 {
		t.Error("cancelled owner ctx not visible after cancellation")
	}
}

// TestCurrentGoroutineID verifies goroutine ids are distinct and parseable.
func TestCurrentGoroutineID(t *testing.T) {
	id := currentGoroutineID()
	if id == 0 {
		t.Fatal("currentGoroutineID() returned 0")
	}
	other := make(chan uint64, 1)
	go func() { other <- currentGoroutineID() }()
	otherID := <-other
	if otherID == id {
		t.Fatalf("two goroutines shared id %d", id)
	}
	if otherID == 0 {
		t.Fatal("parallel goroutine id was 0")
	}
}

// TestCancelTaskExecutionStateMachine pins the abort state machine: marking a
// task aborted is sticky, cancels only a registered running execution, and
// never panics for unknown ids.
func TestCancelTaskExecutionStateMachine(t *testing.T) {
	if isTaskAborted(99991) {
		t.Fatal("fresh task id reported aborted")
	}

	// No registered execution: flag is set, nothing cancelled.
	cancelTaskExecution(99991)
	if !isTaskAborted(99991) {
		t.Fatal("cancelTaskExecution did not mark the id aborted")
	}

	// A running execution with a registered cancel func gets cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	registerExecCancel(99992, cancel)
	cancelTaskExecution(99992)
	if ctx.Err() == nil {
		t.Error("registered execution was not cancelled")
	}
	unregisterExecCancel(99992)

	// Sticky: an aborted task stays aborted even after unregistering.
	if !isTaskAborted(99992) {
		t.Error("aborted flag was lost after unregistering execution")
	}

	// Unknown ids are safe no-ops.
	cancelTaskExecution(99993)
	if !isTaskAborted(99993) {
		t.Error("cancelTaskExecution(unknown) did not mark id")
	}
}

// TestRunTaskSkippedWhenAbortedBeforeStart verifies a task cancelled while
// still queued is skipped with an operator-facing error instead of executing.
func TestRunTaskSkippedWhenAbortedBeforeStart(t *testing.T) {
	cancelTaskExecution(99994)
	defer func() {
		abortMu.Lock()
		delete(abortedTasks, 99994)
		abortMu.Unlock()
	}()

	res := runTask(Task{ID: 99994, Type: "shell", Command: "echo hi"})
	if res.Error == "" {
		t.Fatal("aborted-before-start task returned no error")
	}
	if res.Type != "shell" || res.TaskID != 99994 {
		t.Errorf("result envelope wrong: %+v", res)
	}
}

// TestInsertTaskEvictsOldest verifies the bounded queue prefers the newest
// task: when the queue is full the oldest entry is evicted and returned as an
// error result, and the new task is accepted.
func TestInsertTaskEvictsOldest(t *testing.T) {
	// Empty any residue from other tests.
	drainTaskQueue()
	defer drainTaskQueue()

	for i := 1; i <= cap(taskQueue); i++ {
		if !insertTask(Task{ID: uint(i), Type: "fill"}) {
			t.Fatalf("insertTask(%d) failed while filling", i)
		}
	}
	if len(taskQueue) != cap(taskQueue) {
		t.Fatalf("queue not full: %d/%d", len(taskQueue), cap(taskQueue))
	}

	if !insertTask(Task{ID: 99999, Type: "new"}) {
		t.Fatal("insertTask rejected a task on a full queue")
	}

	// Drain and verify the newest made it and the oldest (id 1) did not.
	seen := make(map[uint]bool)
drain:
	for {
		select {
		case task := <-taskQueue:
			seen[task.ID] = true
		default:
			break drain
		}
	}
	if len(seen) != cap(taskQueue) {
		t.Errorf("queue should hold exactly %d tasks after eviction, got %d", cap(taskQueue), len(seen))
	}
	if seen[99999] != true {
		t.Error("newest task missing from queue after insert")
	}
	if seen[1] {
		t.Error("evicted task id 1 is still in the queue")
	}

	// The eviction surfaces as a terminal error result for the operator.
	pendingMu.Lock()
	evictedResult := false
	for _, r := range pendingResults {
		if r.TaskID == 1 && r.Error != "" {
			evictedResult = true
		}
	}
	ackedEvicted := false
	for _, id := range pendingTaskAcks {
		if id == 1 {
			ackedEvicted = true
		}
	}
	pendingResults = nil
	pendingTaskAcks = nil
	pendingMu.Unlock()
	if !evictedResult {
		t.Error("evicted task did not produce an error result")
	}
	if !ackedEvicted {
		t.Error("evicted task was not acknowledged")
	}
}

// drainTaskQueue empties the global task queue. No workers run in unit tests,
// so any residue would leak between tests.
func drainTaskQueue() {
	for {
		select {
		case <-taskQueue:
		default:
			return
		}
	}
}

// TestIsCancelRelatedError pins which error strings map to an abort/timeout
// interruption rather than a real command failure.
func TestIsCancelRelatedError(t *testing.T) {
	cases := map[string]bool{
		"exit status 1":              false,
		"command not found":          false,
		"signal: killed":             true,
		"signal: terminated":         true,
		"context canceled":           true,
		"context deadline exceeded":  true,
		"task cancelled by operator": true,
		"permission denied":          false,
	}
	for err, want := range cases {
		if got := isCancelRelatedError(err); got != want {
			t.Errorf("isCancelRelatedError(%q) = %v, want %v", err, got, want)
		}
	}
}
