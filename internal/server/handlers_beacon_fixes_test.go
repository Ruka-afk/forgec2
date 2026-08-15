package server

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/gin-gonic/gin"
)

// extC2Post sends a JSON body to the extc2 receive endpoint with the shared
// token set on the server config.
func extC2Post(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/extc2/v1/receive", s.handleExtC2Receive)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/extc2/v1/receive", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ExtC2-Token", s.cfg.RateLimit.ExtC2.APIToken)
	r.ServeHTTP(w, req)
	return w
}

// TestBadCiphertextDoesNotAdvanceReplayWindow (S2): a ciphertext frame that
// fails AEAD authentication must NOT burn the replay window. An attacker who
// can guess a UUID but not the session key must not be able to lock out the
// real agent by flooding garbage frames.
func TestBadCiphertextDoesNotAdvanceReplayWindow(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	conn, done := tcpFrameConn(t, s)
	defer done()

	agent := v3TestAgent(t, s, "aaaaaaaa-bbbb-4333-8444-cccccccccccc")

	// Register (seq 1) and establish the ECDH session.
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	respFrame := tcpReadFrame(t, conn)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("registration failed: %v (body=%s)", err, respFrame)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish session: %v", err)
	}

	// Attacker frame: seq 2 with ciphertext encrypted under the WRONG AAD.
	// The server decrypts with the correct AAD (uuid||seq) -> GCM tag check
	// fails -> frame rejected. If the server advanced the replay window before
	// decryption (the old behaviour), seq 2 would be burned forever.
	badSeq := agent.nextSeq()
	badCipher, err := agent.encryptWithAAD([]byte(`{"uuid":"`+agent.uuid+`","pv":2}`), []byte("attacker-controlled-aad"))
	if err != nil || badCipher == "" {
		t.Fatalf("attacker encrypt failed: %v", err)
	}
	badFrame, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"seq":  badSeq,
		"ts":   time.Now().Unix(),
		"c":    badCipher,
	})
	tcpWriteFrame(t, conn, badFrame)

	// The server must close the connection for the undecryptable frame.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for bad ciphertext, got frame length %d", msgLen)
	}

	// The REAL agent reconnects and sends a valid frame with the SAME seq 2.
	// With the fix the window was never advanced, so this must be accepted.
	conn.Close()
	conn, done = tcpFrameConn(t, s)
	defer done()
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"pv":   2,
		"info": map[string]string{"hostname": "REPLAY-SAFE", "username": "u", "ip": "10.0.0.8"},
	})
	validCipher, err := agent.encryptWithAAD(inner, agent.aad(badSeq))
	if err != nil {
		t.Fatalf("valid encrypt failed: %v", err)
	}
	validFrame, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"seq":  badSeq,
		"ts":   time.Now().Unix(),
		"c":    validCipher,
	})
	tcpWriteFrame(t, conn, validFrame)

	respFrame = tcpReadFrame(t, conn)
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(respFrame, &encResp); err != nil || encResp.CipherB64 == "" {
		t.Fatalf("expected encrypted response for valid seq=%d frame, got %s (err=%v)", badSeq, respFrame, err)
	}
	if _, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(badSeq)); err != nil {
		t.Fatalf("decrypt valid response: %v", err)
	}

	// A replay of the valid seq=2 frame must still be rejected.
	tcpWriteFrame(t, conn, validFrame)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for replayed seq=%d, got length %d", badSeq, msgLen)
	}
}

// TestExtC2ReceiveEndToEnd (S1): the extc2 receive endpoint must run the full
// v2 envelope authentication — plaintext/garbage frames are rejected, a real
// registration + encrypted exchange succeeds, and the response is encrypted.
func TestExtC2ReceiveEndToEnd(t *testing.T) {
	s, database := v2TestServer(t)
	s.configMu.Lock()
	s.cfg.RateLimit.ExtC2.APIToken = "extc2-test-token"
	s.configMu.Unlock()

	agentUUID := "33333333-4444-4333-8444-555555555555"
	agent := v3TestAgent(t, s, agentUUID)

	wrap := func(raw string) string {
		b, _ := json.Marshal(extC2ReceiveRequest{BeaconID: "attacker-chosen-id", Raw: base64.StdEncoding.EncodeToString([]byte(raw))})
		return string(b)
	}

	t.Run("missing token rejected", func(t *testing.T) {
		r := gin.New()
		r.POST("/extc2/v1/receive", s.handleExtC2Receive)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/extc2/v1/receive", strings.NewReader(wrap(agent.registerFrame())))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 without token, got %d", w.Code)
		}
	})

	t.Run("plaintext beacon request rejected", func(t *testing.T) {
		// Old behaviour: an opaque plaintext beaconRequest was accepted and
		// processed under the attacker-chosen beacon_id.
		w := extC2Post(t, s, wrap(`{"uuid":"`+agentUUID+`","pv":1,"info":{"hostname":"SPOOF","username":"u","ip":"1.2.3.4"}}`))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for plaintext frame, got %d; body=%s", w.Code, w.Body.String())
		}
		var count int64
		database.Model(&db.Implant{}).Where("id = ?", agentUUID).Count(&count)
		if count != 0 {
			t.Fatalf("unauthenticated extc2 frame must not create implants, count=%d", count)
		}
	})

	t.Run("garbage raw rejected", func(t *testing.T) {
		w := extC2Post(t, s, wrap("not-an-envelope-at-all"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for garbage raw, got %d; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("registration via extc2", func(t *testing.T) {
		w := extC2Post(t, s, wrap(agent.registerFrame()))
		if w.Code != http.StatusOK {
			t.Fatalf("registration via extc2: expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		var resp struct {
			Success bool   `json:"success"`
			Data    string `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Success {
			t.Fatalf("extc2 response parse: %v (body=%s)", err, w.Body.String())
		}
		inner, err := base64.StdEncoding.DecodeString(resp.Data)
		if err != nil {
			t.Fatalf("response data base64: %v", err)
		}
		var regResp struct {
			Seq     uint64 `json:"seq"`
			RegOK   bool   `json:"reg_ok"`
			ECDHPub string `json:"ecdh_pub"`
			Mac     string `json:"mac"`
		}
		if err := encoding.Unmarshal(inner, &regResp); err != nil || !regResp.RegOK {
			t.Fatalf("registration response invalid: %v (inner=%s)", err, inner)
		}
		if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
			t.Fatalf("registration response MAC mismatch")
		}
		if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
			t.Fatalf("establish session: %v", err)
		}
		var implant db.Implant
		if err := database.Where("id = ?", agentUUID).First(&implant).Error; err != nil || !implant.Registered {
			t.Fatalf("agent not registered via extc2: %v", err)
		}
	})

	t.Run("encrypted beacon via extc2", func(t *testing.T) {
		inner, _ := json.Marshal(map[string]interface{}{
			"uuid": agentUUID,
			"pv":   2,
			"info": map[string]string{"hostname": "EXT-C2", "username": "u", "ip": "10.0.0.8"},
		})
		w := extC2Post(t, s, wrap(agent.encryptedFrame(inner)))
		if w.Code != http.StatusOK {
			t.Fatalf("encrypted beacon via extc2: expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		var resp struct {
			Success bool   `json:"success"`
			Data    string `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || !resp.Success {
			t.Fatalf("extc2 response parse: %v (body=%s)", err, w.Body.String())
		}
		innerResp, err := base64.StdEncoding.DecodeString(resp.Data)
		if err != nil {
			t.Fatalf("response data base64: %v", err)
		}
		var encResp struct {
			CipherB64 string `json:"c"`
		}
		if err := encoding.Unmarshal(innerResp, &encResp); err != nil || encResp.CipherB64 == "" {
			t.Fatalf("extc2 response envelope invalid: %v (inner=%s)", err, innerResp)
		}
		plaintext, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
		if err != nil {
			t.Fatalf("decrypt extc2 response: %v", err)
		}
		var br beaconResponse
		if err := encoding.Unmarshal(plaintext, &br); err != nil {
			t.Fatalf("extc2 response is not a beaconResponse: %v", err)
		}
	})
}

// TestBadRegHMACCreatesNoImplantRow (S6): a registration frame with a bad
// reg_hmac must be rejected BEFORE the implant row is created — an
// unauthenticated actor must never be able to write rows.
func TestBadRegHMACCreatesNoImplantRow(t *testing.T) {
	s, database := v2TestServer(t)
	agentUUID := "55555555-6666-4333-8444-777777777777"

	wrongAgent := newTCPTestAgent(t, agentUUID).withRegKey(strings.Repeat("ff", 32))
	w := v2Post(t, s, wrongAgent.registerFrame())
	if w.Code == http.StatusOK {
		t.Fatalf("expected rejection for bad reg_hmac, got 200; body=%s", w.Body.String())
	}
	var count int64
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).Count(&count)
	if count != 0 {
		t.Fatalf("bad reg_hmac must not create an implant row, count=%d", count)
	}

	// The genuine agent registers fine afterwards with the same UUID.
	genuine := v3TestAgent(t, s, agentUUID)
	w = v2Post(t, s, genuine.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("genuine registration after rejected attempt must succeed, got %d; body=%s", w.Code, w.Body.String())
	}
}
