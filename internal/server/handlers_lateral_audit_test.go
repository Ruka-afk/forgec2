package server

import (
	"strings"
	"testing"
)

// TestLateralAuditSummaryRedactsCredentials ensures the lateral-movement spec
// redactor never leaks password/hash/credential/pivot material into audit logs
// or server logs, while keeping investigation-relevant method/target/username.
func TestLateralAuditSummaryRedactsCredentials(t *testing.T) {
	spec := `{"source":"abc","target":"10.0.0.5","method":"smb","username":"CORP\\jsmith",` +
		`"password":"S3cretP@ss!","hash":"aad3b435b51404eeaad3b435b51404ee","credential":"7",` +
		`"key_path":"/root/.ssh/id_rsa","pivot":"10.0.0.9","share":"ADMIN$","command":"whoami"}`
	out := lateralAuditSummary(spec)

	if !strings.Contains(out, "method=smb") || !strings.Contains(out, "target=10.0.0.5") || !strings.Contains(out, "username=CORP\\jsmith") {
		t.Fatalf("summary missing investigation fields: %q", out)
	}
	for _, secret := range []string{"S3cretP@ss!", "aad3b435b51404eeaad3b435b51404ee", "/root/.ssh/id_rsa", "10.0.0.9", "whoami"} {
		if strings.Contains(out, secret) {
			t.Fatalf("summary leaked secret %q: %q", secret, out)
		}
	}
	if strings.Contains(out, "ADMIN$") {
		t.Fatalf("summary leaked share detail: %q", out)
	}
}

// TestLateralAuditSummaryMalformedAndEmpty ensures garbage specs degrade to a
// byte count and empty specs produce a constant label, without panicking.
func TestLateralAuditSummaryMalformedAndEmpty(t *testing.T) {
	if out := lateralAuditSummary(""); out != "lateral movement" {
		t.Fatalf("empty spec: %q", out)
	}
	if out := lateralAuditSummary("not-json-at-all"); !strings.Contains(out, "spec 15 bytes") {
		t.Fatalf("malformed spec: %q", out)
	}
	if out := lateralAuditSummary(`{"method":"smb"}`); !strings.Contains(out, "method=smb") {
		t.Fatalf("minimal spec: %q", out)
	}
}