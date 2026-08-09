package server

import (
	"crypto/hmac"
	"crypto/sha256"
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
		regHMAC := base64.StdEncoding.EncodeToString(crypto.ComputeRegHMAC(agent.regKey, agentUUID, idPub, oldTs, seq))
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

// TestV2OversizedBodyRejectedBeforeDecrypt proves the raw body is rejected by
// the pre-decode length check (default 10MB max payload → ~13.4MB raw limit)
// instead of being base64-decoded and run through AES-GCM first.
func TestV2OversizedBodyRejectedBeforeDecrypt(t *testing.T) {
	s, _ := v2TestServer(t)
	big := strings.Repeat("A", 15*1024*1024)
	body := `{"uuid":"11111111-2222-4333-8444-555555555555","seq":1,"ts":` + strconv.FormatInt(time.Now().Unix(), 10) + `,"c":"` + big + `"}`
	w := v2Post(t, s, body)
	if w.Code == http.StatusOK {
		t.Fatalf("expected rejection for oversized body, got 200; body=%s", w.Body.String())
	}
}

// TestV2RegSeqTamperRejected proves the frame seq is bound into the
// registration HMAC: a captured registration frame replayed with an inflated
// seq (which previously burned the replay window) must now fail auth.
func TestV2RegSeqTamperRejected(t *testing.T) {
	s, _ := v2TestServer(t)
	agentUUID := "aaaa1111-2222-4333-8444-333333333333"
	agent := newTCPTestAgent(t, agentUUID).withRegKey(s.cfg.Server.BeaconKey)

	seq := agent.nextSeq()
	ts := time.Now().Unix()
	idPub := agent.publicKeyB64()
	// HMAC is computed over seq+1 while the envelope carries seq: the seq
	// must be part of the MAC input, so this must be rejected.
	badMAC := base64.StdEncoding.EncodeToString(crypto.ComputeRegHMAC(agent.regKey, agentUUID, idPub, ts, seq+1))
	body := `{"uuid":"` + agentUUID + `","seq":` + strconv.FormatUint(seq, 10) + `,"ts":` + strconv.FormatInt(ts, 10) + `,"ecdh_pub":"` + idPub + `","id_pub":"` + idPub + `","reg_hmac":"` + badMAC + `"}`
	w := v2Post(t, s, body)
	if w.Code == http.StatusOK {
		t.Fatalf("expected rejection for seq-tampered reg_hmac, got 200; body=%s", w.Body.String())
	}
}

// TestV2HandshakeSeqTamperRejected proves the frame seq is bound into the
// handshake MAC: replaying a captured handshake frame with an inflated seq
// must fail auth instead of advancing the replay window.
func TestV2HandshakeSeqTamperRejected(t *testing.T) {
	s, _ := v2TestServer(t)
	agentUUID := "bbbb2222-3333-4333-8444-444444444444"
	agent := newTCPTestAgent(t, agentUUID).withRegKey(s.cfg.Server.BeaconKey)

	// Register normally.
	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// Handshake envelope carrying seq, but MAC computed over seq+1.
	seq := agent.nextSeq()
	ts := time.Now().Unix()
	pub := agent.publicKeyB64()
	mac := hmac.New(sha256.New, agent.regKey)
	mac.Write([]byte(agentUUID))
	mac.Write([]byte(pub))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte(strconv.FormatUint(seq+1, 10)))
	tamperedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	body := `{"uuid":"` + agentUUID + `","seq":` + strconv.FormatUint(seq, 10) + `,"ts":` + strconv.FormatInt(ts, 10) + `,"ecdh_pub":"` + pub + `","mac":"` + tamperedMAC + `"}`
	w = v2Post(t, s, body)
	if w.Code == http.StatusOK {
		t.Fatalf("expected rejection for seq-tampered handshake, got 200; body=%s", w.Body.String())
	}
}

// TestV2SeqHardCapLockout proves a cryptographically valid frame with an
// implausible seq jump is rejected and locks the agent out briefly instead of
// burning the replay window.
func TestV2SeqHardCapLockout(t *testing.T) {
	s, _ := v2TestServer(t)
	agentUUID := "cccc3333-4444-4333-8444-555555555555"
	agent := newTCPTestAgent(t, agentUUID).withRegKey(s.cfg.Server.BeaconKey)

	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// Valid-MAC handshake but seq jumps far beyond the hard cap (maxSeqJump*10).
	hugeSeq := agent.nextSeq() + maxSeqJump*100
	ts := time.Now().Unix()
	pub := agent.publicKeyB64()
	mac := hmac.New(sha256.New, agent.regKey)
	mac.Write([]byte(agentUUID))
	mac.Write([]byte(pub))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte(strconv.FormatUint(hugeSeq, 10)))
	validMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	body := `{"uuid":"` + agentUUID + `","seq":` + strconv.FormatUint(hugeSeq, 10) + `,"ts":` + strconv.FormatInt(ts, 10) + `,"ecdh_pub":"` + pub + `","mac":"` + validMAC + `"}`
	w = v2Post(t, s, body)
	if w.Code == http.StatusOK {
		t.Fatalf("expected rejection for hard-cap seq jump, got 200; body=%s", w.Body.String())
	}

	// A perfectly normal next frame must also be rejected while locked out:
	// the huge jump must not have advanced the window either.
	okSeq := hugeSeq - maxSeqJump/2
	ts2 := time.Now().Unix()
	pub2 := agent.publicKeyB64()
	mac2 := hmac.New(sha256.New, agent.regKey)
	mac2.Write([]byte(agentUUID))
	mac2.Write([]byte(pub2))
	mac2.Write([]byte(strconv.FormatInt(ts2, 10)))
	mac2.Write([]byte(strconv.FormatUint(okSeq, 10)))
	okMAC := base64.StdEncoding.EncodeToString(mac2.Sum(nil))
	body2 := `{"uuid":"` + agentUUID + `","seq":` + strconv.FormatUint(okSeq, 10) + `,"ts":` + strconv.FormatInt(ts2, 10) + `,"ecdh_pub":"` + pub2 + `","mac":"` + okMAC + `"}`
	w2 := v2Post(t, s, body2)
	if w2.Code == http.StatusOK {
		t.Fatalf("expected rejection while seq lockout active, got 200; body=%s", w2.Body.String())
	}
}
