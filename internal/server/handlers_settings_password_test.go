package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// newPasswordChangeServer builds a server with the password-change cooldown map
// wired the way NewServer does; the login test helper leaves it nil.
func newPasswordChangeServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{
		db:             newContractDB(t),
		cfg:            config.DefaultConfig(),
		pwdChangeTimes: make(map[uint]time.Time),
	}
}

func seedPasswordUser(t *testing.T, s *Server, username, password string) db.User {
	t.Helper()
	hash, err := middleware.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := db.User{Username: username, PasswordHash: hash, Role: "admin", IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

// changePasswordForm posts the change-password form for one user.
func changePasswordForm(s *Server, user db.User, current, next, confirm string) *httptest.ResponseRecorder {
	form := url.Values{}
	form.Set("current_password", current)
	form.Set("new_password", next)
	form.Set("confirm_password", confirm)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_id", user.ID)
	c.Set("user", user.Username)
	c.Set("user_role", user.Role)
	s.handleChangePassword(c)
	return w
}

func storedPasswordHash(t *testing.T, s *Server, userID uint) string {
	t.Helper()
	var user db.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return user.PasswordHash
}

// ── Rate limiting ───────────────────────────────────────────────────────────

// TestChangePasswordRateLimitsFailedAttempts is the regression: the cooldown
// used to be armed only after a successful change, so a caller with a valid
// session but no knowledge of the current password could guess forever.
func TestChangePasswordRateLimitsFailedAttempts(t *testing.T) {
	s := newPasswordChangeServer(t)
	user := seedPasswordUser(t, s, "pw-brute", "Current-Pass1")

	// The first wrong guess is a normal 401.
	first := changePasswordForm(s, user, "wrong-guess", "New-Pass123", "New-Pass123")
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first wrong guess: status = %d, want 401 (%s)", first.Code, first.Body.String())
	}

	// The second attempt, correct or not, is inside the cooldown.
	second := changePasswordForm(s, user, "wrong-guess", "New-Pass123", "New-Pass123")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second guess: status = %d, want 429 (%s)", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "rate limited") {
		t.Fatalf("body = %s, want the rate-limit message", second.Body.String())
	}

	// Even the correct password is refused while the window is open, so the
	// guess counter cannot be side-stepped by retrying with the right answer.
	third := changePasswordForm(s, user, "Current-Pass1", "New-Pass123", "New-Pass123")
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("correct password inside the window: status = %d, want 429", third.Code)
	}
	var reloaded db.User
	if err := s.db.First(&reloaded, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !middleware.CheckPassword(reloaded.PasswordHash, "Current-Pass1") {
		t.Fatal("a rate-limited attempt still rotated the password")
	}
}

// TestChangePasswordRateLimitIsPerUser proves one user's cooldown does not lock
// another user out.
func TestChangePasswordRateLimitIsPerUser(t *testing.T) {
	s := newPasswordChangeServer(t)
	noisy := seedPasswordUser(t, s, "pw-noisy", "Current-Pass1")
	quiet := seedPasswordUser(t, s, "pw-quiet", "Current-Pass1")

	if w := changePasswordForm(s, noisy, "wrong", "New-Pass123", "New-Pass123"); w.Code != http.StatusUnauthorized {
		t.Fatalf("noisy user first attempt = %d, want 401", w.Code)
	}
	if w := changePasswordForm(s, noisy, "wrong", "New-Pass123", "New-Pass123"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("noisy user second attempt = %d, want 429", w.Code)
	}
	if w := changePasswordForm(s, quiet, "Current-Pass1", "New-Pass123", "New-Pass123"); w.Code != http.StatusOK {
		t.Fatalf("quiet user = %d, want 200 (%s)", w.Code, w.Body.String())
	}
}

// TestChangePasswordCooldownExpires proves the window actually opens again.
func TestChangePasswordCooldownExpires(t *testing.T) {
	s := newPasswordChangeServer(t)
	user := seedPasswordUser(t, s, "pw-wait", "Current-Pass1")

	s.pwdChangeTimesMu.Lock()
	s.pwdChangeTimes[user.ID] = time.Now().Add(-PasswordChangeCooldown - time.Second)
	s.pwdChangeTimesMu.Unlock()

	if w := changePasswordForm(s, user, "Current-Pass1", "New-Pass123", "New-Pass123"); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 once the window elapsed (%s)", w.Code, w.Body.String())
	}
}

// TestConsumePasswordChangeSlot pins the helper directly, including that it arms
// on the very first claim.
func TestConsumePasswordChangeSlot(t *testing.T) {
	s := newPasswordChangeServer(t)

	if remaining, ok := s.consumePasswordChangeSlot(1); !ok || remaining != 0 {
		t.Fatalf("first claim = (%v, %v), want (0, true)", remaining, ok)
	}
	remaining, ok := s.consumePasswordChangeSlot(1)
	if ok {
		t.Fatal("a second claim inside the window was granted")
	}
	if remaining <= 0 || remaining > PasswordChangeCooldown {
		t.Fatalf("remaining = %v, want a positive wait no longer than %v", remaining, PasswordChangeCooldown)
	}
	// A different user has its own slot.
	if _, ok := s.consumePasswordChangeSlot(2); !ok {
		t.Fatal("another user's slot was blocked")
	}
}

// ── Success path ────────────────────────────────────────────────────────────

// TestChangePasswordRotatesAndRevokes proves the happy path still rotates the
// hash, records history, revokes live sessions and audits once.
func TestChangePasswordRotatesAndRevokes(t *testing.T) {
	s := newPasswordChangeServer(t)
	user := seedPasswordUser(t, s, "pw-happy", "Current-Pass1")
	oldHash := storedPasswordHash(t, s, user.ID)

	for i := 0; i < 2; i++ {
		if err := s.db.Create(&db.UserSession{UserID: user.ID, TokenHash: "tok"}).Error; err != nil {
			t.Fatalf("seed session: %v", err)
		}
	}

	if w := changePasswordForm(s, user, "Current-Pass1", "New-Pass123", "New-Pass123"); w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	newHash := storedPasswordHash(t, s, user.ID)
	if newHash == oldHash {
		t.Fatal("password hash was not rotated")
	}
	if !middleware.CheckPassword(newHash, "New-Pass123") {
		t.Fatal("new password does not verify against the stored hash")
	}
	if middleware.CheckPassword(newHash, "Current-Pass1") {
		t.Fatal("the old password still verifies")
	}

	var live int64
	if err := s.db.Model(&db.UserSession{}).Where("user_id = ? AND revoked_at = ?", user.ID, time.Time{}).Count(&live).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if live != 0 {
		t.Fatalf("%d sessions survived the password change", live)
	}

	var history int64
	if err := s.db.Model(&db.PasswordHistory{}).Where("user_id = ?", user.ID).Count(&history).Error; err != nil {
		t.Fatalf("count history: %v", err)
	}
	if history != 1 {
		t.Fatalf("history rows = %d, want 1", history)
	}

	var audits int64
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "password_change").Count(&audits).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 1 {
		t.Fatalf("password_change audit rows = %d, want 1", audits)
	}
}

// TestChangePasswordRefusalsLeavePasswordIntact pins the three rejection paths
// (mismatch, complexity, recent reuse) and proves none of them rotate the hash.
func TestChangePasswordRefusalsLeavePasswordIntact(t *testing.T) {
	s := newPasswordChangeServer(t)
	user := seedPasswordUser(t, s, "pw-refuse", "Current-Pass1")
	original := storedPasswordHash(t, s, user.ID)

	cases := []struct {
		name                   string
		current, next, confirm string
		want                   int
	}{
		{"mismatched confirm", "Current-Pass1", "New-Pass123", "Different1", http.StatusBadRequest},
		{"too weak", "Current-Pass1", "short", "short", http.StatusBadRequest},
		{"wrong current", "not-the-password", "New-Pass123", "New-Pass123", http.StatusUnauthorized},
		{"unknown user", "Current-Pass1", "New-Pass123", "New-Pass123", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each case gets a fresh cooldown slot: a refusal consumes one by
			// design, so sharing a user would trip the rate limiter instead.
			s.pwdChangeTimesMu.Lock()
			delete(s.pwdChangeTimes, user.ID)
			s.pwdChangeTimesMu.Unlock()

			target := user
			if tc.name == "unknown user" {
				target.ID = 999999
			}
			if w := changePasswordForm(s, target, tc.current, tc.next, tc.confirm); w.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tc.want, w.Body.String())
			}
			if got := storedPasswordHash(t, s, user.ID); got != original {
				t.Fatalf("a refused change rotated the password")
			}
		})
	}
}

// TestChangePasswordRefusesReusedHistory proves the reuse check fires.
func TestChangePasswordRefusesReusedHistory(t *testing.T) {
	s := newPasswordChangeServer(t)
	user := seedPasswordUser(t, s, "pw-reuse", "Current-Pass1")

	oldHash, err := middleware.HashPassword("Reused-Pass123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := s.db.Create(&db.PasswordHistory{UserID: user.ID, PasswordHash: oldHash, CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("seed history: %v", err)
	}

	w := changePasswordForm(s, user, "Current-Pass1", "Reused-Pass123", "Reused-Pass123")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "reuse") {
		t.Fatalf("body = %s, want the reuse message", w.Body.String())
	}
}

// ── Partial failure honesty ─────────────────────────────────────────────────

// TestChangePasswordReportsSessionRevokeFailure proves the response tells the
// truth when the password was rotated but session revocation failed: the old
// code returned a bare 500, which reads as "nothing changed".
func TestChangePasswordReportsSessionRevokeFailure(t *testing.T) {
	s := newPasswordChangeServer(t)
	user := seedPasswordUser(t, s, "pw-revoke", "Current-Pass1")
	oldHash := storedPasswordHash(t, s, user.ID)

	// Remove the table so the revocation UPDATE cannot succeed.
	if err := s.db.Migrator().DropTable(&db.UserSession{}); err != nil {
		t.Fatalf("drop user_sessions: %v", err)
	}

	w := changePasswordForm(s, user, "Current-Pass1", "New-Pass123", "New-Pass123")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (%s)", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{`"password_changed":true`, `"sessions_revoked":false`, "could not be revoked"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %s, want it to contain %s", body, want)
		}
	}
	// The password really did change; the response must not imply otherwise.
	if got := storedPasswordHash(t, s, user.ID); got == oldHash {
		t.Fatal("expected the password to have been rotated before revocation ran")
	}
	var audit db.AuditLog
	if err := s.db.Where("action = ?", "password_change_sessions_revoked").First(&audit).Error; err != nil {
		t.Fatalf("no failure audit entry: %v", err)
	}
	if audit.Success {
		t.Fatal("the partial failure was audited as a success")
	}
	if audit.User != user.Username {
		t.Fatalf("audit user = %q, want %q", audit.User, user.Username)
	}
}
