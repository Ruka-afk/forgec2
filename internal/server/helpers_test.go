package server

import (
	"testing"
)

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "hello", "hello"},
		{"percent wildcard", "100%", `100\%`},
		{"underscore wildcard", "test_file", `test\_file`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"bracket open", "query[0]", `query\[0\]`},
		{"bracket close", "data[1]", `data\[1\]`},
		{"all special", `a%b_c\d[e]`, `a\%b\_c\\d\[e\]`},
		{"empty string", "", ""},
		{"no special chars", "abcdef123", "abcdef123"},
		{"SQL injection attempt", "%' OR '1'='1", `\%' OR '1'='1`},
		{"double backslash", `\\\\`, `\\\\\\\\`},
		{"mixed escapes", `test%_\[]`, `test\%\_\\\[\]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeLike(tc.input)
			if got != tc.expected {
				t.Errorf("escapeLike(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		context string
		want    string
	}{
		{"duplicate record", "UNIQUE constraint failed", "", "A record with the same value already exists"},
		{"no such table", "no such table: users", "", "Database schema error — restart the server to apply migrations"},
		{"foreign key", "FOREIGN KEY constraint failed", "", "Cannot complete: related records still reference this item"},
		{"record not found", "record not found", "", "Record not found"},
		{"permission denied", "permission denied", "", "Permission denied"},
		{"connection refused", "connection refused", "", "External service is unreachable"},
		{"timeout", "context deadline exceeded", "", "Operation timed out"},
		{"disk full", "no space left on device", "", "Disk space is full"},
		{"unknown error", "something weird happened", "save config", "save config failed"},
		{"unknown error no context", "unexpected issue", "", "An internal error occurred"},
		{"nil error", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.errMsg != "" {
				err = &testError{msg: tc.errMsg}
			}
			got := sanitizeError(err, tc.context)
			if got != tc.want {
				t.Errorf("sanitizeError(%q, %q) = %q, want %q", tc.errMsg, tc.context, got, tc.want)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func TestValidateFilePath(t *testing.T) {
	base := t.TempDir()
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid path", base + "/test.txt", false},
		{"traversal", base + "/../../../etc/passwd", true},
		{"same dir", base, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFilePath(tc.path, base)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateFilePath(%q, %q) error = %v, wantErr %v", tc.path, base, err, tc.wantErr)
			}
		})
	}
}
