package server

import (
	"errors"
	"strings"
	"testing"
)

// TestCreateTaskRequestErrors pins the contract the operator API relies on:
// failures caused by the request itself (missing mandatory parameter, unknown
// type, oversized command) carry *taskRequestError so apiCreateTask answers
// 400 with the reason instead of an opaque 500 "task creation failed".
//
// Regression: POST /api/v1/tasks {"type":"read","path":"..."} returned 500
// because read's spec parameter is 'command' (the file path) - the E2E gate
// hit this while dispatching a read task through the documented API.
func TestCreateTaskRequestErrors(t *testing.T) {
	s := newTasksTestServer(t)
	seedAgent(t, s, "agent-req")

	// read without 'command' must be a request error naming the parameter.
	_, err := s.createTask("agent-req", "read", "", "", "/tmp/x", "", 0, 0)
	if err == nil {
		t.Fatal("read without command must be rejected")
	}
	var reqErr *taskRequestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("missing-parameter error %v (%T) is not a *taskRequestError", err, err)
	}
	if !strings.Contains(err.Error(), "'command' parameter") {
		t.Fatalf("error %q does not name the missing parameter", err.Error())
	}

	// With 'command' the same task is accepted (path stays an optional hint).
	if _, err := s.createTask("agent-req", "read", "/tmp/x", "", "", "", 0, 0); err != nil {
		t.Fatalf("read with command must be accepted: %v", err)
	}

	// Unknown types are request errors, not server faults.
	if _, err := s.createTask("agent-req", "definitely_not_a_task", "", "", "", "", 0, 0); err == nil {
		t.Fatal("unknown task type must be rejected")
	} else if !errors.As(err, &reqErr) {
		t.Fatalf("unknown-type error %v is not a *taskRequestError", err)
	}

	// Oversized commands are request errors too.
	if _, err := s.createTask("agent-req", "shell", strings.Repeat("a", MaxCommandLength+1), "", "", "", 0, 0); err == nil {
		t.Fatal("oversized command must be rejected")
	} else if !errors.As(err, &reqErr) {
		t.Fatalf("oversized-command error %v is not a *taskRequestError", err)
	}
}
