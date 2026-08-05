//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"regexp"
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

var resultIDRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TestEnsureResultIDUUIDv7 verifies agent-generated result ids are RFC 9562
// UUIDv7: sortable and unpredictable, generated only when absent.
func TestEnsureResultIDUUIDv7(t *testing.T) {
	// Empty result id gets populated.
	var res TaskResult
	ensureResultID(&res)
	if !resultIDRe.MatchString(res.ResultID) {
		t.Fatalf("result id not a UUID: %q", res.ResultID)
	}
	if res.ResultID[14] != '7' {
		t.Fatalf("result id version nibble = %c, want 7", res.ResultID[14])
	}

	// Pre-assigned ids (e.g. produced inside the task worker) are preserved.
	res.ResultID = "custom-id"
	ensureResultID(&res)
	if res.ResultID != "custom-id" {
		t.Fatalf("pre-assigned result id overwritten: %q", res.ResultID)
	}

	// Distinct calls produce distinct values.
	var a, b TaskResult
	ensureResultID(&a)
	ensureResultID(&b)
	if a.ResultID == b.ResultID {
		t.Fatal("duplicate result id generated")
	}
	_ = protocol.UUIDv7()
}
