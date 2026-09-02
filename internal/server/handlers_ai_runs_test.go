package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAIRunStorageTest(t *testing.T) *Server {
	t.Helper()
	key := strings.Repeat("23", 32)
	forgecrypto.InitLootEncryption(key)
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.AutoMigrate(
		&db.User{}, &db.Implant{}, &db.Task{}, &db.Listener{}, &db.Alert{}, &db.CredentialEntry{},
		&db.AIChatSession{}, &db.AIChatMessage{},
		&db.AIChatRun{}, &db.AIChatRunEvent{}, &db.AIExecutionIntent{},
		&db.AIProviderProfile{}, &db.AIAttachment{}, &db.AIKnowledgeCollection{},
		&db.AIKnowledgeSource{}, &db.AIKnowledgeChunk{},
	); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Crypto.LootKey = key
	return &Server{db: database, cfg: cfg, aiRuns: newAIRunBroker()}
}

func TestAIToolsFilterAgentsAndTasksByTenant(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.Implant{ID: "tenant-a-agent", TenantID: 1, Hostname: "alpha", Status: "online"})
	s.db.Create(&db.Implant{ID: "tenant-b-agent", TenantID: 2, Hostname: "bravo", Status: "online"})
	foreignTask := db.Task{AgentID: "tenant-b-agent", Type: "shell", Command: "secret", Status: "completed"}
	s.db.Create(&foreignTask)

	ctx := &aiReqCtx{Principal: aiPrincipal{UserID: 10, Username: "alice", TenantID: 1, Role: db.RoleAdmin}}
	listed := s.executeToolSwitchCtx(ctx, "list_agents", `{"status":"online"}`)
	if !strings.Contains(listed, "tenant-a-agent") || strings.Contains(listed, "tenant-b-agent") {
		t.Fatalf("tenant leak in list_agents: %s", listed)
	}
	detail := s.executeToolSwitchCtx(ctx, "get_task_detail", `{"task_id":`+strings.TrimSpace(jsonNumber(foreignTask.ID))+`}`)
	if !strings.Contains(detail, "task not found") {
		t.Fatalf("foreign task was visible: %s", detail)
	}
}

func TestAISituationToolIsAvailableAndTenantScoped(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.Implant{ID: "tenant-a-agent", TenantID: 1, Hostname: "alpha", Status: "online"})
	s.db.Create(&db.Implant{ID: "tenant-b-agent", TenantID: 2, Hostname: "bravo", Status: "online"})
	s.db.Create(&db.CredentialEntry{TenantID: 1, Username: "tenant-a-user"})
	s.db.Create(&db.CredentialEntry{TenantID: 2, Username: "tenant-b-user"})

	ctx := &aiReqCtx{Principal: aiPrincipal{UserID: 10, Username: "alice", TenantID: 1, Role: db.RoleAdmin}}
	tools := s.buildToolsForContext(ctx)
	found := false
	for _, tool := range tools {
		if tool.Function.Name == "get_situation" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("get_situation was not exposed to a tenant-scoped operator")
	}

	result := s.executeToolCtx(ctx, "get_situation", `{}`)
	if strings.Contains(result, "tool is not available") {
		t.Fatalf("get_situation was rejected: %s", result)
	}
	var situation aiSituation
	if err := json.Unmarshal([]byte(result), &situation); err != nil {
		t.Fatalf("decode situation: %v (%s)", err, result)
	}
	if situation.AgentsTotal != 1 || situation.AgentsOnline != 1 || situation.Credentials != 1 {
		t.Fatalf("tenant-scoped situation mismatch: %#v", situation)
	}
}

func jsonNumber(value uint) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func TestAIToolPolicyCreatesBoundIntentInsteadOfExecuting(t *testing.T) {
	s := setupAIRunStorageTest(t)
	session := db.AIChatSession{TenantID: 7, OwnerID: 11, Owner: "operator", Title: "approval", WritePolicy: "approval"}
	if err := s.db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	s.db.Create(&db.Implant{ID: "approval-agent", TenantID: 7, Hostname: "host", Status: "online"})
	ctx := &aiReqCtx{
		Principal: aiPrincipal{UserID: 11, Username: "operator", TenantID: 7, Role: db.RoleAdmin},
		SessionID: session.ID, RunID: "run-approval",
	}
	result := s.executeToolCtx(ctx, "execute_command", `{"agent_id":"approval-agent","command":"whoami"}`)
	if !strings.Contains(result, "pending_approval") {
		t.Fatalf("expected pending intent, got %s", result)
	}
	var taskCount int64
	s.db.Model(&db.Task{}).Count(&taskCount)
	if taskCount != 0 {
		t.Fatalf("tool executed before approval")
	}
	var intent db.AIExecutionIntent
	if err := s.db.First(&intent).Error; err != nil {
		t.Fatal(err)
	}
	if intent.OwnerID != 11 || intent.TenantID != 7 || intent.SessionID != session.ID || intent.RunID != "run-approval" || intent.Risk != aiToolRiskWrite {
		t.Fatalf("intent binding mismatch: %#v", intent)
	}
	if !strings.Contains(intent.Arguments, "whoami") {
		t.Fatalf("encrypted arguments did not decrypt for authorized execution")
	}
	var raw db.AIExecutionIntent
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).First(&raw, "id = ?", intent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw.Arguments, "FC2ENC:") {
		t.Fatalf("intent arguments were stored in plaintext: %q", raw.Arguments)
	}
}

func TestAIReasoningIsNotReplayedToNewSubscriber(t *testing.T) {
	broker := newAIRunBroker()
	broker.publish("run-1", aiLiveEvent{Sequence: 3, Type: "reasoning", Payload: "private chain"}, true)
	stream, unsubscribe := broker.subscribe("run-1")
	defer unsubscribe()
	select {
	case event := <-stream:
		t.Fatalf("ephemeral reasoning was replayed: %#v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestAIKnowledgeSearchRejectsCrossTenantCollections(t *testing.T) {
	s := setupAIRunStorageTest(t)
	one := db.AIKnowledgeCollection{TenantID: 1, OwnerID: 1, Name: "one"}
	two := db.AIKnowledgeCollection{TenantID: 2, OwnerID: 2, Name: "two", Shared: true}
	s.db.Create(&one)
	s.db.Create(&two)
	sourceOne := db.AIKnowledgeSource{TenantID: 1, OwnerID: 1, CollectionID: one.ID, Name: "allowed.txt", ChunkCount: 1}
	sourceTwo := db.AIKnowledgeSource{TenantID: 2, OwnerID: 2, CollectionID: two.ID, Name: "foreign.txt", ChunkCount: 1}
	s.db.Create(&sourceOne)
	s.db.Create(&sourceTwo)
	s.db.Create(&db.AIKnowledgeChunk{TenantID: 1, CollectionID: one.ID, SourceID: sourceOne.ID, Content: "alpha operator guide", SearchTokens: s.blindAIKnowledgeTokens("alpha operator guide")})
	s.db.Create(&db.AIKnowledgeChunk{TenantID: 2, CollectionID: two.ID, SourceID: sourceTwo.ID, Content: "alpha foreign secret", SearchTokens: s.blindAIKnowledgeTokens("alpha foreign secret")})
	s.initializeAIKnowledgeIndex()

	results, err := s.searchAIKnowledge(aiPrincipal{UserID: 1, TenantID: 1, Role: db.RoleAdmin}, aiKnowledgeSearchRequest{
		Query: "alpha", CollectionIDs: []uint{one.ID, two.ID}, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Source != "allowed.txt" || strings.Contains(results[0].Content, "foreign") {
		t.Fatalf("cross-tenant knowledge result: %#v", results)
	}
}
