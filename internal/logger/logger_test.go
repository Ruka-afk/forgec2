package logger

import (
	"errors"
	"strings"
	"testing"
)

func TestAppError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewAppError("AUTH_FAIL", "authentication failed", nil)
		msg := err.Error()
		if !strings.Contains(msg, "AUTH_FAIL") || !strings.Contains(msg, "authentication failed") {
			t.Fatalf("unexpected error message: %s", msg)
		}
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("invalid password")
		err := NewAppError("AUTH_FAIL", "authentication failed", cause)
		msg := err.Error()
		if !strings.Contains(msg, "invalid password") {
			t.Fatalf("error message should contain cause: %s", msg)
		}
		if err.Unwrap() != cause {
			t.Fatal("Unwrap() should return the underlying cause")
		}
	})

	t.Run("with context", func(t *testing.T) {
		err := NewAppError("DB_ERR", "db error", nil).
			WithContext("table", "users").
			WithContext("query", "SELECT *")
		if err.Context["table"] != "users" {
			t.Fatalf("expected context[table]=users, got %v", err.Context["table"])
		}
		if err.Context["query"] != "SELECT *" {
			t.Fatalf("expected context[query]=SELECT *, got %v", err.Context["query"])
		}
	})

	t.Run("with status code", func(t *testing.T) {
		err := NewAppError("NOT_FOUND", "not found", nil).WithStatusCode(404)
		if err.StatusCode != 404 {
			t.Fatalf("StatusCode = %d, want 404", err.StatusCode)
		}
	})

	t.Run("stack trace captured", func(t *testing.T) {
		err := NewAppError("TEST", "test", nil)
		if err.Stack == "" {
			t.Fatal("stack trace should not be empty")
		}
		if !strings.Contains(err.Stack, "logger_test.go") {
			t.Fatalf("stack should reference this file: %s", err.Stack)
		}
	})
}

func TestErrorTracker(t *testing.T) {
	t.Run("track and get errors", func(t *testing.T) {
		et := NewErrorTracker()
		err := NewAppError("ERR1", "first error", nil)
		et.Track(err)

		all := et.GetErrors()
		if len(all) != 1 {
			t.Fatalf("expected 1 error, got %d", len(all))
		}
		if all["ERR1"].Count != 1 {
			t.Fatalf("expected count 1, got %d", all["ERR1"].Count)
		}
	})

	t.Run("track duplicate aggregates count", func(t *testing.T) {
		et := NewErrorTracker()
		et.Track(NewAppError("ERR1", "first", nil))
		et.Track(NewAppError("ERR1", "first", nil))
		et.Track(NewAppError("ERR1", "first", nil))

		all := et.GetErrors()
		if all["ERR1"].Count != 3 {
			t.Fatalf("expected count 3, got %d", all["ERR1"].Count)
		}
	})

	t.Run("track different errors", func(t *testing.T) {
		et := NewErrorTracker()
		et.Track(NewAppError("A", "error A", nil))
		et.Track(NewAppError("B", "error B", nil))

		all := et.GetErrors()
		if len(all) != 2 {
			t.Fatalf("expected 2 errors, got %d", len(all))
		}
	})

	t.Run("get top errors", func(t *testing.T) {
		et := NewErrorTracker()
		et.Track(NewAppError("A", "a", nil)) // count 1
		et.Track(NewAppError("B", "b", nil)) // count 1
		et.Track(NewAppError("C", "c", nil)) // count 1
		et.Track(NewAppError("B", "b", nil)) // count 2

		top := et.GetTopErrors(2)
		if len(top) != 2 {
			t.Fatalf("expected 2 top errors, got %d", len(top))
		}
		if top[0].Code != "B" {
			t.Fatalf("expected B as top error, got %s", top[0].Code)
		}
	})

	t.Run("reset", func(t *testing.T) {
		et := NewErrorTracker()
		et.Track(NewAppError("A", "a", nil))
		et.Reset()

		all := et.GetErrors()
		if len(all) != 0 {
			t.Fatalf("expected 0 errors after reset, got %d", len(all))
		}
	})
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrNotFound", ErrNotFound, "resource not found"},
		{"ErrUnauthorized", ErrUnauthorized, "unauthorized access"},
		{"ErrForbidden", ErrForbidden, "access forbidden"},
		{"ErrBadRequest", ErrBadRequest, "invalid request"},
		{"ErrInternal", ErrInternal, "internal server error"},
		{"ErrAgentOffline", ErrAgentOffline, "agent is offline"},
		{"ErrTaskFailed", ErrTaskFailed, "task execution failed"},
		{"ErrDatabaseError", ErrDatabaseError, "database operation failed"},
		{"ErrFileNotFound", ErrFileNotFound, "file not found"},
		{"ErrPermissionDenied", ErrPermissionDenied, "permission denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Fatalf("got %q, want %q", tt.err.Error(), tt.msg)
			}
		})
	}
}
