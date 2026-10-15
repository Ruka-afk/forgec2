package main

import (
	"runtime"
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
// must fail before any OS window call happens, and must never report success.
//
// The wording of the error is platform-specific and used to be asserted
// unconditionally, which is why this test passed on Windows and failed on
// ubuntu CI: off Windows, closeWindow returns "window_close only on Windows"
// before it ever looks at the target, so the HWND usage text never appears.
// The invariant worth pinning everywhere is "rejected, no output"; the
// Windows wording is still checked, but only where it is produced.
func TestWindowCloseRequiresTarget(t *testing.T) {
	res := TaskResult{}
	handleWindowClose(Task{ID: 903, Type: protocol.TaskTypeWindowClose}, &res)
	if res.Error == "" {
		t.Fatal("expected an error for empty window_close target")
	}
	if res.Output != "" {
		t.Fatalf("empty target produced output %q, want none", res.Output)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(res.Error, "HWND") {
			t.Fatalf("error should mention HWND/title usage on Windows, got: %q", res.Error)
		}
	} else if !strings.Contains(res.Error, "only on Windows") {
		// Off Windows closeWindow bails before touching the target, so the
		// message is the unsupported-task one from task_window.go. Asserting it
		// keeps this branch honest instead of skipping the check entirely.
		t.Fatalf("error should say the task is Windows-only off Windows, got: %q", res.Error)
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
