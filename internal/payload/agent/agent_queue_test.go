//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "testing"

// TestEnqueueTaskDedupesRedelivery proves server-side lost-response recovery
// (re-carried unacked tasks) never double-executes: the second delivery of an
// accepted task ID is acked again but not queued.
func TestEnqueueTaskDedupesRedelivery(t *testing.T) {
	pendingMu.Lock()
	seenTaskIDs = make(map[uint]struct{})
	seenTaskIDOrder = nil
	pendingTaskAcks = nil
	pendingMu.Unlock()
	drainTaskQueue()
	defer func() {
		pendingMu.Lock()
		seenTaskIDs = make(map[uint]struct{})
		seenTaskIDOrder = nil
		pendingTaskAcks = nil
		pendingMu.Unlock()
		drainTaskQueue()
	}()

	enqueueTask(Task{ID: 90001, Type: "shell", Command: "whoami"})
	enqueueTask(Task{ID: 90001, Type: "shell", Command: "whoami"})
	enqueueTask(Task{ID: 90002, Type: "shell", Command: "hostname"})

	queued := make(map[uint]int)
drain:
	for {
		select {
		case task := <-taskQueue:
			queued[task.ID]++
		default:
			break drain
		}
	}
	if queued[90001] != 1 {
		t.Fatalf("task 90001 queued %d times, want exactly 1", queued[90001])
	}
	if queued[90002] != 1 {
		t.Fatalf("task 90002 queued %d times, want exactly 1", queued[90002])
	}

	pendingMu.Lock()
	acks90001 := 0
	for _, id := range pendingTaskAcks {
		if id == 90001 {
			acks90001++
		}
	}
	pendingMu.Unlock()
	if acks90001 != 2 {
		t.Fatalf("task 90001 acked %d times, want 2 (accept + redelivery)", acks90001)
	}
}
