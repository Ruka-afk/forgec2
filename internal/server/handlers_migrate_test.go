package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestHandleMigrateCreatesTask(t *testing.T) {
	s := initKillSwitchServer(t)
	user := seedKillSwitchOperator(t, s)
	if err := s.db.Create(&db.Implant{ID: "agent-1", Registered: true}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	form := url.Values{}
	form.Set("path", "C:\\Users\\public\\stage\\note.exe")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/agents/agent-1/migrate", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Params = gin.Params{{Key: "id", Value: "agent-1"}}
	c.Set("user_id", user.ID)
	c.Set("user_role", db.RoleAdmin)
	c.Set("user", "admin")
	s.handleMigrate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("migrate: expected 200 got %d; body=%s", w.Code, w.Body.String())
	}

	var task db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "agent-1", "migrate").First(&task).Error; err != nil {
		t.Fatalf("migrate task not created: %v", err)
	}
	if task.Command != "C:\\Users\\public\\stage\\note.exe" {
		t.Fatalf("task command mangled: %q", task.Command)
	}
	if task.Status != "pending" {
		t.Fatalf("task status = %q, want pending", task.Status)
	}
}

func TestHandleMigrateRequiresOperator(t *testing.T) {
	s := initKillSwitchServer(t)
	user := seedKillSwitchOperator(t, s)
	if err := s.db.Create(&db.Implant{ID: "agent-2", Registered: true}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/agents/agent-2/migrate", nil)
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Params = gin.Params{{Key: "id", Value: "agent-2"}}
	c.Set("user_id", user.ID)
	c.Set("user_role", "viewer")
	s.handleMigrate(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("migrate by viewer: expected 403 got %d", w.Code)
	}
}
