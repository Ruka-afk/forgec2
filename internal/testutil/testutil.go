package testutil

import (
	"encoding/json"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// SQLite ":memory:" is per-connection; a single open connection keeps the
	// schema and rows shared across all queries in the test (and avoids the
	// classic "no such table" when the pool opens a second in-memory DB).
	if sqlDB, err := database.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	err = database.AutoMigrate(
		&db.Implant{}, &db.Task{}, &db.AuditLog{}, &db.Listener{},
		&db.TokenEntry{}, &db.SocksSession{}, &db.User{},
		&db.CredentialEntry{}, &db.CloudCred{}, &db.ScheduledReport{},
		&db.BuildLog{}, &db.ScanResult{}, &db.NetworkHost{}, &db.CommandTemplate{},
		&db.BOFFile{}, &db.BOFLibrary{}, &db.ServerConfig{}, &db.WebhookConfig{},
		&db.Plugin{}, &db.PluginReview{}, &db.PluginDependency{}, &db.PluginUpdateStatus{},
		&db.AutomationRule{}, &db.AlertRule{}, &db.Alert{}, &db.SystemMetric{},
		&db.GeneratedReport{}, &db.RolePermission{}, &db.MeshPeer{},
		&db.BloodHoundResult{}, &db.BloodHoundFile{}, &db.Campaign{},
		&db.CampaignAgent{}, &db.OpsecHistory{}, &db.CircuitBreakerConfig{},
		&db.CircuitBreakerEvent{}, &db.CustomRole{}, &db.SessionRecording{},
		&db.PhishingTemplate{}, &db.PhishingCampaign{}, &db.PhishingEvent{},
		&db.AgentTag{}, &db.AgentTagAssignment{}, &db.AutoTagRule{}, &db.ScheduledTask{}, &db.Notification{},
		&db.AgentGroup{}, &db.AgentGroupAssignment{}, &db.Workflow{}, &db.WorkflowStep{},
		&db.WorkflowExecution{}, &db.WorkflowStepLog{}, &db.ChatMessage{},
		&db.StagerToken{}, &db.Redirector{}, &db.AgentLock{},
		&db.AIChatSession{}, &db.AIChatMessage{}, &db.ExtC2Channel{},
		&db.UserSession{}, &db.BackupCode{}, &db.OpsecRule{},
		&db.PasswordHistory{}, &db.ApiKey{}, &db.Script{},
		&db.RegSecret{}, &db.KillSwitch{},
	)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

type TestServer struct {
	DB *gorm.DB
	G  *gin.Engine
}

func NewGinTestServer(t *testing.T, db *gorm.DB) *TestServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &TestServer{DB: db}
}

func AssertValidJSON(t *testing.T, body []byte, label string) {
	t.Helper()
	if len(body) == 0 {
		t.Fatalf("%s: empty body", label)
	}
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("%s: invalid json: %v; body=%s", label, err, string(body))
	}
}

func AssertKeyExists(t *testing.T, body []byte, label, key string) {
	t.Helper()
	var bodyObj map[string]any
	if err := json.Unmarshal(body, &bodyObj); err != nil {
		t.Fatalf("%s: invalid json: %v; body=%s", label, err, string(body))
	}
	if _, ok := bodyObj[key]; !ok {
		t.Fatalf("%s: missing key %q; body=%s", label, key, string(body))
	}
}
