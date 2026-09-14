package main

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestWindowHandlersRegistered pins the gh0st C_SYSTEM parity wiring:
// both window task types must resolve to handlers on every platform.
func TestWindowHandlersRegistered(t *testing.T) {
	for _, typ := range []string{protocol.TaskTypeWindowList, protocol.TaskTypeWindowClose} {
		if taskHandlers[typ] == nil {
			t.Errorf("task %q has no registered handler", typ)
		}
	}
}

// TestWindowCloseRequiresTarget is platform-independent: an empty target
// must fail before any OS window call happens.
func TestWindowCloseRequiresTarget(t *testing.T) {
	res := TaskResult{}
	handleWindowClose(Task{ID: 903, Type: protocol.TaskTypeWindowClose}, &res)
	if res.Error == "" {
		t.Fatal("expected an error for empty window_close target")
	}
	if !strings.Contains(res.Error, "HWND") {
		t.Fatalf("error should mention HWND/title usage, got: %q", res.Error)
	}
}

// TestWindowCloseUnknownTarget must fail closed without side effects.
func TestWindowCloseUnknownTarget(t *testing.T) {
	res := TaskResult{}
	handleWindowClose(Task{ID: 904, Type: protocol.TaskTypeWindowClose, Command: "no-such-window-zz-404"}, &res)
	if res.Error == "" {
		t.Fatal("expected an error for an unmatched window target")
	}
	if res.Output != "" {
		t.Fatalf("failed close must not produce output, got: %q", res.Output)
	}
}
