package server

import (
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestFSTaskTypesRegisteredAndDocumented pins the mkdir/rename/chmod
// registry entries: they must be known task types, carry the parameter docs
// the UI renders, and mark the required parameters so createTask validation
// enforces them.
func TestFSTaskTypesRegisteredAndDocumented(t *testing.T) {
	wantParams := map[string]int{"mkdir": 1, "rename": 2, "chmod": 2}
	for typ, wantCount := range wantParams {
		info, ok := getTaskTypeInfo(typ)
		if !ok {
			t.Fatalf("task type %q missing from registry", typ)
		}
		if !IsKnownTaskType(typ) || !protocol.ValidTaskType(typ) {
			t.Fatalf("task type %q not recognized as valid", typ)
		}
		required := 0
		for _, p := range info.Parameters {
			if p.Required {
				required++
			}
		}
		if required != wantCount {
			t.Fatalf("%s: required params = %d, want %d (params: %+v)", typ, required, wantCount, info.Parameters)
		}
	}
}

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
