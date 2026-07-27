package server

import (
	"strings"
	"testing"
)

func TestParseCredentialsFromText_Mimikatz(t *testing.T) {
	initCredRegexps()

	tests := []struct {
		name     string
		input    string
		wantUser string
		wantDom  string
		wantNTLM string
		wantPW   string
		wantSrc  string
	}{
		{
			name: "single block with all fields",
			input: strings.Join([]string{
				"Authentication Id : 0 ; 99999",
				"Username : admin",
				"Domain : CORP",
				"NTLM : aad3b435b51404eeaad3b435b51404ee",
				"Password : P@ssw0rd",
			}, "\n"),
			wantUser: "admin",
			wantDom:  "CORP",
			wantNTLM: "aad3b435b51404eeaad3b435b51404ee",
			wantPW:   "P@ssw0rd",
			wantSrc:  "mimikatz",
		},
		{
			name: "two blocks separated by blank line",
			input: strings.Join([]string{
				"Authentication Id : 0 ; 99999",
				"Username : admin",
				"Domain : CORP",
				"NTLM : aad3b435b51404eeaad3b435b51404ee",
				"Password : P@ssw0rd",
				"",
				"Authentication Id : 0 ; 88888",
				"Username : john",
				"Domain : DEV",
				"NTLM : bbb3b435b51404eeaad3b435b51404ee",
				"Password : hunter2",
			}, "\n"),
			wantUser: "john",
			wantDom:  "DEV",
			wantNTLM: "bbb3b435b51404eeaad3b435b51404ee",
			wantPW:   "hunter2",
			wantSrc:  "mimikatz",
		},
		{
			name: "NTLM without password",
			input: strings.Join([]string{
				"Username : svc_sql",
				"Domain : CORP",
				"NTLM : 11111111111111111111111111111111",
			}, "\n"),
			wantUser: "svc_sql",
			wantDom:  "CORP",
			wantNTLM: "11111111111111111111111111111111",
			wantPW:   "",
			wantSrc:  "mimikatz",
		},
		{
			name: "password marked null is ignored",
			input: strings.Join([]string{
				"Username : admin",
				"Password : (null)",
			}, "\n"),
			wantUser: "",
			wantDom:  "",
			wantNTLM: "",
			wantPW:   "",
			wantSrc:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCredentialsFromText(tc.input, "agent-1", 42)
			if tc.wantUser == "" {
				if len(got) != 0 {
					t.Fatalf("expected 0 entries, got %d", len(got))
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("expected at least 1 entry, got 0")
			}
			// Find entry matching wantUser (last block in multi-block tests)
			var e = got[len(got)-1]
			if e.Username != tc.wantUser {
				t.Errorf("Username = %q, want %q", e.Username, tc.wantUser)
			}
			if e.Domain != tc.wantDom {
				t.Errorf("Domain = %q, want %q", e.Domain, tc.wantDom)
			}
			if e.Hash != tc.wantNTLM {
				t.Errorf("Hash = %q, want %q", e.Hash, tc.wantNTLM)
			}
			if e.Password != tc.wantPW {
				t.Errorf("Password = %q, want %q", e.Password, tc.wantPW)
			}
			if e.Source != tc.wantSrc {
				t.Errorf("Source = %q, want %q", e.Source, tc.wantSrc)
			}
			if e.AgentID != "agent-1" {
				t.Errorf("AgentID = %q, want %q", e.AgentID, "agent-1")
			}
			if e.TaskID != 42 {
				t.Errorf("TaskID = %d, want %d", e.TaskID, 42)
			}
		})
	}
}

func TestParseCredentialsFromText_SAM(t *testing.T) {
	initCredRegexps()

	tests := []struct {
		name      string
		input     string
		wantCount int
		wantUser  string
		wantHash  string
	}{
		{
			name:      "single SAM line",
			input:     "admin:500:aad3b435b51404eeaad3b435b51404ee:aad3b435b51404eeaad3b435b51404ee:::\n",
			wantCount: 1,
			wantUser:  "admin",
			wantHash:  "aad3b435b51404eeaad3b435b51404ee",
		},
		{
			name: "multiple SAM lines",
			input: strings.Join([]string{
				"admin:500:aad3b435b51404eeaad3b435b51404ee:aad3b435b51404eeaad3b435b51404ee:::",
				"guest:501:00000000000000000000000000000000:00000000000000000000000000000000:::",
				"john:1001:bbb3b435b51404eeaad3b435b51404ee:bbb3b435b51404eeaad3b435b51404ee:::",
			}, "\n"),
			wantCount: 3,
			wantUser:  "john",
			wantHash:  "bbb3b435b51404eeaad3b435b51404ee",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCredentialsFromText(tc.input, "agent-sam", 7)
			if len(got) != tc.wantCount {
				t.Fatalf("got %d entries, want %d", len(got), tc.wantCount)
			}
			last := got[len(got)-1]
			if last.Username != tc.wantUser {
				t.Errorf("Username = %q, want %q", last.Username, tc.wantUser)
			}
			if last.Hash != tc.wantHash {
				t.Errorf("Hash = %q, want %q", last.Hash, tc.wantHash)
			}
			if last.Source != "sam" {
				t.Errorf("Source = %q, want %q", last.Source, "sam")
			}
			if last.Type != "ntlm" {
				t.Errorf("Type = %q, want %q", last.Type, "ntlm")
			}
		})
	}
}

func TestParseCredentialsFromText_Simple(t *testing.T) {
	initCredRegexps()

	tests := []struct {
		name     string
		input    string
		wantDom  string
		wantUser string
		wantPW   string
	}{
		{
			name:     "domain backslash user colon password",
			input:    "CORP\\admin:P@ssw0rd",
			wantDom:  "CORP",
			wantUser: "admin",
			wantPW:   "P@ssw0rd",
		},
		{
			name:     "user colon password without domain",
			input:    "john:letmein",
			wantDom:  "",
			wantUser: "john",
			wantPW:   "letmein",
		},
		{
			name: "multiple simple lines",
			input: strings.Join([]string{
				"CORP\\admin:P@ssw0rd",
				"john:letmein",
			}, "\n"),
			wantDom:  "",
			wantUser: "john",
			wantPW:   "letmein",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCredentialsFromText(tc.input, "agent-simple", 99)
			if len(got) == 0 {
				t.Fatal("expected at least 1 entry, got 0")
			}
			last := got[len(got)-1]
			if last.Domain != tc.wantDom {
				t.Errorf("Domain = %q, want %q", last.Domain, tc.wantDom)
			}
			if last.Username != tc.wantUser {
				t.Errorf("Username = %q, want %q", last.Username, tc.wantUser)
			}
			if last.Password != tc.wantPW {
				t.Errorf("Password = %q, want %q", last.Password, tc.wantPW)
			}
			if last.Source != "manual_parse" {
				t.Errorf("Source = %q, want %q", last.Source, "manual_parse")
			}
			if last.Type != "cleartext" {
				t.Errorf("Type = %q, want %q", last.Type, "cleartext")
			}
		})
	}
}

func TestParseCredentialsFromText_Empty(t *testing.T) {
	initCredRegexps()

	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"only whitespace", "   \n  \n  "},
		{"only newlines", "\n\n\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCredentialsFromText(tc.input, "agent-empty", 0)
			if len(got) != 0 {
				t.Fatalf("expected 0 entries, got %d", len(got))
			}
		})
	}
}

func TestParseCredentialsFromText_Invalid(t *testing.T) {
	initCredRegexps()

	tests := []struct {
		name  string
		input string
	}{
		{"random text", "lorem ipsum dolor sit amet"},
		{"no separator", "no colon here at all"},
		{"partial username only", "Username : admin"},
		{"domain only", "Domain : CORP"},
		{"ntlm only no username", "NTLM : aad3b435b51404eeaad3b435b51404ee"},
		{"bad ntlm hash too short", "NTLM : aabb"},
		{"colon only", "::::"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCredentialsFromText(tc.input, "agent-invalid", 0)
			if len(got) != 0 {
				t.Fatalf("expected 0 entries for %q, got %d: %+v", tc.name, len(got), got)
			}
		})
	}
}

func TestCSVSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "hello", "hello"},
		{"empty string", "", ""},
		{"double quotes escaped", `say "hi"`, `say ""hi""`},
		{"comma preserved", "a,b,c", "a,b,c"},
		{"equals prefix", "=SUM(A1)", "'=SUM(A1)"},
		{"plus prefix", "+cmd", "'+cmd"},
		{"minus prefix", "-flag", "'-flag"},
		{"at prefix", "@import", "'@import"},
		{"tab prefix", "\tdata", "'\tdata"},
		{"newline prefix", "\ndata", "'\ndata"},
		{"carriage return prefix", "\rdata", "'\rdata"},
		{"no prefix for normal", "clean_value", "clean_value"},
		{"quotes with formula", `="test"`, `'=""test""`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := csvSanitize(tc.input)
			if got != tc.want {
				t.Errorf("csvSanitize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
