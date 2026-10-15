package server

import (
	"encoding/json"
	"strings"
	"sync"
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
	stream, _, unsubscribe := broker.subscribe("run-1")
	defer unsubscribe()
	select {
	case event := <-stream:
		t.Fatalf("ephemeral reasoning was replayed: %#v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestAIBrokerCoalescesStalledRetries(t *testing.T) {
	broker := newAIRunBroker()
	stream, _, unsubscribe := broker.subscribe("run-1")
	// Fill the 256 buffer so every publish takes the retry path.
	for i := 0; i < 256; i++ {
		broker.publish("run-1", aiLiveEvent{Sequence: int64(i), Type: "text", Payload: "x"}, false)
	}
	// Flood: with per-event goroutines this parks hundreds of goroutines;
	// with coalescing at most one retry worker exists per channel.
	for i := 0; i < 500; i++ {
		broker.publish("run-1", aiLiveEvent{Sequence: int64(1000 + i), Type: "text", Payload: "y"}, false)
	}
	broker.mu.Lock()
	n := len(broker.retrying)
	broker.mu.Unlock()
	if n > 1 {
		t.Fatalf("expected at most 1 retry worker, got %d", n)
	}
	// Unsubscribe mid-retry must not panic a racing worker (send-on-close).
	unsubscribe()
	select {
	case <-stream:
	case <-time.After(20 * time.Millisecond):
	}
	// Drain remaining worker without hanging the suite.
	time.Sleep(150 * time.Millisecond)
}

// TestAIBrokerUnsubscribeRacesRetriedSend hammers the exact interleaving that
// used to be a data race: publish takes the retry path (buffer full, one
// worker parked in its blocking send) while unsubscribe runs concurrently.
//
// The old design closed the event channel from the unsubscribe closure, so a
// retry worker could be inside `dst <- ev` when the close landed. That was
// caught by a recover(), which silences the panic but not the race. The new
// design never closes the event channel — termination is a separate done
// channel — so this test passes under -race with no recover in sight.
func TestAIBrokerUnsubscribeRacesRetriedSend(t *testing.T) {
	for attempt := 0; attempt < 40; attempt++ {
		broker := newAIRunBroker()
		const runID = "run-race"
		_, _, unsubscribe := broker.subscribe(runID)

		// Saturate the 256-slot buffer so the next publish must take the
		// blocking retry path and park a worker on the send.
		for i := 0; i < 256; i++ {
			broker.publish(runID, aiLiveEvent{Sequence: int64(i), Type: "text", Payload: "x"}, false)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				broker.publish(runID, aiLiveEvent{Sequence: int64(1000 + i), Type: "text", Payload: "y"}, false)
			}
		}()
		go func() {
			defer wg.Done()
			// Repeatedly cycle the subscription so workers spawned by one
			// generation overlap the close of the next.
			for i := 0; i < 10; i++ {
				unsubscribe()
				_, _, unsubscribe = broker.subscribe(runID)
			}
			unsubscribe()
		}()
		wg.Wait()
		// Let any parked retry worker finish against the dead subscription.
		time.Sleep(2 * time.Millisecond)
	}

	// After every unsubscribe the run must be fully deregistered: a leak here
	// would keep retrying workers alive for a run nobody is listening to.
	broker := newAIRunBroker()
	_, done, unsubscribe := broker.subscribe("run-cleanup")
	unsubscribe()
	unsubscribe() // must be idempotent, not a double-close panic
	select {
	case <-done:
	default:
		t.Fatal("unsubscribe must close the done channel")
	}
	broker.mu.Lock()
	leftover := len(broker.subscribers["run-cleanup"])
	broker.mu.Unlock()
	if leftover != 0 {
		t.Fatalf("run kept %d subscribers after unsubscribe", leftover)
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
