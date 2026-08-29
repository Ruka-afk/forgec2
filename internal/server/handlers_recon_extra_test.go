package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestHandleUSBDropRequiresSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newTestFileServer(t, testutil.SetupTestDB(t))
	agent := seedImplant(t, s.db)

	w, c := newFormContext(http.MethodPost, "/agents/"+agent.ID+"/usb_drop", nil)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	s.handleUSBDrop(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without source, got %d body=%s", w.Code, w.Body.String())
	}
}
func TestHandleUSBDropCreatesTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newTestFileServer(t, testutil.SetupTestDB(t))
	agent := seedImplant(t, s.db)

	body := bytes.NewBufferString(`{"path":"C:\\temp\\payload.exe","dest":"E:\\"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/agents/"+agent.ID+"/usb_drop", body)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	c.Set("user_role", "operator")
	s.handleUSBDrop(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var stored db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", agent.ID, "usb_drop").First(&stored).Error; err != nil {
		t.Fatalf("task: %v", err)
	}
	if stored.Path != `C:\temp\payload.exe` {
		t.Fatalf("path=%q", stored.Path)
	}
	if stored.Command != `E:\` {
		t.Fatalf("command=%q", stored.Command)
	}
}

func TestHandleFileHuntJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newTestFileServer(t, testutil.SetupTestDB(t))
	agent := seedImplant(t, s.db)

	body := bytes.NewBufferString(`{"path":"C:\\Users\\lab","pattern":"*.pdf","download":false}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/agents/"+agent.ID+"/file_hunt", body)
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	c.Set("user_role", "operator")
	s.handleFileHunt(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var stored db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", agent.ID, "file_hunt").First(&stored).Error; err != nil {
		t.Fatalf("task: %v", err)
	}
	if stored.Command != "*.pdf" {
		t.Fatalf("command=%q", stored.Command)
	}
	if !strings.Contains(stored.Path, "Users") {
		t.Fatalf("path=%q", stored.Path)
	}
}

func TestHandleScreenTriggerStartRequiresMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newTestFileServer(t, testutil.SetupTestDB(t))
	agent := seedImplant(t, s.db)
	w, c := newFormContext(http.MethodPost, "/agents/"+agent.ID+"/screen_trigger/start", nil)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	s.handleScreenTriggerStart(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBuildQuickActionCommandReconExtra(t *testing.T) {
	typ, cmd := buildQuickActionCommand("session_recon", "", "")
	if typ != "session_recon" {
		t.Fatalf("type=%q", typ)
	}
	typ, cmd = buildQuickActionCommand("usb_drop", `C:\temp\a.exe`, "")
	if typ != "usb_drop" || cmd != `C:\temp\a.exe` {
		t.Fatalf("usb_drop %q %q", typ, cmd)
	}
	typ, cmd = buildQuickActionCommand("file_hunt", "*.kdbx", "")
	if typ != "file_hunt" || cmd != "*.kdbx" {
		t.Fatalf("file_hunt %q %q", typ, cmd)
	}
}

func TestReconExtraTaskTypesKnown(t *testing.T) {
	for _, typ := range []string{
		"file_hunt", "screen_trigger_start", "screen_trigger_stop",
		"usb_enum", "usb_drop", "browser_history", "session_recon",
	} {
		if !IsKnownTaskType(typ) {
			t.Errorf("%s not registered", typ)
		}
	}
}
