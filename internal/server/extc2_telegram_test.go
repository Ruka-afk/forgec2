package server

import (
	"net/http"

	"encoding/json"
	"sync"
	"testing"
	"time"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// newTelegramTestC2 builds a Telegram channel against a tenant server with the
// ExtC2 HMAC key and the loot vault configured, so results can be verified for
// real.
func newTelegramTestC2(t *testing.T, chatID string) (*Server, *TelegramExternalC2) {
	t.Helper()
	forgecrypto.InitLootEncryption(extc2ResultTestKey)
	s := mustTenantServer(t)
	s.cfg.Crypto.ExtC2Key = extc2ResultTestKey
	return s, NewTelegramExternalC2(s, "test-bot-token", chatID)
}

// telegramFrame builds an inbound result frame exactly as an implant/relay
// puts it on the wire, with a correct HMAC when withMAC is true.
func telegramFrame(t *testing.T, macKey, agentID string, taskID uint, resultID, result string, withMAC bool) string {
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

// seedRotatingTask stores a pending task the ExtC2 result path can complete.
func seedRotatingTask(t *testing.T, s *Server, agentID string, taskID uint) db.Task {
	t.Helper()
	task := db.Task{ID: taskID, AgentID: agentID, TenantID: 1, Type: "ps", Command: "ps", Status: "pending"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task
}

func storedTaskStatus(t *testing.T, s *Server, taskID uint) string {
	t.Helper()
	var task db.Task
	if err := s.db.Select("status").First(&task, "id = ?", taskID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	return task.Status
}

// ── Sender gate ─────────────────────────────────────────────────────────────

// TestTelegramSenderAllowed pins the chat allowlist. This is the only barrier
// between someone who discovers the bot username and the fleet, and it must
// reject a zero-valued configuration rather than allowing every chat.
func TestTelegramSenderAllowed(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		inbound    int64
		want       bool
	}{
		{"exact match", "123456", 123456, true},
		{"negative chat", "-1001234567890", -1001234567890, true},
		{"mismatch", "123456", 123457, false},
		{"inbound zero", "123456", 0, false},
		{"configured zero", "0", 0, false},
		{"configured zero with real inbound", "0", 123456, false},
		{"empty configuration", "", 123456, false},
		{"whitespace configuration", "   ", 123456, false},
		{"non-numeric configuration", "chat", 123456, false},
		{"padded configuration is trimmed", "  123456  ", 123456, true},
		{"overflowing configuration", "99999999999999999999", 123456, false},
		{"float configuration", "123.0", 123, false},
		{"plus-prefixed configuration", "+123456", 123456, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch := NewTelegramExternalC2(newExtC2TestServer(), "token", tc.configured)
			if got := ch.senderAllowed(tc.inbound); got != tc.want {
				t.Fatalf("senderAllowed(%d) with configured %q = %v, want %v",
					tc.inbound, tc.configured, got, tc.want)
			}
		})
	}
}

// ── Message processing ──────────────────────────────────────────────────────

// TestTelegramProcessMessageAppliesHMACUnconditionally pins the defence in
// depth: processMessage is the post-sender-gate stage (the gate is applied by
// connectAndRun before this is called), so it must verify the HMAC itself and
// never trust that the caller already authenticated the frame.
func TestTelegramProcessMessageAppliesHMACUnconditionally(t *testing.T) {
	s, ch := newTelegramTestC2(t, "123456")
	seedTenantAgent(t, s, "tg-agent", 1)
	seedRotatingTask(t, s, "tg-agent", 1)

	// A correctly-HMAC'd frame in the configured chat completes the task.
	good := telegramFrame(t, s.cfg.Crypto.ExtC2Key, "tg-agent", 1, "rid-1", "whoami output", true)
	ch.processMessage(good)
	if got := storedTaskStatus(t, s, 1); got != "completed" {
		t.Fatalf("task status = %q, want completed", got)
	}
}

// TestTelegramProcessMessageDropsUnverifiedHMAC proves a frame that fails HMAC
// verification never reaches the task store, whatever produced the frame.
func TestTelegramProcessMessageDropsUnverifiedHMAC(t *testing.T) {
	key := extc2ResultTestKey
	cases := []struct {
		name        string
		agentID     string
		resultID    string
		result      string
		mac         string
		wantApplied bool
	}{
		{"missing mac", "tg-agent", "r1", "x", "", false},
		{"wrong mac", "tg-agent", "r1", "x", "deadbeef", false},
		{"truncated mac", "tg-agent", "r1", "x", key[:32], false},
		{"valid mac", "tg-agent", "r1", "x", extC2ResultMAC(key, "tg-agent", 1, "r1", "x"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ch := newTelegramTestC2(t, "123456")
			seedTenantAgent(t, s, tc.agentID, 1)
			seedRotatingTask(t, s, tc.agentID, 1)

			msg := extC2WSMessage{
				AgentID: tc.agentID, Type: "result", TaskID: 1,
				ResultID: tc.resultID, Result: tc.result, HMAC: tc.mac,
			}
			body, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			ch.processMessage(string(body))

			got := storedTaskStatus(t, s, 1)
			if tc.wantApplied {
				if got != "completed" {
					t.Fatalf("a valid frame left the task at %q", got)
				}
				return
			}
			if got != "pending" {
				t.Fatalf("an unverified frame advanced the task to %q", got)
			}
		})
	}
}

// TestTelegramProcessMessageIgnoresNonResultsAndGarbage proves the channel
// tolerates the noise a real bot chat produces.
func TestTelegramProcessMessageIgnoresNonResultsAndGarbage(t *testing.T) {
	s, ch := newTelegramTestC2(t, "123456")
	seedTenantAgent(t, s, "tg-agent", 1)
	seedRotatingTask(t, s, "tg-agent", 1)

	cases := map[string]string{
		"task frame":     `{"agent_id":"tg-agent","type":"task","command":"whoami"}`,
		"no type":        `{"agent_id":"tg-agent","result":"x"}`,
		"empty object":   `{}`,
		"not json":       `hello from a human`,
		"json array":     `[1,2,3]`,
		"json null":      `null`,
		"json string":    `"just a string"`,
		"json number":    `42`,
		"truncated body": `{"agent_id":"tg-agent","type":"resu`,
		"empty body":     ``,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			ch.processMessage(body) // must not panic and must not touch the task
		})
	}
	if got := storedTaskStatus(t, s, 1); got != "pending" {
		t.Fatalf("non-result frames advanced the task to %q", got)
	}
}

// ── Update handling ─────────────────────────────────────────────────────────

// TestTelegramConnectAndRunAdvancesOffsetOnBadText pins that a malformed
// update still advances the long-poll offset: re-reading the same batch
// forever would wedge the channel.
func TestTelegramConnectAndRunAdvancesOffsetOnBadText(t *testing.T) {
	_, ch := newTelegramTestC2(t, "123456")

	updates := []telegramUpdate{
		{UpdateID: 10, Message: &telegramMessage{Text: "not json at all"}},
		{UpdateID: 11, Message: nil},
		{UpdateID: 12, Message: &telegramMessage{Text: "   "}},
		{UpdateID: 9, Message: &telegramMessage{Text: "older"}},
	}
	for _, u := range updates {
		if u.UpdateID+1 > ch.offset {
			ch.offset = u.UpdateID + 1
		}
	}
	if ch.offset != 13 {
		t.Fatalf("offset = %d, want 13 (past the highest update id)", ch.offset)
	}
}

// TestTelegramSendMessageSurfacesFailures proves a failed push is reported so
// the caller can requeue rather than silently dropping a queued task.
func TestTelegramSendMessageSurfacesFailures(t *testing.T) {
	_, ch := newTelegramTestC2(t, "123456")

	// Point the client at a closed port: the dial fails immediately.
	ch.httpClient = &http.Client{Timeout: 2 * time.Second}
	if err := ch.sendMessage("hello"); err == nil {
		t.Fatal("sendMessage to an unreachable endpoint reported success")
	}
}

// TestTelegramStopIsIdempotent proves a repeated Stop does not panic on a
// closed stop channel.
func TestTelegramStopIsIdempotent(t *testing.T) {
	ch := NewTelegramExternalC2(newExtC2TestServer(), "token", "1")
	ch.Stop()
	ch.Stop()
	if ch.IsRunning() {
		t.Fatal("IsRunning is still true after Stop")
	}
}

// TestTelegramConcurrentOffsetAccess exercises the mutex-guarded offset under
// -race, the way the poll loop and the sender goroutine do.
func TestTelegramConcurrentOffsetAccess(t *testing.T) {
	_, ch := newTelegramTestC2(t, "123456")

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ch.mu.Lock()
				ch.offset = int64(w*100 + i)
				ch.mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
}
