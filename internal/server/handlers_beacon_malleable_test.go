package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestMalleableWrapsBeaconResponse verifies the HTTP beacon reply passes
// through applyMalleableProfile when the server-side profile is enabled:
// prepend/append bytes wrap the JSON envelope, and the inner JSON survives.
func TestMalleableWrapsBeaconResponse(t *testing.T) {
	s, _ := v2TestServer(t)
	s.configMu.Lock()
	s.cfg.Malleable.Enabled = true
	s.cfg.Malleable.Prepend = "<html><body>"
	s.cfg.Malleable.Append = "</body></html>"
	s.configMu.Unlock()

	agent := newTCPTestAgent(t, "11111111-2222-4333-8444-555555555555").withRegKey(s.cfg.Server.BeaconKey)

	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	// Registration replies are wrapped just like any other HTTP beacon reply;
	// the agent strips the wrapping before parsing (mirrored here).
	if err := json.Unmarshal([]byte(stripTestWrapper(w.Body.String())), &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("registration failed: %v (body=%s)", err, w.Body.String())
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish: %v", err)
	}

	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid, "pv": 2,
		"info": map[string]string{"hostname": "MALLEABLE", "username": "u", "ip": "10.0.0.8"},
	})
	enc := agent.encryptedFrame(inner)
	if enc == "" {
		t.Fatal("encrypt failed")
	}

	w = v2Post(t, s, enc)
	if w.Code != http.StatusOK {
		t.Fatalf("encrypted beacon: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "<html><body>") || !strings.HasSuffix(body, "</body></html>") {
		t.Fatalf("malleable wrapping missing: %q", body)
	}

	// The wrapped JSON must still parse as the beacon response envelope.
	var env struct {
		CipherB64 string `json:"c"`
	}
	if err := json.Unmarshal([]byte(stripTestWrapper(body)), &env); err != nil || env.CipherB64 == "" {
		t.Fatalf("wrapped body is not valid beacon JSON: %v (body=%q)", err, body)
	}
	if _, err := base64.StdEncoding.DecodeString(env.CipherB64); err != nil {
		t.Fatalf("cipher field not valid base64: %v", err)
	}
}

func stripTestWrapper(body string) string {
	body = strings.TrimPrefix(body, "<html><body>")
	return strings.TrimSuffix(body, "</body></html>")
}

// TestMalleableDisabledLeavesBodyUntouched verifies the fallback path keeps
// the raw JSON reply byte-for-byte when the profile is disabled.
func TestMalleableDisabledLeavesBodyUntouched(t *testing.T) {
	s, _ := v2TestServer(t)
	agent := newTCPTestAgent(t, "22222222-3333-4333-8444-666666666666").withRegKey(s.cfg.Server.BeaconKey)

	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d", w.Code)
	}
	raw := w.Body.String()
	if strings.Contains(raw, "<html>") {
		t.Fatalf("malleable disabled must not wrap: %q", raw)
	}
	var env struct {
		Seq   uint64 `json:"seq"`
		RegOK bool   `json:"reg_ok"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil || !env.RegOK {
		t.Fatalf("raw reply not valid auth envelope: %v (body=%q)", err, raw)
	}
}
