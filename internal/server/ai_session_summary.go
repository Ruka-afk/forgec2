package server

import (
	"context"
	"log/slog"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
)

// Rolling conversation summary (LLM compression + memory sediment).
//
// trimConversationHistory drops old turns once the context budget is blown,
// losing every fact in them. Sessions instead keep a rolling digest: once 20+
// messages accumulate past the SummaryUpToID watermark, the oldest batch is
// folded into Session.Summary via one background completion and the summary
// is injected into the system prompt on subsequent runs. Token cost is one
// ~1k completion per 20 messages instead of re-sending full history.

const (
	// aiSummaryThreshold starts a fold once this many unsummarized messages
	// exist. Kept well under aiMaxContextChars pressure so folds are rare.
	aiSummaryThreshold = 20
	// aiSummaryBatch caps how many old messages one fold consumes.
	aiSummaryBatch = 30
	// aiSummaryMaxChars bounds the stored digest.
	aiSummaryMaxChars = 2000
)

// sessionSummaryBlock loads the stored digest for prompt injection. Empty
// when the session has nothing summarized yet.
func (s *Server) sessionSummaryBlock(sessionID uint) string {
	if s == nil || s.db == nil || sessionID == 0 {
		return ""
	}
	var summary string
	if err := s.db.Model(&db.AIChatSession{}).Where("id = ?", sessionID).Pluck("summary", &summary).Error; err != nil {
		return ""
	}
	if strings.TrimSpace(summary) == "" {
		return ""
	}
	return "## Earlier conversation (auto-summary, older turns omitted)\n" + summary
}

// maybeSummarizeSession folds the oldest unsummarized messages into the
// session digest when past threshold. Fire-and-forget safe: all failures
// only log. Never summarizes the newest tail (still sent verbatim).
func (s *Server) maybeSummarizeSession(sessionID uint) {
	if s == nil || s.db == nil || sessionID == 0 || !s.aiAssistReady() {
		return
	}
	var session db.AIChatSession
	if err := s.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return
	}
	var unsummarized []db.AIChatMessage
	if err := s.db.Where("session_id = ? AND id > ?", sessionID, session.SummaryUpToID).
		Order("id").Limit(aiSummaryThreshold + aiSummaryBatch).Find(&unsummarized).Error; err != nil {
		return
	}
	if len(unsummarized) < aiSummaryThreshold {
		return
	}
	// Keep the newest threshold/2 messages verbatim; fold the rest.
	cutoff := len(unsummarized) - aiSummaryThreshold/2
	batch := unsummarized[:cutoff]
	if len(batch) > aiSummaryBatch {
		batch = batch[:aiSummaryBatch]
	}
	var sb strings.Builder
	for _, m := range batch {
		sb.WriteString("[" + m.Role + "] " + truncateStr(m.Content, 1000) + "\n")
	}
	transcript := sb.String()
	if len(transcript) > 12000 {
		transcript = transcript[:12000] + "\n...[truncated]"
	}
	system := `You maintain the rolling memory of a red-team C2 assistant conversation. Fold the new transcript into the existing summary and reply with ONLY the updated summary (plain prose, max 1500 chars). Preserve: operator goals, agents touched (ids/hostnames), actions taken and their outcomes, credentials findings (types only, never values), and open threads. Drop chatter and duplicate tool output.`
	user := "Existing summary (may be empty):\n" + session.Summary + "\n\nNew transcript:\n" + transcript
	text, err := s.aiOneShot(context.Background(), system, user, 800)
	if err != nil {
		slog.Warn("AI session summary fold failed", "session", sessionID, "err", err)
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if len(text) > aiSummaryMaxChars {
		text = text[:aiSummaryMaxChars]
	}
	watermark := batch[len(batch)-1].ID
	if err := s.db.Model(&db.AIChatSession{}).Where("id = ? AND summary_up_to_id = ?", sessionID, session.SummaryUpToID).
		Updates(map[string]interface{}{"summary": text, "summary_up_to_id": watermark}).Error; err != nil {
		slog.Warn("AI session summary persist failed", "session", sessionID, "err", err)
		return
	}
	slog.Info("AI session summary folded", "session", sessionID, "messages", len(batch), "up_to", watermark)
}

// sessionSummaryForPrompt is the injection helper used at both prompt build
// sites (background runs and legacy streaming chat).
func (s *Server) sessionSummaryForPrompt(sessionID uint) string {
	return s.sessionSummaryBlock(sessionID)
}
