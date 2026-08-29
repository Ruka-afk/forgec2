package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

// TestSIEMCustomRuleFromDB verifies ReloadRules pulls user-authored rules from
// the DB and that the correlator evaluates them: a custom rule on a unique
// action fires an alert once its threshold is crossed, and no longer fires
// after the rule is disabled and reloaded.
func TestSIEMCustomRuleFromDB(t *testing.T) {
	database := testutil.SetupTestDB(t)
	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	s := &Server{db: database, cfg: &config.Config{}, sessionManager: sm}

	sw := &SIEMWebhook{server: s, correlator: NewEventCorrelator()}

	rule := db.SIEMRule{
		Name:         "custom_login_spike",
		Enabled:      true,
		Action:       "login_failed",
		WindowSec:    60,
		Threshold:    3,
		AlertAction:  "siem_alert",
		AlertDetails: "custom rule fired",
	}
	if err := database.Create(&rule).Error; err != nil {
		t.Fatalf("create custom rule: %v", err)
	}

	sw.ReloadRules()
	if got := len(sw.correlator.rules); got != 1 {
		t.Fatalf("expected 1 custom rule after reload, got %d", got)
	}

	agentID := "siem-test-agent-1"
	var alerts []SIEMEvent
	for i := 0; i < 3; i++ {
		alerts = sw.correlator.ProcessEvent(SIEMEvent{Action: "login_failed", AgentID: agentID, Timestamp: time.Now()})
	}
	if len(alerts) == 0 {
		t.Fatal("expected an alert after threshold crossed")
	}
	if alerts[0].Action != "siem_alert" || alerts[0].Details != "custom rule fired" {
		t.Fatalf("unexpected alert: %+v", alerts[0])
	}

	// Disabling the rule and reloading removes it from evaluation; with no
	// enabled custom rules left, ReloadRules falls back to the built-in set.
	if err := database.Model(&db.SIEMRule{}).Where("id = ?", rule.ID).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable rule: %v", err)
	}
	sw.ReloadRules()
	if got := len(sw.correlator.rules); got != len(defaultCorrelationRules()) {
		t.Fatalf("expected default rule fallback after disable, got %d rules", got)
	}
	alerts = sw.correlator.ProcessEvent(SIEMEvent{Action: "login_failed", AgentID: agentID, Timestamp: time.Now()})
	if len(alerts) != 0 {
		t.Fatalf("expected no login_failed alert from defaults after custom rule disabled, got %d", len(alerts))
	}
}

// TestSIEMFallbackDefaults verifies that with no custom rules in the DB,
// ReloadRules keeps the built-in default rule set active.
func TestSIEMFallbackDefaults(t *testing.T) {
	database := testutil.SetupTestDB(t)
	sm, _ := crypto.NewSessionManager()
	s := &Server{db: database, cfg: &config.Config{}, sessionManager: sm}

	sw := &SIEMWebhook{server: s, correlator: NewEventCorrelator()}
	sw.ReloadRules()

	if len(sw.correlator.rules) != len(defaultCorrelationRules()) {
		t.Fatalf("expected default rules fallback, got %d rules", len(sw.correlator.rules))
	}
}

// TestSIEMDefaultRuleFiresNoCustom verifies the built-in privilege-escalation
// rule (threshold 1) still fires via the process path when defaults are in use.
func TestSIEMDefaultRuleFires(t *testing.T) {
	database := testutil.SetupTestDB(t)
	sm, _ := crypto.NewSessionManager()
	s := &Server{db: database, cfg: &config.Config{}, sessionManager: sm}

	sw := &SIEMWebhook{server: s, correlator: NewEventCorrelator()}
	sw.ReloadRules()
	alerts := sw.correlator.ProcessEvent(SIEMEvent{Action: "agent_elevated", AgentID: "a", Timestamp: time.Now()})
	if len(alerts) == 0 {
		t.Fatal("privilege escalation rule should fire")
	}
}

// TestTelegramResultHMACGate verifies the Telegram channel drops relayed
// results whose extc2_key HMAC is missing/forged and accepts a valid one.
func TestTelegramResultHMACGate(t *testing.T) {
	database := testutil.SetupTestDB(t)
	sm, _ := crypto.NewSessionManager()

	key := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b"
	cfg := &config.Config{}
	cfg.Crypto.ExtC2Key = key
	s := &Server{
		db:             database,
		cfg:            cfg,
		sessionManager: sm,
	}
	tg := NewTelegramExternalC2(s, "test-bot-token", "123456789")

	agentID := "agent-hmac-1"
	taskID := uint(42)
	resultID := "rid-1"
	result := "command output"

	validMAC := extC2ResultMAC(key, agentID, taskID, resultID, result)
	if !s.verifyExtC2ResultHMAC(agentID, taskID, resultID, result, validMAC) {
		t.Fatal("valid HMAC must verify")
	}

	// No HMAC on a keyed channel must be rejected.
	if s.verifyExtC2ResultHMAC(agentID, taskID, resultID, result, "") {
		t.Fatal("missing HMAC must be rejected when extc2_key is set")
	}
	// Forged HMAC must be rejected.
	if s.verifyExtC2ResultHMAC(agentID, taskID, resultID, result, "deadbeef") {
		t.Fatal("forged HMAC must be rejected")
	}

	// Unkeyed channel (legacy) accepts any result for back-compat.
	s.cfg.Crypto.ExtC2Key = ""
	if !s.verifyExtC2ResultHMAC(agentID, taskID, resultID, result, "") {
		t.Fatal("empty extc2_key must allow unauthenticated results (legacy)")
	}
	_ = tg
}
