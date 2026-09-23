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
		{"11111111222243338444555555555555", true},
		{"B5EB40D2AA40402DB019272452C504B3", true},
		{"ws_11111111-2222-4333-8444-555555555555", true},
		{"unknown-11111111-2222-4333-8444-555555555555", true},
		{"ws_11111111222243338444555555555555", true},
		{"", false},
		{"..", false},
		{"../..", false},
		{"..\\..", false},
		{"agent-1", false},
		{"test-agent", false},
		{"11111111", false},
		{"1111111122224333844455555555555", false},
		{"111111112222433384445555555555555", false},
		{"1111111122224333844455555555555g", false},
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

func TestNormalizeAgentID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"b5eb40d2aa40402db019272452c504b3", "b5eb40d2-aa40-402d-b019-272452c504b3"},
		{"B5EB40D2AA40402DB019272452C504B3", "b5eb40d2-aa40-402d-b019-272452c504b3"},
		{"b5eb40d2-aa40-402d-b019-272452c504b3", "b5eb40d2-aa40-402d-b019-272452c504b3"},
		{"B5EB40D2-AA40-402D-B019-272452C504B3", "b5eb40d2-aa40-402d-b019-272452c504b3"},
		{"ws_11111111222243338444555555555555", "11111111-2222-4333-8444-555555555555"},
		{"not-a-uuid", "not-a-uuid"},
	}
	for _, tt := range tests {
		if got := normalizeAgentID(tt.in); got != tt.want {
			t.Errorf("normalizeAgentID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
