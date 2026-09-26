package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
)

// newRedirectRequest builds the *http.Request shape a CheckRedirect hook sees.
func newRedirectRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	return &http.Request{URL: mustURL(target)}
}

// The AI endpoint guard exists so a red-team teamserver can point the assistant
// at a local model without weakening the outbound-request protection that also
// covers webhooks, update checks, file fetches and the stager. These tests pin
// both halves: the allowlist works for AI, and everything else stays strict.

func aiGuardServer(t *testing.T, allowAll bool, entries ...string) *Server {
	t.Helper()
	s := &Server{cfg: config.DefaultConfig()}
	s.cfg.AI.AllowPrivateEndpoint = allowAll
	s.cfg.AI.AllowedEndpoints = entries
	return s
}

func TestAIEndpointBlockedByDefault(t *testing.T) {
	s := aiGuardServer(t, false)
	for _, endpoint := range []string{
		"http://127.0.0.1:11434/v1",
		"http://localhost:11434/v1",
		"http://10.0.0.5:8000/v1",
		"http://192.168.1.10:1234/v1",
		"http://169.254.169.254/v1", // cloud metadata
		"http://[::1]:11434/v1",
	} {
		if err := s.validateAIEndpoint(endpoint); err == nil {
			t.Errorf("validateAIEndpoint(%q) allowed a private target with no allowlist", endpoint)
		}
	}
}

func TestAIEndpointAllowsConfiguredPrivateHost(t *testing.T) {
	s := aiGuardServer(t, false, "127.0.0.1:11434", "localhost:11434", "ollama.internal:8000")
	for _, endpoint := range []string{
		"http://127.0.0.1:11434/v1",
		"http://localhost:11434/v1",
		"http://ollama.internal:8000/v1",
	} {
		if err := s.validateAIEndpoint(endpoint); err != nil {
			t.Errorf("validateAIEndpoint(%q) = %v, want allowed", endpoint, err)
		}
	}
	// The allowlist is host-scoped: allowing 11434 must not open other ports
	// on the same host.
	if err := s.validateAIEndpoint("http://127.0.0.1:22/v1"); err == nil {
		t.Error("allowlisting port 11434 must not permit port 22 on the same host")
	}
	// And it must not become a wildcard for the whole private range.
	if err := s.validateAIEndpoint("http://10.0.0.5:8000/v1"); err == nil {
		t.Error("allowlisting loopback must not permit an unrelated private address")
	}
}

func TestAIEndpointAllowlistEntryFormats(t *testing.T) {
	// A bare host entry matches any port on that host.
	s := aiGuardServer(t, false, "ollama.internal")
	if err := s.validateAIEndpoint("http://ollama.internal:11434/v1"); err != nil {
		t.Errorf("bare-host allowlist entry failed: %v", err)
	}
	// A full-URL entry is reduced to its host.
	s = aiGuardServer(t, false, "http://ollama.internal:11434/v1")
	if err := s.validateAIEndpoint("http://ollama.internal:11434/v1"); err != nil {
		t.Errorf("full-URL allowlist entry failed: %v", err)
	}
	// Case and surrounding whitespace must not matter.
	s = aiGuardServer(t, false, "  LocalHost:11434  ")
	if err := s.validateAIEndpoint("http://localhost:11434/v1"); err != nil {
		t.Errorf("allowlist matching is case/space sensitive: %v", err)
	}
}

func TestAIEndpointAllowAllPrivate(t *testing.T) {
	s := aiGuardServer(t, true)
	if err := s.validateAIEndpoint("http://127.0.0.1:11434/v1"); err != nil {
		t.Errorf("allow_private_endpoint did not permit loopback: %v", err)
	}
	if err := s.validateAIEndpoint("http://10.1.2.3:8000/v1"); err != nil {
		t.Errorf("allow_private_endpoint did not permit a private address: %v", err)
	}
	// Even with the blanket opt-in, malformed and non-http URLs stay refused.
	for _, endpoint := range []string{"ftp://127.0.0.1/v1", "file:///etc/passwd", "not a url", ""} {
		if err := s.validateAIEndpoint(endpoint); err == nil {
			t.Errorf("validateAIEndpoint(%q) allowed a malformed/unsupported URL", endpoint)
		}
	}
}

func TestAIEndpointBlockedErrorNamesTheAllowlist(t *testing.T) {
	// The most common cause is pointing the assistant at a local model, so the
	// error must say how to allow it rather than just "blocked".
	s := aiGuardServer(t, false)
	err := s.validateAIEndpoint("http://localhost:11434/v1")
	if err == nil {
		t.Fatal("expected a block error")
	}
	if !strings.Contains(err.Error(), "ai.allowed_endpoints") {
		t.Errorf("error does not mention the remedy: %v", err)
	}
	if !strings.Contains(err.Error(), "localhost:11434") {
		t.Errorf("error does not name the host:port to allow: %v", err)
	}
}

// The relaxation must be scoped to AI. validateExternalURL also guards
// webhooks, notification routes, file fetches, the stager and update checks,
// and those must keep rejecting private targets even when the AI allowlist is
// wide open.
func TestAllowlistDoesNotWeakenOtherFetchPaths(t *testing.T) {
	allowAll := aiGuardServer(t, true)
	if err := allowAll.validateAIEndpoint("http://127.0.0.1:11434/v1"); err != nil {
		t.Fatalf("AI path should be allowed: %v", err)
	}
	// The shared validator is a package-level function with no config access,
	// so it cannot be affected by the AI allowlist by construction. Assert it
	// anyway, because that invariant is the whole reason for the split.
	// (Redirect-hop behaviour of ssrfSafeClient is covered by
	// TestSSRFSafeClientRejectsRedirectToInternal.)
	for _, target := range []string{
		"http://127.0.0.1:11434/v1",
		"http://localhost:8080/",
		"http://10.0.0.1/",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if err := validateExternalURL(target); err == nil {
			t.Errorf("validateExternalURL(%q) was weakened by the AI allowlist", target)
		}
	}
}

// An allowlisted endpoint must not become a springboard: a redirect to a
// private address that is NOT allowlisted has to be refused.
func TestAISSRFClientRefusesRedirectOffAllowlist(t *testing.T) {
	s := aiGuardServer(t, false, "localhost:11434")
	client := s.aiSSRFClient(nil)
	if client.CheckRedirect == nil {
		t.Fatal("aiSSRFClient must install a CheckRedirect hook")
	}
	req := newRedirectRequest(t, "http://10.0.0.5:22/")
	if err := client.CheckRedirect(req, nil); err == nil {
		t.Error("aiSSRFClient followed a redirect to a non-allowlisted private address")
	}
	req = newRedirectRequest(t, "http://localhost:11434/v1")
	if err := client.CheckRedirect(req, nil); err != nil {
		t.Errorf("aiSSRFClient refused a redirect within the allowlist: %v", err)
	}
	// Redirect budget is still enforced.
	if err := client.CheckRedirect(newRedirectRequest(t, "http://localhost:11434/"), make([]*http.Request, 5)); err == nil {
		t.Error("aiSSRFClient did not enforce the redirect limit")
	}
}

func TestAIEndpointFailsClosedWithoutConfig(t *testing.T) {
	// A Server with no config must not fall open.
	for _, s := range []*Server{nil, {}, {cfg: nil}} {
		if err := s.validateAIEndpoint("http://127.0.0.1:11434/v1"); err == nil {
			t.Errorf("validateAIEndpoint allowed loopback on a server without config: %#v", s)
		}
	}
}

func TestAIAllowlistKeys(t *testing.T) {
	cases := map[string][]string{
		"localhost:11434":             {"localhost:11434"},
		"  LocalHost:11434 ":          {"localhost:11434"},
		"http://ollama.internal:8000": {"ollama.internal:8000"},
		"https://gw.example.com/v1":   {"gw.example.com:443"},
		"http://gw.example.com/v1":    {"gw.example.com:80"},
		"ollama.internal":             {"ollama.internal"},
		"":                            nil,
		"http://":                     nil,
	}
	for in, want := range cases {
		got := aiAllowlistKeys(in)
		if len(got) != len(want) {
			t.Errorf("aiAllowlistKeys(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("aiAllowlistKeys(%q) = %v, want %v", in, got, want)
				break
			}
		}
	}
}

func TestAIEndpointAllowedMatching(t *testing.T) {
	if aiEndpointAllowed("http://x:1/", nil) {
		t.Error("nil allowlist must not allow anything")
	}
	if !aiEndpointAllowed("http://a.b:1/", map[string]bool{"*": true}) {
		t.Error("wildcard entry should allow any host")
	}
	allow := map[string]bool{"host.local": true}
	if !aiEndpointAllowed("http://host.local:9999/", allow) {
		t.Error("bare-host entry should match any port")
	}
	if aiEndpointAllowed("http://other:9999/", allow) {
		t.Error("unrelated host must not match")
	}
	// An https URL with no explicit port matches an entry naming :443.
	allow = map[string]bool{"host.local:443": true}
	if !aiEndpointAllowed("https://host.local/v1", allow) {
		t.Error("https URL with implicit port should match a :443 entry")
	}
	if aiEndpointAllowed("http://host.local/v1", allow) {
		t.Error("http (implicit :80) must not match a :443 entry")
	}
}
