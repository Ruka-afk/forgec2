package db

import (
	"testing"
)

func TestNormalizeKeyScopes(t *testing.T) {
	got, err := NormalizeKeyScopes([]string{"agents.read", "Agents.Read", " tasks.write ", "", "agents.read"})
	if err != nil {
		t.Fatalf("valid scopes rejected: %v", err)
	}
	if got != "agents.read,tasks.write" {
		t.Fatalf("normalized=%q, want sorted deduped csv", got)
	}
	if _, err := NormalizeKeyScopes([]string{"agents.read", "root.everything"}); err == nil {
		t.Fatalf("unknown scope accepted")
	}
	if got, err := NormalizeKeyScopes(nil); err != nil || got != "" {
		t.Fatalf("empty scopes should normalize to empty, got %q err %v", got, err)
	}
}

func TestKeyScopeSet(t *testing.T) {
	if set := KeyScopeSet(""); set != nil {
		t.Fatalf("empty string must yield nil (legacy full key), got %+v", set)
	}
	set := KeyScopeSet("agents.read,tasks.write")
	if set == nil || !set["agents.read"] || !set["tasks.write"] || set["agents.write"] {
		t.Fatalf("bad set: %+v", set)
	}
	if !KeyScopeAllows(nil, "users.delete") {
		t.Fatalf("nil set must allow everything")
	}
	if !KeyScopeAllows(set, "agents.read") || KeyScopeAllows(set, "agents.write") {
		t.Fatalf("membership check wrong: %+v", set)
	}
}

func TestNormalizeKeyCIDRs(t *testing.T) {
	got, err := NormalizeKeyCIDRs("10.0.0.0/8, 203.0.113.7, ::1")
	if err != nil {
		t.Fatalf("valid cidrs rejected: %v", err)
	}
	if got != "10.0.0.0/8,203.0.113.7/32,::1/128" {
		t.Fatalf("normalized=%q", got)
	}
	if _, err := NormalizeKeyCIDRs("10.0.0.0/33"); err == nil {
		t.Fatalf("bad CIDR accepted")
	}
	if _, err := NormalizeKeyCIDRs("not-an-ip"); err == nil {
		t.Fatalf("garbage accepted")
	}
}

func TestKeyCIDRAllows(t *testing.T) {
	if !KeyCIDRAllows("", "8.8.8.8") {
		t.Fatalf("empty allowlist must allow any source")
	}
	if !KeyCIDRAllows("10.0.0.0/8,203.0.113.7/32", "10.1.2.3") {
		t.Fatalf("CIDR member denied")
	}
	if !KeyCIDRAllows("10.0.0.0/8,203.0.113.7/32", "203.0.113.7") {
		t.Fatalf("bare-IP member denied")
	}
	if KeyCIDRAllows("10.0.0.0/8", "192.168.1.1") {
		t.Fatalf("outsider allowed")
	}
	if KeyCIDRAllows("10.0.0.0/8", "not-an-ip") {
		t.Fatalf("unparseable client allowed")
	}
}

func TestAllKeyScopesContainsBulkExport(t *testing.T) {
	found := false
	for _, p := range AllKeyScopes() {
		if p == PermBulkExport {
			found = true
		}
	}
	if !found {
		t.Fatalf("PermBulkExport missing from assignable scopes")
	}
}
