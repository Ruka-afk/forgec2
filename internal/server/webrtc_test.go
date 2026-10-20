package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
)

func testWebRTCConfig() *config.Config {
	cfg := &config.Config{}
	cfg.WebRTC.Enabled = true
	cfg.WebRTC.TurnAddr = ":3478"
	cfg.WebRTC.Realm = "forgec2"
	cfg.WebRTC.RelayMinPort = 49152
	cfg.WebRTC.RelayMaxPort = 65535
	cfg.WebRTC.SessionTTLSeconds = 3600
	cfg.WebRTC.MaxSessions = 2
	return cfg
}

func TestNormalizeWebRTCStart(t *testing.T) {
	got, err := normalizeWebRTCStart(webrtcStartRequest{})
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if got.Width != 1280 || got.Height != 720 || got.FPS != 30 || got.BitrateKbps != 2500 {
		t.Fatalf("unexpected 720p defaults: %+v", got)
	}

	got, err = normalizeWebRTCStart(webrtcStartRequest{Width: 1920, Height: 1080, FPS: 30, BitrateKbps: 5000, Encoder: "H264_NVENC"})
	if err != nil {
		t.Fatalf("1080p: %v", err)
	}
	if got.Encoder != "h264_nvenc" || got.BitrateKbps != 5000 {
		t.Fatalf("encoder/bitrate not normalized: %+v", got)
	}

	for _, req := range []webrtcStartRequest{
		{Width: 3840, Height: 2160},         // 4K is not guaranteed
		{Width: 1281, Height: 720},          // odd width
		{Width: 1280, Height: 720, FPS: 60}, // above the 30 fps ceiling
		{Width: 1280, Height: 720, BitrateKbps: 20000},
		{Width: 1280, Height: 720, Encoder: "vp9"},
	} {
		if _, err := normalizeWebRTCStart(req); err == nil {
			t.Errorf("expected rejection for %+v", req)
		}
	}
}

func TestWebRTCSessionTokenIsSingleUseAndAgentBound(t *testing.T) {
	h := newWebRTCHub(testWebRTCConfig())
	sess, token, err := h.createSession("agent-1", 7, 3, webrtcStartOptions{Width: 1280, Height: 720}, time.Minute)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer sess.end("test")

	if _, err := h.sessionForOperator(sess.ID, 8); err == nil {
		t.Fatal("another operator must not read the session")
	}
	got, err := h.sessionForOperator(sess.ID, 7)
	if err != nil || got != sess {
		t.Fatalf("owner lookup: %v", err)
	}

	// A token presented for the wrong agent is consumed, not retried.
	if _, err := h.authenticateAgent(sess.ID, token, "agent-2"); err == nil {
		t.Fatal("token must be bound to the session agent")
	}
	if _, err := h.authenticateAgent(sess.ID, token, "agent-1"); err == nil {
		t.Fatal("token must be single-use")
	}
}

func TestWebRTCSessionLimitsAndExpiry(t *testing.T) {
	h := newWebRTCHub(testWebRTCConfig())
	first, _, err := h.createSession("agent-1", 1, 0, webrtcStartOptions{}, time.Minute)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	defer first.end("test")
	if _, _, err := h.createSession("agent-1", 1, 0, webrtcStartOptions{}, time.Minute); err == nil {
		t.Fatal("duplicate agent session must fail")
	}
	second, _, err := h.createSession("agent-2", 1, 0, webrtcStartOptions{}, time.Minute)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	defer second.end("test")
	if _, _, err := h.createSession("agent-3", 1, 0, webrtcStartOptions{}, time.Minute); err == nil {
		t.Fatal("max sessions must be enforced")
	}
	second.end("test")
	expired, expiredToken, err := h.createSession("agent-4", 1, 0, webrtcStartOptions{}, time.Minute)
	if err != nil {
		t.Fatalf("expired setup: %v", err)
	}
	expired.ExpiresAt = time.Now().Add(-time.Second)
	if _, err := h.sessionForOperator(expired.ID, 1); err == nil {
		t.Fatal("expired session must not be readable")
	}
	if _, err := h.authenticateAgent(expired.ID, expiredToken, "agent-4"); err == nil {
		t.Fatal("expired session must not accept signaling")
	}
	expired.end("test")
}

func TestWebRTCTURNCredentials(t *testing.T) {
	h := newWebRTCHub(testWebRTCConfig())
	sess, _, err := h.createSession("agent-1", 1, 0, webrtcStartOptions{}, time.Minute)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer sess.end("test")

	key, ok := h.authenticateTURN(sess.TurnUser, "forgec2")
	if !ok || len(key) == 0 {
		t.Fatal("valid TURN credentials must authenticate")
	}
	if _, ok := h.authenticateTURN(sess.TurnUser, "other-realm"); ok {
		t.Fatal("realm mismatch must fail")
	}
	if _, ok := h.authenticateTURN("no-such-user", "forgec2"); ok {
		t.Fatal("unknown TURN user must fail")
	}

	sess.hub.mu.Lock()
	cred := h.turn[sess.TurnUser]
	cred.expires = time.Now().Add(-time.Second)
	h.turn[sess.TurnUser] = cred
	sess.hub.mu.Unlock()
	if _, ok := h.authenticateTURN(sess.TurnUser, "forgec2"); ok {
		t.Fatal("expired TURN credentials must fail")
	}
}

func TestWebRTCTURNUsesConfiguredRealmFallback(t *testing.T) {
	cfg := testWebRTCConfig()
	cfg.WebRTC.Realm = "" // ensureTURN advertises "forgec2" in this case
	h := newWebRTCHub(cfg)
	sess, _, err := h.createSession("agent-1", 1, 0, webrtcStartOptions{}, time.Minute)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer sess.end("test")

	key, ok := h.authenticateTURN(sess.TurnUser, "forgec2")
	if !ok || len(key) == 0 {
		t.Fatal("empty configured realm must fall back to forgec2")
	}
	if _, ok := h.authenticateTURN(sess.TurnUser, "other-realm"); ok {
		t.Fatal("realm mismatch must still fail with the fallback realm")
	}
}

func TestDetectLocalIPv4(t *testing.T) {
	if got := detectLocalIPv4(); got == "" {
		t.Fatal("detectLocalIPv4 returned empty")
	}
}
