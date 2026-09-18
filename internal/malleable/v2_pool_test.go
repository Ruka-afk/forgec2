package malleable

import (
	"testing"
)

// TestValidateRequestHeaderPool proves the pool grammar is enforced at load:
// "Name: value" with safe characters, nothing else.
func TestValidateRequestHeaderPool(t *testing.T) {
	valid := &ProfileV2{Name: "t", RequestHeaderPool: []string{"X-Trace: abc", "X-Req: a:b"}}
	if err := ValidateProfileV2(valid); err != nil {
		t.Fatalf("valid pool rejected: %v", err)
	}
	for _, pool := range [][]string{
		{"nogood"},
		{":noval"},
		{"noname:"},
		{"X-Evil: a\r\nb"},
		{"X-`q`: v"},
	} {
		p := &ProfileV2{Name: "t", RequestHeaderPool: pool}
		if err := ValidateProfileV2(p); err == nil {
			t.Fatalf("pool %q accepted, want reject", pool)
		}
	}
}
