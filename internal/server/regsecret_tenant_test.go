package server

import (
	"encoding/base64"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
)

// v3 registration secrets carry the minting tenant: the implant an operator
// builds lands in the operator's tenant when it first registers, instead of
// the bootstrap default (legacy 0 rows keep the old default-tenant behavior,
// so pre-feature binaries keep working after an upgrade).

// runV3Registration performs the full TCP v3 registration dance for one minted
// secret and returns the implant row it produced.
func runV3Registration(t *testing.T, s *Server, secretB64, secretID, agentUUID string) db.Implant {
	t.Helper()
	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	conn, done := tcpFrameConn(t, s)
	defer done()

	agent := newTCPTestAgent(t, agentUUID).
		withRawRegKey(secret).
		withSecretID(secretID)

	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	respFrame := tcpReadFrame(t, conn)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &regResp); err != nil {
		t.Fatalf("register response parse failed: %v (body=%s)", err, respFrame)
	}
	if !regResp.RegOK {
		t.Fatalf("v3 registration must succeed, got %s", respFrame)
	}
	var implant db.Implant
	if err := s.db.Where("id = ?", agentUUID).First(&implant).Error; err != nil {
		t.Fatalf("implant row missing: %v", err)
	}
	return implant
}

func TestRegSecretTenantScopesRegisteredAgent(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	team := db.Tenant{Name: "team-two"}
	if err := database.Create(&team).Error; err != nil {
		t.Fatalf("create second tenant: %v", err)
	}

	id, secretB64, err := s.createRegSecret(team.ID)
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	implant := runV3Registration(t, s, secretB64, id,
		"aaaaaaaa-bbbb-4333-8444-cccccccccccc")
	if implant.TenantID != team.ID {
		t.Fatalf("team-minted implant landed in tenant %d, want %d", implant.TenantID, team.ID)
	}
	var secretRow db.RegSecret
	if err := database.Where("id = ?", id).First(&secretRow).Error; err != nil {
		t.Fatalf("secret row missing: %v", err)
	}
	if secretRow.TenantID != team.ID {
		t.Fatalf("secret tenant wrong: %d, want %d", secretRow.TenantID, team.ID)
	}

	// Legacy path unchanged: a tenant-0 (pre-feature / CLI) secret still lands
	// its implant in the bootstrap default tenant.
	legacyID, legacySecretB64, err := s.createRegSecret(0)
	if err != nil {
		t.Fatalf("createRegSecret(legacy): %v", err)
	}
	legacy := runV3Registration(t, s, legacySecretB64, legacyID,
		"aaaaaaaa-bbbb-4333-8444-dddddddddddd")
	if legacy.TenantID != s.defaultTenantID() {
		t.Fatalf("legacy implant tenant %d, want default %d", legacy.TenantID, s.defaultTenantID())
	}
}

func TestRelayChildInheritsParentTenant(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := &Server{db: database}

	team := db.Tenant{Name: "team-two"}
	if err := database.Create(&team).Error; err != nil {
		t.Fatalf("create second tenant: %v", err)
	}
	parent := db.Implant{
		ID:       "aaaaaaaa-bbbb-4333-8444-eeeeeeeeeeee",
		TenantID: team.ID,
		Status:   "online",
	}
	if err := database.Create(&parent).Error; err != nil {
		t.Fatalf("create parent: %v", err)
	}

	childID := "aaaaaaaa-bbbb-4333-8444-ffffffffffff"
	if !s.bindRelayChildToParent(childID, parent.ID) {
		t.Fatalf("relay child bind failed")
	}
	var child db.Implant
	if err := database.Where("id = ?", childID).First(&child).Error; err != nil {
		t.Fatalf("child row missing: %v", err)
	}
	if child.TenantID != team.ID {
		t.Fatalf("relay child tenant %d, want parent's %d", child.TenantID, team.ID)
	}
	if child.ParentID != parent.ID {
		t.Fatalf("relay child parent %q, want %q", child.ParentID, parent.ID)
	}
}
