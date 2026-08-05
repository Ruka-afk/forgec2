package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/gin-gonic/gin"
)

const v2TestMasterKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

func TestV2BeaconRegistrationAuth(t *testing.T) {
	s, _ := v2TestServer(t)
	agentUUID := "11111111-2222-4333-8444-555555555555"
	agent := newTCPTestAgent(t, agentUUID).withRegKey(v2TestMasterKey)

	t.Run("plaintext frame rejected", func(t *testing.T) {
		body := `{"uuid":"` + agentUUID + `","info":{"hostname":"TESTPC","username":"u","ip":"10.0.0.1"},"pv":1}`
		w := v2Post(t, s, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for plaintext frame, got %d; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("wrong reg_hmac rejected", func(t *testing.T) {
		// Frame with the wrong master key => wrong per-agent reg key.
		wrongAgent := newTCPTestAgent(t, agentUUID).withRegKey(strings.Repeat("ff", 32))
		w := v2Post(t, s, wrongAgent.registerFrame())
		if w.Code == http.StatusOK {
			t.Fatalf("expected rejection for wrong reg_hmac, got 200; body=%s", w.Body.String())
		}
	})

	t.Run("no beacon key configured fails closed", func(t *testing.T) {
		// A server without a master key must reject all auth frames.
		gin.SetMode(gin.TestMode)
		sm, _ := crypto.NewSessionManager()
		ns := &Server{
			db:               s.db,
			cfg:              &config.Config{},
			sessionManager:   sm,
			beaconDedupCache: make(map[string]time.Time),
			eventManager:     s.eventManager,
			socksEngine:      s.socksEngine,
		}
		ns.configMu.Lock()
		ns.cfg.Server.BeaconKey = ""
		ns.configMu.Unlock()
		r := gin.New()
		r.POST("/beacon", ns.handleBeacon)
		ns.router = r

		w := v2Post(t, ns, agent.registerFrame())
		if w.Code == http.StatusOK {
			t.Fatalf("expected rejection without master key, got 200; body=%s", w.Body.String())
		}
	})

	t.Run("correct reg_hmac accepted", func(t *testing.T) {
		w := v2Post(t, s, agent.registerFrame())
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for correct reg_hmac, got %d; body=%s", w.Code, w.Body.String())
		}
		var resp struct {
			Seq     uint64 `json:"seq"`
			RegOK   bool   `json:"reg_ok"`
			ECDHPub string `json:"ecdh_pub"`
			Mac     string `json:"mac"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("register response parse: %v (body=%s)", err, w.Body.String())
		}
		if !resp.RegOK {
			t.Fatalf("expected reg_ok=true, got %s", w.Body.String())
		}
		if !agent.verifyResponseMAC(resp.Seq, resp.ECDHPub, resp.Mac) {
			t.Fatalf("register response MAC mismatch: %s", w.Body.String())
		}
	})

	t.Run("registration replay rejected", func(t *testing.T) {
		// Re-sending the same register frame (same seq) must be rejected.
		w := v2Post(t, s, agent.registerFrame())
		if w.Code == http.StatusOK {
			t.Fatalf("expected replay rejection, got 200; body=%s", w.Body.String())
		}
	})

	t.Run("stale timestamp rejected", func(t *testing.T) {
		// Register frame with an old ts (clock skew beyond tolerance).
		seq := agent.nextSeq()
		oldTs := time.Now().Unix() - beaconTsTolerance - 60
		idPub := agent.publicKeyB64()
		regHMAC := base64.StdEncoding.EncodeToString(crypto.ComputeRegHMAC(agent.regKey, agentUUID, idPub, oldTs))
		body := `{"uuid":"` + agentUUID + `","seq":` + strconv.FormatUint(seq, 10) + `,"ts":` + strconv.FormatInt(oldTs, 10) + `,"ecdh_pub":"` + idPub + `","id_pub":"` + idPub + `","reg_hmac":"` + regHMAC + `"}`
		w := v2Post(t, s, body)
		if w.Code == http.StatusOK {
			t.Fatalf("expected rejection for stale timestamp, got 200; body=%s", w.Body.String())
		}
	})

	t.Run("encrypted frame for unregistered agent rejected", func(t *testing.T) {
		// Even with a matching session key, an agent that never registered has
		// no implant row: the seq gate rejects the ciphertext frame.
		fresh := newTCPTestAgent(t, "44444444-5555-4333-8444-888888888888").withRegKey(v2TestMasterKey)
		pub, _ := base64.StdEncoding.DecodeString(fresh.publicKeyB64())
		if err := s.sessionManager.EstablishSession(fresh.uuid, pub); err != nil {
			t.Fatalf("establish session: %v", err)
		}
		serverPub := base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
		if err := fresh.establishFromServerKey(serverPub); err != nil {
			t.Fatalf("agent establish session: %v", err)
		}
		inner := `{"uuid":"` + fresh.uuid + `","pv":2}`
		enc := fresh.encryptedFrame([]byte(inner))
		if enc == "" {
			t.Fatalf("agent encrypt failed")
		}
		w := v2Post(t, s, enc)
		if w.Code == http.StatusOK {
			t.Fatalf("expected rejection for unregistered agent ciphertext, got 200; body=%s", w.Body.String())
		}
	})
}

func TestV2ComputeAuthMAC(t *testing.T) {
	regKey := crypto.DeriveRegistrationKeyFromHex(v2TestMasterKey, "uuid-1")
	if regKey == nil {
		t.Fatal("expected reg key")
	}
	a := computeAuthMAC(regKey, "uuid-1", "1", "pub-1")
	if a == "" {
		t.Fatal("expected non-empty MAC")
	}
	if a != computeAuthMAC(regKey, "uuid-1", "1", "pub-1") {
		t.Fatal("MAC must be deterministic for identical inputs")
	}
	if a == computeAuthMAC(regKey, "uuid-1", "2", "pub-1") {
		t.Fatal("MAC must differ when the seq changes")
	}
	if a == computeAuthMAC(regKey, "uuid-1", "1", "pub-2") {
		t.Fatal("MAC must differ when the server pub key changes")
	}
	otherKey := crypto.DeriveRegistrationKeyFromHex(strings.Repeat("ff", 32), "uuid-1")
	if a == computeAuthMAC(otherKey, "uuid-1", "1", "pub-1") {
		t.Fatal("MAC must differ when the reg key changes")
	}
}
