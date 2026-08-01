package server

import "testing"

func TestIsValidAgentID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"11111111-2222-4333-8444-555555555555", true},
		{"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", true},
		{"AAAAAAAA-BBBB-4CCC-8DDD-EEEEEEEEEEEE", true},
		{"ws_11111111-2222-4333-8444-555555555555", true},
		{"unknown-11111111-2222-4333-8444-555555555555", true},
		{"", false},
		{"..", false},
		{"../..", false},
		{"..\\..", false},
		{"agent-1", false},
		{"test-agent", false},
		{"11111111", false},
		{"11111111-2222-4333-8444-555555555555-extra", false},
		{"11111111-2222-4333-8444-55555555555g", false},
		{"/etc/passwd", false},
	}
	for _, tt := range tests {
		if got := isValidAgentID(tt.id); got != tt.want {
			t.Errorf("isValidAgentID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}
