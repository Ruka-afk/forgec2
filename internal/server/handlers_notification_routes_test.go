package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupNotificationRouteTestServer(t *testing.T) (*Server, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&db.NotificationRoute{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := &Server{db: database}
	r := gin.New()
	r.GET("/routes", s.handleListNotificationRoutes)
	r.POST("/routes", s.handleCreateNotificationRoute)
	r.PUT("/routes/:id", s.handleUpdateNotificationRoute)
	r.POST("/routes/:id/test", s.handleTestNotificationRoute)
	return s, r
}

func performNotificationJSON(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateNotificationRouteDoesNotEchoSecret(t *testing.T) {
	s, r := setupNotificationRouteTestServer(t)
	w := performNotificationJSON(r, http.MethodPost, "/routes", map[string]interface{}{
		"name": "soc-telegram", "channel": "telegram", "target": "-100123",
		"secret": "123456:bot-token", "min_severity": "warning",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create returned %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Route db.NotificationRoute `json:"route"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Route.Secret != notificationRouteMask {
		t.Fatalf("response exposed secret: %q", response.Route.Secret)
	}

	var stored db.NotificationRoute
	if err := s.db.First(&stored, response.Route.ID).Error; err != nil {
		t.Fatalf("load stored route: %v", err)
	}
	if stored.Secret != "123456:bot-token" {
		t.Fatalf("stored secret = %q", stored.Secret)
	}
}

func TestListNotificationRoutesRedactsWebhookTarget(t *testing.T) {
	s, r := setupNotificationRouteTestServer(t)
	route := db.NotificationRoute{
		Name: "soc-discord", Channel: "discord",
		Target: "https://discord.example/hooks/sensitive-token", Enabled: true,
	}
	if err := s.db.Create(&route).Error; err != nil {
		t.Fatalf("seed route: %v", err)
	}
	w := performNotificationJSON(r, http.MethodGet, "/routes", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list returned %d", w.Code)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("sensitive-token")) {
		t.Fatalf("list response exposed webhook credential: %s", w.Body.String())
	}
}

func TestCreateNotificationRouteRejectsPrivateTarget(t *testing.T) {
	_, r := setupNotificationRouteTestServer(t)
	w := performNotificationJSON(r, http.MethodPost, "/routes", map[string]interface{}{
		"name": "internal", "channel": "webhook", "target": "http://127.0.0.1:9000/hook",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("private target returned %d, want 400", w.Code)
	}
}

func TestUpdateNotificationRouteKeepsMaskedCredentials(t *testing.T) {
	s, r := setupNotificationRouteTestServer(t)
	route := db.NotificationRoute{
		Name: "existing", Channel: "discord", Target: "https://1.1.1.1/hooks/token",
		Secret: "stored-secret", Enabled: true, MinSeverity: "info",
	}
	if err := s.db.Create(&route).Error; err != nil {
		t.Fatalf("seed route: %v", err)
	}
	w := performNotificationJSON(r, http.MethodPut, "/routes/1", map[string]interface{}{
		"name": "renamed", "target": notificationRouteMask, "secret": notificationRouteMask,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update returned %d: %s", w.Code, w.Body.String())
	}
	var stored db.NotificationRoute
	if err := s.db.First(&stored, route.ID).Error; err != nil {
		t.Fatalf("load route: %v", err)
	}
	if stored.Target != route.Target || stored.Secret != route.Secret {
		t.Fatalf("masked update changed credentials: target=%q secret=%q", stored.Target, stored.Secret)
	}
}

func TestNotificationRouteTestReportsDeliveryFailure(t *testing.T) {
	s, r := setupNotificationRouteTestServer(t)
	route := db.NotificationRoute{Name: "legacy", Channel: "webhook", Target: "http://127.0.0.1:1/hook"}
	if err := s.db.Create(&route).Error; err != nil {
		t.Fatalf("seed route: %v", err)
	}
	w := performNotificationJSON(r, http.MethodPost, "/routes/1/test", map[string]interface{}{})
	if w.Code != http.StatusBadGateway {
		t.Fatalf("failed delivery returned %d, want 502", w.Code)
	}
}
