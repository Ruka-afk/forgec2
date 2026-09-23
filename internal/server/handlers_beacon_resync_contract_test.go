package server

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
)

// applyResync mirrors the agent-side tryResync contract (agent_transport.go):
// snap the local counter to last_seq+1 so the next nextBeaconSeq() yields
// last_seq+2 (same as the Go agent after a MAC-verified resync).
func (a *tcpTestAgent) applyResync(lastSeq uint64) {
	a.seq = lastSeq + 1
}

// invalidate mirrors ecdhSession.invalidate(): rotate the ephemeral keypair
// and drop the session key so the next handshake derives a fresh shared secret
// (forward secrecy on rekey / restart recovery).
func (a *tcpTestAgent) invalidate(t *testing.T) {
	t.Helper()
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("rotate agent key: %v", err)
	}
	a.privateKey = k
	a.sessionKey = nil
}

// postResyncEnvelope posts one beacon frame and classifies the response.
// Returns (isResync, body). A resync is a plaintext envelope with rekey=true
// and no ciphertext; a healthy response carries "c".
func classifyBeaconResp(t *testing.T, s *Server, frame string) (resync bool, body []byte) {
	t.Helper()
	w := v2Post(t, s, frame)
	if w.Code != http.StatusOK {
		t.Fatalf("beacon: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var probe struct {
		CipherB64 string `json:"c"`
		Rekey     bool   `json:"rekey"`
		ECDHPub   string `json:"ecdh_pub"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &probe); err != nil {
		t.Fatalf("resp parse: %v (body=%s)", err, w.Body.String())
	}
	if probe.CipherB64 == "" && probe.Rekey && probe.ECDHPub != "" {
		return true, w.Body.Bytes()
	}
	return false, w.Body.Bytes()
}

type resyncContract struct {
	Seq     uint64 `json:"seq"`
	LastSeq uint64 `json:"last_seq"`
	Rekey   bool   `json:"rekey"`
	ECDHPub string `json:"ecdh_pub"`
	Mac     string `json:"mac"`
}

// TestResyncDropSessionRecoveryLoop pins the full recovery protocol contract
// on HTTP (shared decode/resync path for every transport):
//
//  1. register → establish → encrypted beacon works (baseline task delivery)
//  2. server loses the ECDH session (restart/sweep) — RemoveSession
//  3. agent still encrypts under the dead key → server cannot decrypt
//  4. server MUST answer a MAC-signed resync with rekey=true, ecdh_pub, last_seq
//  5. agent applies tryResync (snap seq), rotates ephemeral key, handshakes
//  6. subsequent encrypted beacons MUST succeed — no second resync (ECDH fail=0)
//  7. tasks continue to be delivered after recovery (operational continuity)
//
// This is the invariant that stopped the C-implant infinite ECDH-fail loop:
// exactly one resync per session loss, then a clean re-handshake.
func TestResyncDropSessionRecoveryLoop(t *testing.T) {
	crypto.InitLootEncryption(testStorageKeyHex)
	s, database := v2TestServer(t)

	agentUUID := "55555555-6666-4777-8888-999999999999"
	agent := v3TestAgent(t, s, agentUUID)

	// ── Phase 1: baseline — register, establish, one successful encrypted beacon.
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
	if err := encoding.Unmarshal(w.Body.Bytes(), &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("register parse: %v (body=%s)", err, w.Body.String())
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register MAC mismatch: %s", w.Body.String())
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish: %v", err)
	}

	taskA := &db.Task{AgentID: agentUUID, Type: "shell", Command: "echo A", Status: "pending"}
	if err := database.Create(taskA).Error; err != nil {
		t.Fatalf("seed task A: %v", err)
	}
	innerA, _ := json.Marshal(map[string]interface{}{
		"uuid": agentUUID, "pv": 2,
		"info": map[string]string{"hostname": "BASE", "username": "u", "ip": "10.0.0.1"},
	})
	resync, respBody := classifyBeaconResp(t, s, agent.encryptedFrame(innerA))
	if resync {
		t.Fatalf("baseline encrypted beacon must not resync: %s", respBody)
	}
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(respBody, &encResp); err != nil || encResp.CipherB64 == "" {
		t.Fatalf("baseline expected ciphertext: %s (err=%v)", respBody, err)
	}
	plain, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
	if err != nil {
		t.Fatalf("baseline decrypt: %v", err)
	}
	var innerResp beaconResponse
	if err := encoding.Unmarshal(plain, &innerResp); err != nil {
		t.Fatalf("baseline inner parse: %v", err)
	}
	if len(innerResp.Tasks) != 1 || innerResp.Tasks[0].ID != taskA.ID {
		t.Fatalf("baseline task delivery: got %+v, want task %d", innerResp.Tasks, taskA.ID)
	}

	// ── Phase 2: server loses the session (restart / sweep).
	s.sessionManager.RemoveSession(agentUUID)

	// ── Phase 3: agent still holds the dead key; server cannot decrypt.
	// Exactly one resync, with the full contract payload.
	resyncCount := 0
	lostFrame, _ := json.Marshal(map[string]interface{}{
		"uuid": agentUUID, "seq": agent.nextSeq(), "ts": time.Now().Unix(),
		"c": "Z3JhYmJlLWNpcGhlcnRleHQ", // garbage: cannot decrypt
	})
	resync, respBody = classifyBeaconResp(t, s, string(lostFrame))
	if !resync {
		t.Fatalf("lost-session frame must get a resync, got: %s", respBody)
	}
	resyncCount++
	var rs resyncContract
	if err := encoding.Unmarshal(respBody, &rs); err != nil {
		t.Fatalf("resync parse: %v (body=%s)", err, respBody)
	}
	if !rs.Rekey {
		t.Fatalf("resync must set rekey=true: %s", respBody)
	}
	if rs.ECDHPub == "" {
		t.Fatalf("resync must carry ecdh_pub: %s", respBody)
	}
	if !agent.verifyResponseMAC(rs.Seq, rs.ECDHPub, rs.Mac) {
		t.Fatalf("resync MAC mismatch: %s", respBody)
	}
	// last_seq must reflect the last ACCEPTED frame (baseline encrypted seq),
	// not the rejected garbage seq — the agent snaps forward from this.
	if rs.LastSeq == 0 {
		t.Fatalf("resync must carry last_seq for agent snap: %s", respBody)
	}
	var dbLastSeq uint64
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentUUID).Pluck("last_seq", &dbLastSeq).Error; err != nil {
		t.Fatalf("read last_seq: %v", err)
	}
	if rs.LastSeq != dbLastSeq {
		t.Fatalf("resync last_seq=%d, db last_seq=%d (must match)", rs.LastSeq, dbLastSeq)
	}

	// ── Phase 4: agent contract — snap seq, rotate ephemeral, handshake.
	agent.applyResync(rs.LastSeq)
	agent.invalidate(t)
	w = v2Post(t, s, agent.handshakeFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("handshake after resync: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var hsResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
		Rekey   bool   `json:"rekey"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &hsResp); err != nil {
		t.Fatalf("handshake parse: %v (body=%s)", err, w.Body.String())
	}
	if hsResp.RegOK {
		t.Fatalf("handshake must not re-register: %s", w.Body.String())
	}
	if hsResp.Rekey {
		t.Fatalf("handshake response must not itself request another rekey: %s", w.Body.String())
	}
	if hsResp.ECDHPub == "" {
		t.Fatalf("handshake must return ecdh_pub: %s", w.Body.String())
	}
	if !agent.verifyResponseMAC(hsResp.Seq, hsResp.ECDHPub, hsResp.Mac) {
		t.Fatalf("handshake MAC mismatch: %s", w.Body.String())
	}
	if err := agent.establishFromServerKey(hsResp.ECDHPub); err != nil {
		t.Fatalf("re-establish: %v", err)
	}

	// ── Phase 5: recovery loop must terminate — N encrypted beacons, zero resyncs.
	taskB := &db.Task{AgentID: agentUUID, Type: "shell", Command: "echo B", Status: "pending"}
	if err := database.Create(taskB).Error; err != nil {
		t.Fatalf("seed task B: %v", err)
	}
	const postRecoveryBeacons = 3
	taskDelivered := false
	for i := 0; i < postRecoveryBeacons; i++ {
		inner, _ := json.Marshal(map[string]interface{}{
			"uuid": agentUUID, "pv": 2,
			"info": map[string]string{"hostname": "RECOVERED", "username": "u", "ip": "10.0.0.2"},
		})
		frame := agent.encryptedFrame(inner)
		if frame == "" {
			t.Fatalf("encrypt failed on post-recovery beacon %d", i)
		}
		resync, respBody = classifyBeaconResp(t, s, frame)
		if resync {
			t.Fatalf("post-recovery beacon %d got a SECOND resync (ECDH fail loop): %s", i, respBody)
		}
		var er struct {
			CipherB64 string `json:"c"`
		}
		if err := encoding.Unmarshal(respBody, &er); err != nil || er.CipherB64 == "" {
			t.Fatalf("post-recovery beacon %d expected ciphertext: %s (err=%v)", i, respBody, err)
		}
		plain, err := agent.decryptWithAAD(er.CipherB64, agent.aad(agent.seq))
		if err != nil {
			t.Fatalf("post-recovery beacon %d decrypt: %v", i, err)
		}
		var ir beaconResponse
		if err := encoding.Unmarshal(plain, &ir); err != nil {
			t.Fatalf("post-recovery beacon %d inner parse: %v", i, err)
		}
		for _, tk := range ir.Tasks {
			if tk.ID == taskB.ID {
				taskDelivered = true
			}
		}
	}
	if resyncCount != 1 {
		t.Fatalf("expected exactly 1 resync for the whole recovery, got %d", resyncCount)
	}
	if !taskDelivered {
		t.Fatalf("task B must be delivered on a post-recovery beacon")
	}
}

// TestResyncDropSessionRecoveryLoopDNS pins the identical contract on the
// DNS listener path (makeBeaconHandler → handleListenerBeacon). This is the
// transport where the C-implant infinite ECDH-fail loop was observed.
func TestResyncDropSessionRecoveryLoopDNS(t *testing.T) {
	crypto.InitLootEncryption(testStorageKeyHex)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	h := s.makeBeaconHandler("dns")

	agentUUID := "66666666-7777-4888-9999-aaaaaaaaaaaa"
	agent := v3TestAgent(t, s, agentUUID)

	// Register + establish.
	resp := h(agentUUID, []byte(agent.registerFrame()))
	if len(resp) == 0 {
		t.Fatal("dns register: empty response")
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(resp, &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("dns register parse: %v (body=%s)", err, resp)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish: %v", err)
	}

	// Baseline encrypted beacon.
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agentUUID, "pv": 2,
		"info": map[string]string{"hostname": "DNS-BASE", "username": "u", "ip": "10.0.0.3"},
	})
	resp = h(agentUUID, []byte(agent.encryptedFrame(inner)))
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(resp, &encResp); err != nil || encResp.CipherB64 == "" {
		t.Fatalf("dns baseline expected ciphertext: %s (err=%v)", resp, err)
	}
	if _, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq)); err != nil {
		t.Fatalf("dns baseline decrypt: %v", err)
	}

	// Lose the session.
	s.sessionManager.RemoveSession(agentUUID)

	// Encrypted (dead key) → must get exactly one resync with last_seq.
	lostFrame, _ := json.Marshal(map[string]interface{}{
		"uuid": agentUUID, "seq": agent.nextSeq(), "ts": time.Now().Unix(),
		"c": "Z3JhYmJl",
	})
	resp = h(agentUUID, []byte(lostFrame))
	var rs resyncContract
	if err := encoding.Unmarshal(resp, &rs); err != nil {
		t.Fatalf("dns resync parse: %v (body=%s)", err, resp)
	}
	if !rs.Rekey || rs.ECDHPub == "" || rs.LastSeq == 0 {
		t.Fatalf("dns resync contract incomplete (rekey=%v ecdh=%q last_seq=%d): %s",
			rs.Rekey, rs.ECDHPub, rs.LastSeq, resp)
	}
	if !agent.verifyResponseMAC(rs.Seq, rs.ECDHPub, rs.Mac) {
		t.Fatalf("dns resync MAC mismatch: %s", resp)
	}

	// Apply contract: snap, rotate, handshake.
	agent.applyResync(rs.LastSeq)
	agent.invalidate(t)
	resp = h(agentUUID, []byte(agent.handshakeFrame()))
	var hsResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(resp, &hsResp); err != nil || hsResp.ECDHPub == "" {
		t.Fatalf("dns handshake: %v (body=%s)", err, resp)
	}
	if hsResp.RegOK {
		t.Fatalf("dns handshake must not re-register: %s", resp)
	}
	if !agent.verifyResponseMAC(hsResp.Seq, hsResp.ECDHPub, hsResp.Mac) {
		t.Fatalf("dns handshake MAC mismatch: %s", resp)
	}
	if err := agent.establishFromServerKey(hsResp.ECDHPub); err != nil {
		t.Fatalf("dns re-establish: %v", err)
	}

	// Three post-recovery beacons: all ciphertext, zero resyncs.
	for i := 0; i < 3; i++ {
		inner, _ := json.Marshal(map[string]interface{}{
			"uuid": agentUUID, "pv": 2,
			"info": map[string]string{"hostname": "DNS-OK", "username": "u", "ip": "10.0.0.3"},
		})
		frame := agent.encryptedFrame(inner)
		if frame == "" {
			t.Fatalf("dns encrypt failed at %d", i)
		}
		resp = h(agentUUID, []byte(frame))
		var er struct {
			CipherB64 string `json:"c"`
			Rekey     bool   `json:"rekey"`
			ECDHPub   string `json:"ecdh_pub"`
		}
		if err := encoding.Unmarshal(resp, &er); err != nil {
			t.Fatalf("dns post-recovery %d parse: %v (body=%s)", i, err, resp)
		}
		if er.CipherB64 == "" {
			if er.Rekey && er.ECDHPub != "" {
				t.Fatalf("dns post-recovery %d got a SECOND resync (ECDH fail loop): %s", i, resp)
			}
			t.Fatalf("dns post-recovery %d empty response: %s", i, resp)
		}
		if _, err := agent.decryptWithAAD(er.CipherB64, agent.aad(agent.seq)); err != nil {
			t.Fatalf("dns post-recovery %d decrypt: %v", i, err)
		}
	}
}
