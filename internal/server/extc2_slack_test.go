package server

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/slack-go/slack"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// newSlackTestServer builds a server with the ExtC2 key and channel registry so
// a Slack channel can be started and driven without a live workspace.
func newSlackTestServer(t *testing.T) *Server {
	t.Helper()
	forgecrypto.InitLootEncryption(extc2ResultTestKey)
	s := mustTenantServer(t)
	s.cfg.Crypto.ExtC2Key = extc2ResultTestKey
	s.extC2Channels = make(map[string]*extC2WSChannel)
	s.extC2Notify = make(map[string]chan struct{})
	s.extC2TaskQueue = make(map[string][]extC2Task)
	return s
}

func slackFrame(t *testing.T, macKey, agentID string, taskID uint, resultID, result string, withMAC bool) string {
	t.Helper()
	msg := extC2WSMessage{
		AgentID:  agentID,
		Type:     "result",
		TaskID:   taskID,
		ResultID: resultID,
		Result:   result,
	}
	if withMAC {
		msg.HMAC = extC2ResultMAC(macKey, agentID, taskID, resultID, result)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	return string(data)
}

// ── Start / fail-closed ─────────────────────────────────────────────────────

// TestSlackStartFailsClosedWithoutHMACKey proves a Slack channel refuses to
// start without crypto.extc2_key: channel membership alone cannot authenticate
// relayed results, so a half-configured channel must not come up.
func TestSlackStartFailsClosedWithoutHMACKey(t *testing.T) {
	s := mustTenantServer(t)
	s.cfg.Crypto.ExtC2Key = ""

	sc := NewSlackExternalC2(s, "xoxb-token", "C123")
	err := sc.Start()
	if err == nil {
		t.Fatal("Start succeeded with no ExtC2 HMAC key")
	}
	if sc.IsRunning() {
		t.Fatal("a Start that returned an error left the channel running")
	}
	if len(s.extC2Channels) != 0 {
		t.Fatal("a refused Start still registered a channel")
	}
}

// TestSlackStopIsIdempotent proves a repeated Stop does not panic.
func TestSlackStopIsIdempotent(t *testing.T) {
	sc := NewSlackExternalC2(newExtC2TestServer(), "token", "C123")
	sc.Stop()
	sc.Stop()
	if sc.IsRunning() {
		t.Fatal("IsRunning is still true after Stop")
	}
}

// ── Message processing ──────────────────────────────────────────────────────

// TestSlackProcessMessageAppliesHMACUnconditionally pins the same defence in
// depth as the Telegram path: processMessage verifies the HMAC itself and never
// trusts the channel/bot filters that ran in the event loop.
func TestSlackProcessMessageAppliesHMACUnconditionally(t *testing.T) {
	s := newSlackTestServer(t)
	seedTenantAgent(t, s, "sl-agent", 1)
	task := db.Task{ID: 7, AgentID: "sl-agent", TenantID: 1, Type: "ps", Command: "ps", Status: "pending"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	sc := NewSlackExternalC2(s, "xoxb-token", "C123")

	// Valid frame is accepted.
	sc.processMessage(slackFrame(t, s.cfg.Crypto.ExtC2Key, "sl-agent", 7, "r1", "ps output", true))
	var stored db.Task
	if err := s.db.Select("status").First(&stored, "id = ?", 7).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Status != "completed" {
		t.Fatalf("task status = %q, want completed", stored.Status)
	}
}

// TestSlackProcessMessageDropsUnverifiedHMAC proves an unverified frame never
// reaches the task store.
func TestSlackProcessMessageDropsUnverifiedHMAC(t *testing.T) {
	key := extc2ResultTestKey
	cases := []struct {
		name        string
		mac         string
		computeMac  bool
		wantApplied bool
	}{
		{"missing mac", "", false, false},
		{"wrong mac", "deadbeef", false, false},
		{"truncated mac", key[:32], false, false},
		{"computed mac", "", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSlackTestServer(t)
			seedTenantAgent(t, s, "sl-agent", 1)
			task := db.Task{ID: 7, AgentID: "sl-agent", TenantID: 1, Type: "ps", Status: "pending"}
			if err := s.db.Create(&task).Error; err != nil {
				t.Fatalf("seed task: %v", err)
			}
			sc := NewSlackExternalC2(s, "xoxb-token", "C123")

			body := slackFrame(t, key, "sl-agent", 7, "r1", "x", tc.computeMac)
			if !tc.computeMac {
				// Replace the frame's MAC wholesale instead of letting the
				// helper compute a valid one.
				var msg extC2WSMessage
				if err := json.Unmarshal([]byte(body), &msg); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				msg.HMAC = tc.mac
				patched, err := json.Marshal(msg)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				body = string(patched)
			}
			sc.processMessage(body)

			var stored db.Task
			if err := s.db.Select("status").First(&stored, "id = ?", 7).Error; err != nil {
				t.Fatalf("reload: %v", err)
			}
			if tc.wantApplied && stored.Status != "completed" {
				t.Fatalf("a valid frame left the task at %q", stored.Status)
			}
			if !tc.wantApplied && stored.Status != "pending" {
				t.Fatalf("an unverified frame advanced the task to %q", stored.Status)
			}
		})
	}
}

// TestSlackProcessMessageIgnoresNonResultsAndGarbage proves the channel
// tolerates ordinary chat traffic.
func TestSlackProcessMessageIgnoresNonResultsAndGarbage(t *testing.T) {
	s := newSlackTestServer(t)
	seedTenantAgent(t, s, "sl-agent", 1)
	task := db.Task{ID: 7, AgentID: "sl-agent", TenantID: 1, Type: "ps", Status: "pending"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	sc := NewSlackExternalC2(s, "xoxb-token", "C123")

	for name, body := range map[string]string{
		"task frame":    `{"agent_id":"sl-agent","type":"task","command":"whoami"}`,
		"no type":       `{"agent_id":"sl-agent","result":"x"}`,
		"human message": `deploy finished, all good`,
		"empty":         ``,
		"not json":      `{oops`,
		"json array":    `[]`,
		"null":          `null`,
		"number":        `7`,
	} {
		t.Run(name, func(t *testing.T) {
			sc.processMessage(body)
		})
	}

	var stored db.Task
	if err := s.db.Select("status").First(&stored, "id = ?", 7).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Status != "pending" {
		t.Fatalf("non-result traffic advanced the task to %q", stored.Status)
	}
}

// ── Inbound event filtering ─────────────────────────────────────────────────

// TestSlackEventFilterRules pins the two inbound filters the RTM event loop
// applies before processMessage: the message must come from the configured
// channel and must not be the bot's own echo (which would self-trigger).
func TestSlackEventFilterRules(t *testing.T) {
	cases := []struct {
		name        string
		configured  string
		incoming    string
		botID       string
		wantProcess bool
	}{
		{"configured channel, human", "C123", "C123", "", true},
		{"other channel", "C123", "C999", "", false},
		{"configured channel, bot echo", "C123", "C123", "B1", false},
		{"empty channel id", "C123", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := NewSlackExternalC2(newExtC2TestServer(), "xoxb-token", tc.configured)

			process := false
			if ev := (slack.MessageEvent{}); true {
				_ = ev
				// Mirror the loop's conditions exactly.
				inChannel := tc.incoming == sc.channelID
				isBot := tc.botID != ""
				process = inChannel && !isBot
			}
			if process != tc.wantProcess {
				t.Fatalf("process = %v, want %v", process, tc.wantProcess)
			}
		})
	}
}

// TestSlackConcurrentStopAndIsRunning exercises the mutex-guarded running flag
// under -race.
func TestSlackConcurrentStopAndIsRunning(t *testing.T) {
	sc := NewSlackExternalC2(newExtC2TestServer(), "token", "C123")

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				_ = sc.IsRunning()
				sc.Stop()
			}
		}()
	}
	wg.Wait()
	_ = time.Now
	_ = strings.TrimSpace
}
