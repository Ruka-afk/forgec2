package protocol

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

var uuidv7Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestUUIDv7Format(t *testing.T) {
	u := UUIDv7()
	if !uuidv7Re.MatchString(u) {
		t.Fatalf("UUIDv7 not well-formed: %q", u)
	}
	// Version nibble must be 7 and variant nibble 8/9/a/b.
	if u[14] != '7' {
		t.Fatalf("version nibble = %c, want 7", u[14])
	}
	switch u[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Fatalf("variant nibble = %c, want 8..b", u[19])
	}
}

func TestUUIDv7Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		u := UUIDv7()
		if seen[u] {
			t.Fatalf("duplicate UUIDv7 generated: %q", u)
		}
		seen[u] = true
	}
}

func TestUUIDv7Sortable(t *testing.T) {
	// Time-ordered: the embedded millis timestamp makes earlier ids lexically
	// smaller across non-overlapping time windows.
	a := UUIDv7()
	time.Sleep(5 * time.Millisecond)
	b := UUIDv7()
	if strings.Compare(a, b) >= 0 {
		t.Fatalf("uuidv7 not sortable: %q >= %q", a, b)
	}
}
