package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func seedTimelineTask(t *testing.T, s *Server, id uint, agentID, command string, tenantID uint) {
	t.Helper()
	task := db.Task{AgentID: agentID, Type: "shell", Command: command, Status: "completed", Result: "result-of-" + command, TenantID: tenantID}
	task.ID = id
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
}

// TestTimelineTenantScoped proves the shared timeline builder (page, JSON
// and CSV export) contains only the caller's tenant: foreign task commands,
// results and hostnames must not appear.
func TestTimelineTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "tl-own", 1)
	seedTenantAgent(t, s, "tl-foreign", 2)
	if err := s.db.Model(&db.Implant{}).Where("id = ?", "tl-own").Update("hostname", "OWN-TL-HOST").Error; err != nil {
		t.Fatalf("rename own: %v", err)
	}
	if err := s.db.Model(&db.Implant{}).Where("id = ?", "tl-foreign").Update("hostname", "FOREIGN-TL-HOST").Error; err != nil {
		t.Fatalf("rename foreign: %v", err)
	}
	seedTimelineTask(t, s, 91001, "tl-own", "own-secret-cmd", 1)
	seedTimelineTask(t, s, 91002, "tl-foreign", "foreign-secret-cmd", 2)

	c, _ := tenantScopedAdminContext(s, t, "tl-viewer", 1)
	events := s.buildTimelineEvents(c, "", "", "", "", "")
	var sb strings.Builder
	for _, ev := range events {
		sb.WriteString(ev.Type + "|" + ev.Action + "|" + ev.Details + "|" + ev.AgentName + "|")
	}
	blob := sb.String()
	for _, want := range []string{"own-secret-cmd", "OWN-TL-HOST"} {
		if !strings.Contains(blob, want) {
			t.Fatalf("own-tenant content %q missing from timeline", want)
		}
	}
	for _, leak := range []string{"foreign-secret-cmd", "result-of-foreign-secret-cmd", "FOREIGN-TL-HOST"} {
		if strings.Contains(blob, leak) {
			t.Fatalf("foreign-tenant content %q leaked into timeline", leak)
		}
	}
}

// TestTimelineExportAudited proves the CSV export writes an audit row.
func TestTimelineExportAudited(t *testing.T) {
	s := mustTenantServer(t)
	c, w := tenantScopedAdminContext(s, t, "tl-exporter", 1)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	s.handleTimelineExport(c)
	if w.Code != http.StatusOK {
		t.Fatalf("export status=%d, want 200", w.Code)
	}
	var n int64
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "timeline_export").Count(&n).Error; err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n == 0 {
		t.Fatalf("no timeline_export audit row")
	}
}

// TestScanExportCrossTenant404 proves scan results inherit the owning
// task's tenant: sequential task IDs are not enumerable across tenants.
func TestScanExportCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTimelineTask(t, s, 92001, "tl-foreign-agent", "nmap 10.0.0.0/8", 2)
	seedTenantAgent(t, s, "tl-foreign-agent", 2)
	if err := s.db.Create(&db.ScanResult{TaskID: 92001, Port: 443, Protocol: "tcp", State: "open", Service: "https"}).Error; err != nil {
		t.Fatalf("seed scan: %v", err)
	}

	c, w := tenantScopedAdminContext(s, t, "tl-scanviewer", 1)
	c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "92001"}}
	s.handleExportScanResults(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign scan export status=%d body=%s, want 404", w.Code, w.Body.String())
	}

	// Own-tenant task exports fine.
	seedTenantAgent(t, s, "tl-own-agent", 1)
	seedTimelineTask(t, s, 92002, "tl-own-agent", "nmap 192.168.1.0/24", 1)
	c2, w2 := tenantScopedAdminContext(s, t, "tl-scanviewer", 1)
	c2.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	c2.Params = gin.Params{{Key: "taskId", Value: "92002"}}
	s.handleExportScanResults(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("own scan export status=%d, want 200", w2.Code)
	}
}
