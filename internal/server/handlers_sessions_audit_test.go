package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// TestSessionRevokeWritesAuditLog proves single-session revoke lands in the
// audit chain so operator kick / self-clean is attributable.
func TestSessionRevokeWritesAuditLog(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "revoke-audit-user")
	issueWSAuthToken(t, s, user)

	var sess db.UserSession
	if err := s.db.Where("user_id = ?", user.ID).First(&sess).Error; err != nil {
		t.Fatalf("load session: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/x", nil)
	c.Set("user", "admin-operator")
	c.Set("user_id", uint(1))
	c.Set("user_role", "admin")
	c.Params = gin.Params{
		{Key: "id", Value: "1"},
		{Key: "sessionId", Value: itoa(int(sess.ID))},
	}
	s.handleRevokeSession(c)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke: got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	s.db.Model(&db.AuditLog{}).Where("action = ?", "session_revoke").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 session_revoke audit entry, got %d", count)
	}
}

// TestSessionRevokeAllWritesAuditLog proves bulk revoke is audited with count.
func TestSessionRevokeAllWritesAuditLog(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "revoke-all-audit")
	if err := s.createSession("tok-a", user.ID, "1.1.1.1", "a", "", 3600); err != nil {
		t.Fatal(err)
	}
	if err := s.createSession("tok-b", user.ID, "2.2.2.2", "b", "", 3600); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/x", nil)
	c.Set("user", "admin-operator")
	c.Set("user_role", "admin")
	c.Params = gin.Params{{Key: "id", Value: itoa(int(user.ID))}}
	s.handleRevokeAllUserSessions(c)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke-all: got %d body=%s", w.Code, w.Body.String())
	}

	var count int64
	s.db.Model(&db.AuditLog{}).Where("action = ?", "session_revoke_all").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 session_revoke_all audit entry, got %d", count)
	}

	// Both sessions must actually be revoked.
	var live int64
	s.db.Model(&db.UserSession{}).
		Where("user_id = ? AND revoked_at = ?", user.ID, time.Time{}).
		Count(&live)
	if live != 0 {
		t.Fatalf("expected 0 live sessions after revoke-all, got %d", live)
	}
}
