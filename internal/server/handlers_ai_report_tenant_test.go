package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

func seedReportAgent(t *testing.T, s *Server, id, hostname string, tenantID uint) {
	t.Helper()
	if err := s.db.Create(&db.Implant{ID: id, Hostname: hostname, TenantID: tenantID, Status: "online"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
}

// TestAIMarkdownReportTenantScoped proves an AI principal's generated
// report contains only its own tenant's agents, tasks, credentials and
// listeners (previously every section was a global query).
func TestAIMarkdownReportTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedReportAgent(t, s, "ar-own", "OWN-HOST", 1)
	seedReportAgent(t, s, "ar-foreign", "FOREIGN-HOST", 2)
	if err := s.db.Create(&db.Task{AgentID: "ar-own", Type: "shell", Command: "own-cmd", Status: "completed"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := s.db.Create(&db.Task{AgentID: "ar-foreign", Type: "shell", Command: "foreign-cmd", Status: "completed"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	if err := s.db.Create(&db.Listener{Name: "ar-own-lis", Scheme: "http", Host: "127.0.0.1", Port: 18091, TenantID: 1}).Error; err != nil {
		t.Fatalf("seed listener: %v", err)
	}
	if err := s.db.Create(&db.Listener{Name: "ar-foreign-lis", Scheme: "http", Host: "127.0.0.1", Port: 18092, TenantID: 2}).Error; err != nil {
		t.Fatalf("seed listener: %v", err)
	}

	reqCtx := &aiReqCtx{Principal: aiPrincipal{UserID: 7, Username: "ar-op", TenantID: 1, Role: "user"}}
	md, _, err := s.buildAIMarkdownReport("full", reqCtx)
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	// Agents + listeners render names; tasks/creds render scoped counts
	// (1 own + 1 foreign seeded => completed row must read exactly 1).
	for _, want := range []string{"OWN-HOST", "ar-own-lis", "Completed: **1**"} {
		if !strings.Contains(md, want) {
			t.Fatalf("own-tenant content %q missing from report", want)
		}
	}
	for _, leak := range []string{"FOREIGN-HOST", "ar-foreign-lis", "Completed: **2**"} {
		if strings.Contains(md, leak) {
			t.Fatalf("foreign-tenant content %q leaked into report", leak)
		}
	}

	// Anonymous/system callers keep the legacy global view.
	mdAll, _, err := s.buildAIMarkdownReport("full", nil)
	if err != nil {
		t.Fatalf("build report (anon): %v", err)
	}
	if !strings.Contains(mdAll, "FOREIGN-HOST") {
		t.Fatalf("anonymous report lost global visibility")
	}
}
