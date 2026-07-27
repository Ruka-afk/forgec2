package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newNotificationsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&db.Notification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Create(&db.Notification{Type: "agent_online", Title: "Agent online", Message: "a", Severity: "info", Read: false})
	database.Create(&db.Notification{Type: "task_completed", Title: "Task done", Message: "b", Severity: "success", Read: true})
	return database
}

// TestHandleListNotifications_Contract guards the JSON contract the frontend
// NotificationsPage depends on: { notifications: [...], total: N }.
func TestHandleListNotifications_Contract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newNotificationsTestDB(t)}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/notifications?page=1&pageSize=50", nil)
	s.handleListNotifications(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Notifications []db.Notification `json:"notifications"`
		Total         int64             `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if resp.Total != 2 {
		t.Fatalf("total = %d, want 2", resp.Total)
	}
	if len(resp.Notifications) != 2 {
		t.Fatalf("notifications len = %d, want 2", len(resp.Notifications))
	}
}

// TestHandleListNotifications_FilterUnread verifies the read=false filter.
func TestHandleListNotifications_FilterUnread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newNotificationsTestDB(t)}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/notifications?read=false", nil)
	s.handleListNotifications(c)

	var resp struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("unread total = %d, want 1", resp.Total)
	}
}

// TestHandleMarkNotificationRead_Contract verifies the read endpoint mutates state.
func TestHandleMarkNotificationRead_Contract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{db: newNotificationsTestDB(t)}

	var n db.Notification
	s.db.Where("read = ?", false).First(&n)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPut, "/notifications/"+strconv.FormatInt(int64(n.ID), 10)+"/read", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(int64(n.ID), 10)}}
	s.handleMarkNotificationRead(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var after db.Notification
	s.db.First(&after, n.ID)
	if !after.Read {
		t.Fatal("notification was not marked read")
	}
}

// TestRegisterBoth_RegistersBothForms ensures the dual-registration helper
// (added to prevent the /monitor/alerts 404 class of bug) registers
// BOTH the /api-prefixed and bare path forms.
func TestRegisterBoth_RegistersBothForms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/")
	handler := func(c *gin.Context) { c.Status(http.StatusOK) }
	registerBoth(rg, http.MethodGet, "/monitor/alerts", handler)

	routes := map[string]bool{}
	for _, rt := range r.Routes() {
		routes[rt.Method+" "+rt.Path] = true
	}
	if !routes["GET /api/monitor/alerts"] || !routes["GET /monitor/alerts"] {
		t.Fatalf("registerBoth missed a form; routes=%v", routes)
	}
}
