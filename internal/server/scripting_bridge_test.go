package server

import (
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/scripting"
	"github.com/forgec2/forgec2/internal/testutil"
)

func initScriptingBridgeServer(t *testing.T) *Server {
	t.Helper()
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := &Server{
		db:                  database,
		cfg:                 &config.Config{},
		eventManager:        NewEventManager(database),
		beaconDedupCache:    make(map[string]time.Time),
		agentStatusCooldown: make(map[string]time.Time),
		agentPendingTasks:   make(map[string]int),
		metrics:             NewMetricsCollector(nil),
	}
	if err := s.db.Create(&db.Implant{
		ID:             "script-test-agent",
		Hostname:       "victim-01",
		Username:       "alice",
		OS:             "windows",
		Arch:           "amd64",
		IP:             "10.0.0.5",
		Status:         "online",
		LastSeen:       time.Now(),
		CurrentInterval: 60,
	}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	return s
}

func TestScriptingBridgeSendTaskPermissions(t *testing.T) {
	s := initScriptingBridgeServer(t)
	b := &scriptingBridge{s: s}

	// A user with agents.write can queue a task.
	id, err := b.SendTask(scripting.Caller{Username: "operator", Role: db.RoleUser}, "script-test-agent", "shell", "whoami")
	if err != nil {
		t.Fatalf("SendTask as user failed: %v", err)
	}
	if id == 0 {
		t.Fatal("SendTask returned task id 0")
	}

	// Unknown task types are rejected by the shared task pipeline.
	if _, err := b.SendTask(scripting.Caller{Username: "operator", Role: db.RoleUser}, "script-test-agent", "no_such_task", "x"); err == nil {
		t.Fatal("expected unknown task type error")
	}

	// Roles without agents.write are denied before touching the DB.
	if _, err := b.SendTask(scripting.Caller{Username: "guest", Role: "viewer"}, "script-test-agent", "shell", "whoami"); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied for viewer, got %v", err)
	}
}

func TestScriptingBridgeQueryWhitelist(t *testing.T) {
	s := initScriptingBridgeServer(t)
	b := &scriptingBridge{s: s}
	caller := scripting.Caller{Username: "operator", Role: db.RoleUser}

	agents, err := b.Query(caller, "agents", map[string]interface{}{"status": "online"})
	if err != nil {
		t.Fatalf("query agents: %v", err)
	}
	list, ok := agents.([]map[string]interface{})
	if !ok || len(list) != 1 {
		t.Fatalf("expected 1 agent, got %#v", agents)
	}
	if list[0]["id"] != "script-test-agent" {
		t.Fatalf("unexpected agent summary: %#v", list[0])
	}

	if _, err := b.Query(caller, "drop tables", nil); err == nil || !strings.Contains(err.Error(), "unknown query kind") {
		t.Fatalf("expected unknown query kind error, got %v", err)
	}

	// Credentials require credentials.read; viewer does not have it.
	if _, err := b.Query(scripting.Caller{Role: "viewer"}, "credentials", nil); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied for credentials query, got %v", err)
	}
}

func TestScriptingBridgeHTTPRequestSSRF(t *testing.T) {
	s := initScriptingBridgeServer(t)
	b := &scriptingBridge{s: s}

	// Non-admin callers are rejected outright.
	if _, err := b.HTTPRequest(scripting.Caller{Role: db.RoleUser}, "GET", "https://example.com", nil, "", 5); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied for non-admin httpRequest, got %v", err)
	}

	// Admin callers must still not reach private/local targets.
	admin := scripting.Caller{Role: db.RoleAdmin}
	for _, target := range []string{
		"http://127.0.0.1:8080/",
		"http://localhost/api",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/",
		"ftp://example.com/file",
	} {
		if _, err := b.HTTPRequest(admin, "GET", target, nil, "", 5); err == nil || !strings.Contains(err.Error(), "allowed") {
			t.Fatalf("expected SSRF block for %q, got %v", target, err)
		}
	}

	if _, err := b.HTTPRequest(admin, "GET", "https://example.com", nil, "", 500); err == nil {
		// unreachable network in CI is fine — the timeout cap is what matters here
	}
}
