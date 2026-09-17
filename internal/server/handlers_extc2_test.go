package server

import (
	"testing"
)

func newExtC2TestServer() *Server {
	return &Server{
		extC2TaskQueue: make(map[string][]extC2Task),
		extC2Notify:    make(map[string]chan struct{}),
	}
}

// TestExtC2RequeuePrepends proves a failed push redelivers in order: the
// unsent tasks go back to the head, ahead of anything queued meanwhile.
func TestExtC2RequeuePrepends(t *testing.T) {
	s := newExtC2TestServer()
	const ch = "ch-1"
	s.extC2Notify[ch] = make(chan struct{}, 1)

	s.QueueExtC2Task(ch, extC2Task{AgentID: "a", Type: "send", Command: "one"})
	s.QueueExtC2Task(ch, extC2Task{AgentID: "a", Type: "send", Command: "two"})

	// Sender drains the queue, first send succeeds, second fails.
	s.extC2TaskMu.Lock()
	drained := s.extC2TaskQueue[ch]
	s.extC2TaskQueue[ch] = nil
	s.extC2TaskMu.Unlock()
	if len(drained) != 2 {
		t.Fatalf("drained %d, want 2", len(drained))
	}
	s.requeueExtC2Tasks(ch, drained[1:])

	s.extC2TaskMu.Lock()
	queue := s.extC2TaskQueue[ch]
	s.extC2TaskMu.Unlock()
	if len(queue) != 1 || queue[0].Command != "two" {
		t.Fatalf("requeued %+v, want [two]", queue)
	}
	// A task queued meanwhile lands behind the redelivery.
	s.QueueExtC2Task(ch, extC2Task{AgentID: "a", Type: "send", Command: "three"})
	s.extC2TaskMu.Lock()
	queue = s.extC2TaskQueue[ch]
	s.extC2TaskMu.Unlock()
	if len(queue) != 2 || queue[0].Command != "two" || queue[1].Command != "three" {
		t.Fatalf("order broken: %+v", queue)
	}
}

// TestExtC2RequeueBounded proves a requeue storm can't grow the queue past
// the per-channel cap (oldest shed, like QueueExtC2Task).
func TestExtC2RequeueBounded(t *testing.T) {
	s := newExtC2TestServer()
	const ch = "ch-2"
	s.extC2Notify[ch] = make(chan struct{}, 1)

	big := make([]extC2Task, 0, MaxExtC2QueuePerChan+10)
	for i := 0; i < MaxExtC2QueuePerChan+10; i++ {
		big = append(big, extC2Task{AgentID: "a", Type: "send", Command: "x"})
	}
	s.requeueExtC2Tasks(ch, big)

	s.extC2TaskMu.Lock()
	n := len(s.extC2TaskQueue[ch])
	s.extC2TaskMu.Unlock()
	if n != MaxExtC2QueuePerChan {
		t.Fatalf("queue size %d, want cap %d", n, MaxExtC2QueuePerChan)
	}
}

// TestExtC2RequeueEmpty is a no-op and must not panic.
func TestExtC2RequeueEmpty(t *testing.T) {
	s := newExtC2TestServer()
	s.requeueExtC2Tasks("ghost", nil)
	if _, ok := s.extC2TaskQueue["ghost"]; ok {
		t.Fatal("empty requeue must not create a queue entry")
	}
}
