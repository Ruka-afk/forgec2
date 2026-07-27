package server

import (
	"testing"
)

func TestRegisteredTaskTypes_NoDuplicates(t *testing.T) {
	types := GetRegisteredTaskTypes()
	if len(types) == 0 {
		t.Fatal("expected at least one registered task type")
	}
	seen := make(map[string]int)
	for _, tt := range types {
		if line, ok := seen[tt.Type]; ok {
			t.Errorf("duplicate task type %q (first at index %d, second at %d)", tt.Type, line, seen[tt.Type])
		}
		seen[tt.Type] = len(seen)
	}
}

func TestIsKnownTaskType(t *testing.T) {
	tests := []struct {
		typ   string
		known bool
	}{
		{"shell", true},
		{"ps", true},
		{"screenshot", true},
		{"mimikatz", true},
		{"", false},
		{"not_a_real_type", false},
		{"shell; rm -rf /", false},
	}

	for _, tc := range tests {
		got := IsKnownTaskType(tc.typ)
		if got != tc.known {
			t.Errorf("IsKnownTaskType(%q) = %v, want %v", tc.typ, got, tc.known)
		}
	}
}
