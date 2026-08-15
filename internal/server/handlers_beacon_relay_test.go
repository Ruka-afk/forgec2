package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/pkg/encoding"
)

// TestP2PRelayDeliversEncryptedChildReply proves a parent beacon carrying an
// opaque child envelope (RelayedFrames) receives the server-built encrypted
// reply envelope in RelayedReplies, and that the child can decrypt it with
// its own session — the parent never touches child plaintext.
func TestP2PRelayDeliversEncryptedChildReply(t *testing.T) {
	s, database := v2TestServer(t)

	parentUUID := "11111111-2222-4333-8444-111111111111"
	parent := v3TestAgent(t, s, parentUUID)

	// Parent registers directly with the server.
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if w := v2Post(t, s, parent.registerFrame()); w.Code != http.StatusOK {
		t.Fatalf("parent registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	} else if err := encoding.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("parent register response parse: %v (body=%s)", err, w.Body.String())
	}
	if err := parent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("parent establish session: %v", err)
	}

	// Child registers DIRECTLY too (first contact establishes its session),
	// then beacons exclusively through the parent.
	childUUID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	child := v3TestAgent(t, s, childUUID)
	w := v2Post(t, s, child.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("child registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var childReg struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &childReg); err != nil {
		t.Fatalf("child register response parse: %v", err)
	}
	if err := child.establishFromServerKey(childReg.ECDHPub); err != nil {
		t.Fatalf("child establish session: %v", err)
	}

	// Seed a pending task for the child.
	task := &db.Task{
		AgentID: childUUID,
		Type:    "shell",
		Command: "whoami",
		Status:  "pending",
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Child builds an encrypted beacon request envelope.
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": childUUID,
		"pv":   2,
		"info": map[string]string{"hostname": "CHILD", "username": "u", "ip": "10.0.0.11"},
	})
	childFrame := child.encryptedFrame(inner)
	if childFrame == "" {
		t.Fatalf("child encrypt failed")
	}

	// Parent wraps the child envelope ([]byte JSON field = base64 on the wire)
	// and sends its own encrypted beacon.
	parentInner, _ := json.Marshal(map[string]interface{}{
		"uuid":          parentUUID,
		"pv":            2,
		"info":          map[string]string{"hostname": "PARENT", "username": "p", "ip": "10.0.0.1"},
		"relayed_frames": []map[string]interface{}{
			{"agent_id": childUUID, "envelope": base64.StdEncoding.EncodeToString([]byte(childFrame))},
		},
	})
	parentFrame := parent.encryptedFrame(parentInner)
	if parentFrame == "" {
		t.Fatalf("parent encrypt failed")
	}
	w = v2Post(t, s, parentFrame)
	if w.Code != http.StatusOK {
		t.Fatalf("parent relayed beacon: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// Parse the parent response envelope.
	var parentEnv struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &parentEnv); err != nil {
		t.Fatalf("parent response parse: %v", err)
	}
	parentPlain, err := parent.decryptWithAAD(parentEnv.CipherB64, parent.aad(parent.seq))
	if err != nil {
		t.Fatalf("parent response decrypt: %v", err)
	}
	var parentResp struct {
		RelayedReplies []relayedReply `json:"relayed_replies"`
	}
	if err := encoding.Unmarshal(parentPlain, &parentResp); err != nil {
		t.Fatalf("parent inner response parse: %v (body=%s)", err, string(parentPlain))
	}
	if len(parentResp.RelayedReplies) != 1 {
		t.Fatalf("expected 1 relayed reply, got %d: %s", len(parentResp.RelayedReplies), string(parentPlain))
	}
	rr := parentResp.RelayedReplies[0]
	if rr.AgentID != childUUID {
		t.Fatalf("relayed reply agent mismatch: %s != %s", rr.AgentID, childUUID)
	}

	// The child decrypts the relayed reply with ITS OWN session key and must
	// see the pending task.
	var childEnv struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(rr.Envelope, &childEnv); err != nil {
		t.Fatalf("child reply envelope parse: %v", err)
	}
	childPlain, err := child.decryptWithAAD(childEnv.CipherB64, child.aad(child.seq))
	if err != nil {
		t.Fatalf("child reply decrypt (child's own key): %v", err)
	}
	var childResp beaconResponse
	if err := encoding.Unmarshal(childPlain, &childResp); err != nil {
		t.Fatalf("child inner response parse: %v (body=%s)", err, string(childPlain))
	}
	if len(childResp.Tasks) != 1 || childResp.Tasks[0].ID != task.ID {
		t.Fatalf("expected child task %d delivered, got %+v", task.ID, childResp.Tasks)
	}

	// Parent-child link must be established for topology.
	var childRow db.Implant
	if err := database.Where("id = ?", childUUID).First(&childRow).Error; err != nil {
		t.Fatalf("child row: %v", err)
	}
	if childRow.ParentID != parentUUID {
		t.Fatalf("child parent_id = %q, want %q", childRow.ParentID, parentUUID)
	}
}

// TestP2PRelayRejectsUnlinkedChild proves a parent cannot relay envelopes for
// a child already linked to a different parent (relay hijacking guard).
func TestP2PRelayRejectsUnlinkedChild(t *testing.T) {
	s, database := v2TestServer(t)

	parentUUID := "99999999-8888-4777-8666-555555555555"
	parent := v3TestAgent(t, s, parentUUID)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if w := v2Post(t, s, parent.registerFrame()); w.Code != http.StatusOK {
		t.Fatalf("parent registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	} else if err := encoding.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("parent register response parse: %v", err)
	}
	if err := parent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("parent establish session: %v", err)
	}

	// A child registered directly and linked to a DIFFERENT parent.
	childUUID := "dddddddd-cccc-4eee-8fff-000000000000"
	otherParent := "77777777-6666-4555-8444-999999999999"
	if err := database.Create(&db.Implant{ID: childUUID, ParentID: otherParent}).Error; err != nil {
		t.Fatalf("create child: %v", err)
	}
	child := v3TestAgent(t, s, childUUID)
	var childReg struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if w := v2Post(t, s, child.registerFrame()); w.Code != http.StatusOK {
		t.Fatalf("child registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	} else if err := encoding.Unmarshal(w.Body.Bytes(), &childReg); err != nil {
		t.Fatalf("child register response parse: %v", err)
	}
	if err := child.establishFromServerKey(childReg.ECDHPub); err != nil {
		t.Fatalf("child establish session: %v", err)
	}

	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": childUUID,
		"pv":   2,
		"info": map[string]string{"hostname": "CHILD2", "username": "u", "ip": "10.0.0.12"},
	})
	childFrame := child.encryptedFrame(inner)
	parentInner, _ := json.Marshal(map[string]interface{}{
		"uuid":           parentUUID,
		"pv":             2,
		"info":           map[string]string{"hostname": "P2", "username": "p", "ip": "10.0.0.2"},
		"relayed_frames": []map[string]interface{}{{"agent_id": childUUID, "envelope": base64.StdEncoding.EncodeToString([]byte(childFrame))}},
	})
	w := v2Post(t, s, parent.encryptedFrame(parentInner))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var parentEnv struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &parentEnv); err != nil {
		t.Fatalf("parent response parse: %v", err)
	}
	parentPlain, err := parent.decryptWithAAD(parentEnv.CipherB64, parent.aad(parent.seq))
	if err != nil {
		t.Fatalf("parent response decrypt: %v", err)
	}
	var parentResp beaconResponse
	if err := encoding.Unmarshal(parentPlain, &parentResp); err != nil {
		t.Fatalf("parent inner parse: %v", err)
	}
	if len(parentResp.RelayedReplies) != 0 {
		t.Fatalf("expected 0 relayed replies for unlinked child, got %d: %s",
			len(parentResp.RelayedReplies), string(parentPlain))
	}
	// A child's own encrypted beacon must not be silently accepted as relayed
	// without parent binding; the child remains online via its own beacons.
	var childRow db.Implant
	if err := database.Where("id = ?", childUUID).First(&childRow).Error; err != nil {
		t.Fatalf("child row: %v", err)
	}
	if childRow.ParentID != otherParent {
		t.Fatalf("child parent_id changed to %q, want %q", childRow.ParentID, otherParent)
	}
}
