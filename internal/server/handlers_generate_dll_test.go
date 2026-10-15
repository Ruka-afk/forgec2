package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// dllForm builds a minimal binary-build form. obfuscate is the deterministic
// half of the DLL pre-flight: garble cannot be combined with
// -buildmode=c-shared on any host, so the refusal does not depend on whether
// this machine happens to have mingw-w64 installed.
func dllForm(obfuscate bool) url.Values {
	form := url.Values{
		"c2_url":     {"http://127.0.0.1:8000"},
		"protocol":   {"http"},
		"interval":   {"60"},
		"filename":   {"payload.dll"},
		"arch":       {"amd64"},
		"generation": {"dll"},
		// buildImplantConfig rejects a form with neither a listener nor a DNS
		// domain, and that check runs before the DLL pre-flight.
		"dns_domain": {"c2.example.com"},
	}
	if obfuscate {
		form.Set("obfuscate", "true")
	}
	return form
}

// dllServer builds a server whose implant config can actually be minted: the
// config builder needs a beacon key, a live v3 registration secret store and
// the reg_secrets table, and it refuses before the DLL pre-flight is ever
// reached. mustTenantServer provides the full schema; the build-job map is
// added because this handler queues into it.
func dllServer(t *testing.T) *Server {
	t.Helper()
	s := mustTenantServer(t)
	s.cfg.Server.BeaconKey = strings.Repeat("ab", 32)
	if s.buildJobs == nil {
		s.buildJobs = make(map[string]*BuildJob)
	}
	if err := s.syncRegSecrets(); err != nil {
		t.Fatalf("sync reg secrets: %v", err)
	}
	return s
}

func postDLL(t *testing.T, s *Server, form url.Values) (int, string) {
	t.Helper()
	c, w := tenantCtx(http.MethodPost, "/x", "dll-op", form.Encode())
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "admin")
	s.handleGenerateDLL(c)
	return w.Code, w.Body.String()
}

// TestGenerateDLLRefusesObfuscationBeforeQueueing proves the pre-flight runs
// before a build job exists.
//
// The old flow answered 200 {"status":"building"} and only failed once the
// queued job reached the compiler check, after extracting agent sources into a
// temp dir. An operator's first and only signal was a build that stalled and
// then broke, for a conflict that was knowable up front.
func TestGenerateDLLRefusesObfuscationBeforeQueueing(t *testing.T) {
	s := dllServer(t)

	code, body := postDLL(t, s, dllForm(true))

	if code != http.StatusUnprocessableEntity {
		t.Fatalf("obfuscated DLL build: status = %d, want 422; body=%s", code, body)
	}
	if !strings.Contains(body, "garble") {
		t.Fatalf("refusal does not explain the conflict: %s", body)
	}
	if strings.Contains(body, "build_id") {
		t.Fatalf("refusal still handed back a build id: %s", body)
	}

	// No job may be left behind for an operator to wait on.
	s.buildJobsMu.RLock()
	jobs := len(s.buildJobs)
	s.buildJobsMu.RUnlock()
	if jobs != 0 {
		t.Fatalf("refused request still queued %d build job(s)", jobs)
	}
}

// TestGenerateDLLRefusalIsMachineReadable keeps the refusal usable by the UI:
// it must stay a normal JSON error envelope rather than a bare string, so the
// client renders the message instead of falling back to a generic failure.
func TestGenerateDLLRefusalIsMachineReadable(t *testing.T) {
	s := dllServer(t)

	_, body := postDLL(t, s, dllForm(true))

	var envelope struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("refusal is not a JSON error envelope: %v body=%s", err, body)
	}
	if envelope.Success {
		t.Fatalf("refusal reported success=true: %s", body)
	}
	if envelope.Error == "" {
		t.Fatalf("refusal carries no error text: %s", body)
	}
}
