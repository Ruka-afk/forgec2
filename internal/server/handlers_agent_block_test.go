package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// blockedCheckinRequest builds a minimal valid v3 beacon for the given UUID.
func blockedCheckinRequest(uuid string) beaconRequest {
	return beaconRequest{
		UUID:            uuid,
		ProtocolVersion: 3,
		Info: map[string]string{
			"hostname": "BLOCKED-HOST",
			"username": "alice",
			"os":       "windows",
			"arch":     "amd64",
			"ip":       "10.0.0.9",
		},
	}
}

// TestBlockedAgentCheckInRefused pins the force-offline contract: a blocked
// implant's check-in is refused (no agent returned), its status stays pinned
// offline, last_seen is NOT refreshed, and the block survives repeated
// beacons (unlike soft-delete, which is restored on re-beacon).
func TestBlockedAgentCheckInRefused(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initV3BeaconServer(t, database, tenantVisibilityMasterHex)

	uuid := "blocked-agent-0001"
	if _, ok := s.processAgentRegistration(blockedCheckinRequest(uuid), "203.0.113.7", time.Now()); !ok {
		t.Fatal("initial registration failed")
	}
	if err := database.Model(&db.Implant{}).Where("id = ?", uuid).
		Updates(map[string]interface{}{"blocked": true, "blocked_reason": "operator ordered"}).Error; err != nil {
		t.Fatalf("block agent: %v", err)
	}

	// Let the agent go "online" first so we can observe the pin-back.
	if err := database.Model(&db.Implant{}).Where("id = ?", uuid).Update("status", "online").Error; err != nil {
		t.Fatalf("set online: %v", err)
	}

	before, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	if err := database.Model(&db.Implant{}).Where("id = ?", uuid).Update("last_seen", before).Error; err != nil {
		t.Fatalf("backdate last_seen: %v", err)
	}

	agent, ok := s.processAgentRegistration(blockedCheckinRequest(uuid), "203.0.113.7", time.Now())
	if ok || agent.ID != "" {
		t.Fatalf("blocked check-in must be refused, got agent=%q ok=%v", agent.ID, ok)
	}

	var implant db.Implant
	if err := database.Unscoped().Where("id = ?", uuid).First(&implant).Error; err != nil {
		t.Fatalf("reload implant: %v", err)
	}
	if !implant.Blocked || implant.BlockedReason != "operator ordered" {
		t.Fatalf("block state mutated by check-in: blocked=%v reason=%q", implant.Blocked, implant.BlockedReason)
	}
	if implant.Status != "offline" {
		t.Fatalf("status = %q, want offline (pinned on refused check-in)", implant.Status)
	}
	if !implant.LastSeen.Equal(before) {
		t.Fatalf("last_seen refreshed for blocked agent: %v", implant.LastSeen)
	}
}

// TestUnblockedAgentResumesService verifies that clearing the flag restores
// normal check-in handling through the regular path.
func TestUnblockedAgentResumesService(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initV3BeaconServer(t, database, tenantVisibilityMasterHex)

	uuid := "blocked-agent-0002"
	if _, ok := s.processAgentRegistration(blockedCheckinRequest(uuid), "203.0.113.7", time.Now()); !ok {
		t.Fatal("initial registration failed")
	}
	if err := database.Model(&db.Implant{}).Where("id = ?", uuid).
		Update("blocked", true).Error; err != nil {
		t.Fatalf("block: %v", err)
	}
	if _, ok := s.processAgentRegistration(blockedCheckinRequest(uuid), "203.0.113.7", time.Now()); ok {
		t.Fatal("blocked check-in was not refused")
	}
	if err := database.Model(&db.Implant{}).Where("id = ?", uuid).
		Updates(map[string]interface{}{"blocked": false, "blocked_reason": ""}).Error; err != nil {
		t.Fatalf("unblock: %v", err)
	}
	// The second return value of processAgentRegistration is isNewAgent, not
	// success — a returning agent legitimately reports false. Refusal is
	// signalled by an empty agent ID only.
	agent, _ := s.processAgentRegistration(blockedCheckinRequest(uuid), "203.0.113.7", time.Now())
	if agent.ID != uuid {
		t.Fatalf("unblocked agent check-in refused: agent=%q", agent.ID)
	}
	var implant db.Implant
	if err := database.Where("id = ?", uuid).First(&implant).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if implant.Status != "online" {
		t.Fatalf("status = %q after unblock, want online", implant.Status)
	}
}

func blockTestGinContext(t *testing.T, method, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	c.Request = r
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "admin")
	c.Params = gin.Params{{Key: "id", Value: "blocked-agent-0003"}}
	return c, w
}

// TestBlockUnblockEndpoints exercises the operator-facing endpoints end to
// end against a real test DB, including the double-unblock conflict.
func TestBlockUnblockEndpoints(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initV3BeaconServer(t, database, tenantVisibilityMasterHex)

	uuid := "blocked-agent-0003"
	if _, ok := s.processAgentRegistration(blockedCheckinRequest(uuid), "203.0.113.7", time.Now()); !ok {
		t.Fatal("initial registration failed")
	}

	// Block with reason.
	c, w := blockTestGinContext(t, http.MethodPost, "/api/agents/"+uuid+"/block",
		[]byte(`{"reason":"compromised channel"}`))
	s.handleBlockAgent(c)
	if w.Code != http.StatusOK {
		t.Fatalf("block: status %d body=%s", w.Code, w.Body.String())
	}
	var implant db.Implant
	if err := database.Where("id = ?", uuid).First(&implant).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !implant.Blocked || implant.BlockedReason != "compromised channel" || implant.Status != "offline" {
		t.Fatalf("after block: blocked=%v reason=%q status=%q", implant.Blocked, implant.BlockedReason, implant.Status)
	}

	// Unblock.
	c, w = blockTestGinContext(t, http.MethodDelete, "/api/agents/"+uuid+"/block", nil)
	s.handleUnblockAgent(c)
	if w.Code != http.StatusOK {
		t.Fatalf("unblock: status %d body=%s", w.Code, w.Body.String())
	}

	// Double unblock conflicts.
	c, w = blockTestGinContext(t, http.MethodDelete, "/api/agents/"+uuid+"/block", nil)
	s.handleUnblockAgent(c)
	if w.Code != http.StatusConflict {
		t.Fatalf("double unblock: status %d, want 409", w.Code)
	}
}
