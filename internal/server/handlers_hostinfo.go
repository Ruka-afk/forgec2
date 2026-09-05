package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// handleRequestHostInfo dispatches a structured host-profile sweep to an
// agent. The optional category form field selects the section (all by
// default) and filter narrows the software listing, so operators can pull one
// slice without paying for a full collection.
//
// POST /agents/:id/hostinfo   category=security&filter=defender   (both optional)
func (s *Server) handleRequestHostInfo(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	category := strings.TrimSpace(c.PostForm("category"))
	if category == "" {
		category = strings.TrimSpace(c.Query("category"))
	}
	filter := strings.TrimSpace(c.PostForm("filter"))
	if filter == "" {
		filter = strings.TrimSpace(c.Query("filter"))
	}

	switch strings.ToLower(category) {
	case "", "all", "security", "system", "software", "network", "runtime":
	default:
		respondError(c, http.StatusBadRequest,
			"invalid category (want: all|security|system|software|network|runtime)")
		return
	}

	details := "host profile sweep"
	if category != "" && category != "all" {
		details += " (" + category + ")"
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "hostinfo", Command: category, Data: filter})
	if task == nil {
		return
	}
	slog.Info("hostinfo requested", "agent_id", id, "category", category, "filter", filter)
	s.LogAuditRecord(c, "request_hostinfo", "agent", id, details, true, nil)
	s.dispatchTask(c, task, "request_hostinfo", details)
}
