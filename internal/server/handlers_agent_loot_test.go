package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

func TestNewLootScreenshot(t *testing.T) {
	mod := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	got := newLootScreenshot("agent-1", "desk.png", mod)
	if got.ID != "screenshot:agent-1:desk.png" {
		t.Fatalf("id=%s", got.ID)
	}
	if got.AgentID != "agent-1" || got.Filename != "desk.png" || got.Path != "agent-1/desk.png" {
		t.Fatalf("dto=%+v", got)
	}
	if got.CreatedAt != "2026-08-14T12:00:00Z" {
		t.Fatalf("created_at=%s", got.CreatedAt)
	}
}

func TestNewLootTaskIncludesHostname(t *testing.T) {
	got := newLootTask(db.Task{
		ID:      7,
		AgentID: "agent-1",
		Type:    "upload",
		Command: `C:\secret.txt`,
		Status:  "completed",
		Result:  "File chunk saved: secret.txt offset 0 (12 bytes)",
		Agent:   db.Implant{Hostname: "WS-01"},
	})
	if got.Hostname != "WS-01" {
		t.Fatalf("hostname=%s", got.Hostname)
	}
	if got.Type != "upload" || got.ID != 7 {
		t.Fatalf("dto=%+v", got)
	}
}
