package server

import (
	"strings"
	"testing"
)

// The implant parses lateral specs as "type|target|user|pass|cmd"
// (internal/payload/agent/agent_windows.go, lateralMove). The server used to
// send a JSON object, which never contained a '|', so SplitN produced a single
// part and every lateral task failed with a format error.

func TestEncodeLateralSpec_MatchesImplantParser(t *testing.T) {
	cases := []struct {
		name string
		in   lateralSpec
		want string
	}{
		{
			name: "full fields",
			in:   lateralSpec{Method: "WMI", Target: "10.0.0.9", Username: "admin", Password: "s3cret", Command: "whoami"},
			want: "wmi|10.0.0.9|admin|s3cret|whoami",
		},
		{
			name: "empty command falls back to the implant default",
			in:   lateralSpec{Method: "psexec", Target: "host", Username: "u", Password: "p"},
			want: "psexec|host|u|p|whoami",
		},
		{
			name: "no credential still keeps five slots",
			in:   lateralSpec{Method: "dcom", Target: "host"},
			want: "dcom|host|||whoami",
		},
		{
			// SplitN(..., 5) keeps the tail intact, so a piped shell command
			// in the last slot is safe.
			name: "pipes in the command survive",
			in:   lateralSpec{Method: "wmi", Target: "h", Username: "u", Password: "p", Command: "type a.txt | findstr secret"},
			want: "wmi|h|u|p|type a.txt | findstr secret",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := encodeLateralSpec(tc.in)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			// The implant requires at least three parts or it errors out.
			if n := len(strings.SplitN(got, "|", 5)); n < 3 {
				t.Fatalf("encoded spec yields only %d part(s); the implant needs >= 3", n)
			}
		})
	}
}

// A pipe in a field before the command would shift every later field.
func TestEncodeLateralSpec_RejectsPipeInLeadingFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   lateralSpec
	}{
		{"target", lateralSpec{Method: "wmi", Target: "a|b"}},
		{"username", lateralSpec{Method: "wmi", Target: "h", Username: "a|b"}},
		{"password", lateralSpec{Method: "wmi", Target: "h", Password: "a|b"}},
		{"method", lateralSpec{Method: "wm|i", Target: "h"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := encodeLateralSpec(tc.in); err == nil {
				t.Fatalf("expected a rejection for a pipe in %s", tc.name)
			}
		})
	}
}

func TestLateralAuditSummary_OmitsCredentials(t *testing.T) {
	// Pipe form: the password slot and the command must both stay out.
	out := lateralAuditSummary("wmi|10.0.0.9|admin|hunter2|whoami")
	for _, leak := range []string{"hunter2", "whoami"} {
		if strings.Contains(out, leak) {
			t.Fatalf("audit summary leaked %q: %s", leak, out)
		}
	}
	for _, want := range []string{"method=wmi", "target=10.0.0.9", "username=admin"} {
		if !strings.Contains(out, want) {
			t.Fatalf("audit summary missing %q: %s", want, out)
		}
	}

	// JSON form keeps working for the other caller.
	outJSON := lateralAuditSummary(`{"method":"smb","target":"10.0.0.5","username":"jsmith","password":"p4ss"}`)
	for _, leak := range []string{"p4ss"} {
		if strings.Contains(outJSON, leak) {
			t.Fatalf("json summary leaked %q: %s", leak, outJSON)
		}
	}
	if !strings.Contains(outJSON, "method=smb") {
		t.Fatalf("json summary lost the method: %s", outJSON)
	}
}
