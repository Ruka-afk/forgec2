package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// aiRunCtx builds a context carrying the auth keys currentAIPrincipal reads, so
// permission resolution is exercised through the real path.
func aiRunCtx(username, role, method, path, id string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, nil)
	if id != "" {
		c.Params = gin.Params{{Key: "id", Value: id}}
	}
	if username != "" {
		c.Set("user", username)
	}
	if role != "" {
		c.Set("user_role", role)
	}
	return c, rec
}

func seedAIRun(t *testing.T, s *Server, id string, tenantID, ownerID uint, owner, status string) db.AIChatRun {
	t.Helper()
	run := db.AIChatRun{
		ID: id, TenantID: tenantID, OwnerID: ownerID, Owner: owner,
		SessionID: 1, Provider: "openai", Model: "gpt-4", Status: status,
		// The (tenant, owner, idempotency_key) triple is unique, so every seed
		// needs its own key.
		IdempotencyKey: "key-" + id,
		CreatedAt:      time.Now(),
	}
	if err := s.db.Create(&run).Error; err != nil {
		t.Fatalf("seed run: %v", err)
	}
	return run
}

func seedAIUser(t *testing.T, s *Server, username string, tenantID uint) db.User {
	t.Helper()
	user := db.User{Username: username, TenantID: tenantID, Role: db.RoleUser, IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

// ── Principal resolution ────────────────────────────────────────────────────

// TestCurrentAIPrincipalRejectsUnauthenticatedCallers proves the AI surface
// never opens to a caller the auth middleware did not identify.
func TestCurrentAIPrincipalRejectsUnauthenticatedCallers(t *testing.T) {
	s := setupAIRunStorageTest(t)

	for name, setUser := range map[string]bool{
		"no user key":  false,
		"empty user":   true,
		"unknown user": true,
	} {
		t.Run(name, func(t *testing.T) {
			var c *gin.Context
			switch setUser {
			case false:
				c, _ = aiRunCtx("", "admin", http.MethodGet, "/", "")
			case true:
				if name == "empty user" {
					c, _ = aiRunCtx("", "admin", http.MethodGet, "/", "")
				} else {
					c, _ = aiRunCtx("ghost-user", "admin", http.MethodGet, "/", "")
				}
			}
			if _, ok := s.currentAIPrincipal(c); ok {
				t.Fatalf("%s resolved to an AI principal", name)
			}
		})
	}
}

// TestCurrentAIPrincipalDeniesRoleWithoutAIUse proves a role that lacks
// ai.use is refused even though it is a perfectly valid authenticated user.
func TestCurrentAIPrincipalDeniesRoleWithoutAIUse(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "reader", 1)

	c, _ := aiRunCtx("reader", "auditor", http.MethodGet, "/", "")
	if _, ok := s.currentAIPrincipal(c); ok {
		t.Fatal("a role without ai.use resolved to an AI principal")
	}
}

// TestCurrentAIPrincipalAllowsValidUser builds the resolution case: the stored
// user is matched and its tenant is carried onto the principal.
func TestCurrentAIPrincipalAllowsValidUser(t *testing.T) {
	s := setupAIRunStorageTest(t)
	user := seedAIUser(t, s, "worker", 5)

	c, _ := aiRunCtx("worker", db.RoleUser, http.MethodGet, "/", "")
	p, ok := s.currentAIPrincipal(c)
	if !ok {
		t.Fatal("a valid user holding ai.use was refused")
	}
	if p.UserID != user.ID || p.TenantID != 5 || p.Username != "worker" {
		t.Fatalf("principal = %+v", p)
	}

	// The documented fallback: an unannotated role is read from the DB row.
	c2, _ := aiRunCtx("worker", "", http.MethodGet, "/", "")
	p2, ok := s.currentAIPrincipal(c2)
	if !ok {
		t.Fatal("a caller with no user_role was refused instead of using the stored role")
	}
	if p2.Role != db.RoleUser {
		t.Fatalf("role = %q, want the stored %q", p2.Role, db.RoleUser)
	}
}

// TestCurrentAIPrincipalIgnoresForgeries proves the identity always comes from
// the DB row: a claimed context role can widen the permission check, but it
// cannot invent an identity or a tenant.
func TestCurrentAIPrincipalIgnoresForgeries(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "forger", 1)

	// The context role is trusted for the permission check (the auth middleware
	// is what sets it), but the username and tenant always come from the row.
	c, _ := aiRunCtx("forger", db.RoleAdmin, http.MethodGet, "/", "")
	p, ok := s.currentAIPrincipal(c)
	if !ok {
		t.Fatal("a context-claimed admin was refused")
	}
	if p.Username != "forger" || p.TenantID != 1 {
		t.Fatalf("principal = %+v, want the DB identity", p)
	}

	// Without the claimed role the stored role governs.
	c2, _ := aiRunCtx("forger", "", http.MethodGet, "/", "")
	if _, ok := s.currentAIPrincipal(c2); !ok {
		t.Fatal("a valid user with no context role was refused")
	}
}

// TestHasPermissionDirect pins the pure permission check, including that the
// base user role carries ai.use by design.
func TestHasPermissionDirect(t *testing.T) {
	s := setupAIRunStorageTest(t)
	admin := aiPrincipal{Role: db.RoleAdmin}
	if !admin.hasPermission(s.db, db.PermAIUse) {
		t.Fatal("admin lacks ai.use")
	}
	if !admin.hasPermission(s.db, "totally.unrelated") {
		t.Fatal("admin was denied an arbitrary permission")
	}
	if !(aiPrincipal{Role: db.RoleUser}).hasPermission(s.db, db.PermAIUse) {
		t.Fatal("the base user role does not carry ai.use")
	}

	// A custom DB-backed role can carry the permission.
	if err := s.db.AutoMigrate(&db.CustomRole{}); err != nil {
		t.Fatalf("migrate custom_roles: %v", err)
	}
	if err := s.db.Create(&db.CustomRole{Name: "ai-operator", Permissions: `["ai.use"]`}).Error; err != nil {
		t.Fatalf("seed custom role: %v", err)
	}
	custom := aiPrincipal{Role: "ai-operator"}
	if !custom.hasPermission(s.db, db.PermAIUse) {
		t.Fatal("a custom role holding ai.use was denied")
	}
	if custom.hasPermission(s.db, "users.write") {
		t.Fatal("a custom role was granted a permission it does not hold")
	}
	// An unknown role is refused.
	if (aiPrincipal{Role: "no-such-role"}).hasPermission(s.db, db.PermAIUse) {
		t.Fatal("an unknown role was granted ai.use")
	}
}

// ── Per-run authorization ────────────────────────────────────────────────────

// TestFindAuthorizedAIRunIsolatesByOwnerAndTenant is the core regression: a run
// belongs to one owner in one tenant, and neither another user nor another
// tenant may read or cancel it.
func TestFindAuthorizedAIRunIsolatesByOwnerAndTenant(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "alice", 1)
	seedAIUser(t, s, "bob", 1)
	seedAIUser(t, s, "carol", 2)
	seedAIRun(t, s, "run-alice", 1, 1, "alice", "running")
	seedAIRun(t, s, "run-carol", 2, 3, "carol", "running")

	// The owner reads their own run.
	c, w := aiRunCtx("alice", db.RoleAdmin, http.MethodGet, "/", "run-alice")
	run, _, ok := s.findAuthorizedAIRun(c)
	if !ok {
		t.Fatalf("the owner could not load their own run: %d", w.Code)
	}
	if run.ID != "run-alice" {
		t.Fatalf("loaded %q, want run-alice", run.ID)
	}

	// Another user in the same tenant gets 404, not the run.
	c2, w2 := aiRunCtx("bob", db.RoleAdmin, http.MethodGet, "/", "run-alice")
	if _, _, ok := s.findAuthorizedAIRun(c2); ok {
		t.Fatal("a different user in the same tenant loaded another user's run")
	}
	if w2.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w2.Code)
	}

	// A user in another tenant gets 404 too.
	c3, w3 := aiRunCtx("carol", db.RoleAdmin, http.MethodGet, "/", "run-alice")
	if _, _, ok := s.findAuthorizedAIRun(c3); ok {
		t.Fatal("a cross-tenant caller loaded another tenant's run")
	}
	if w3.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w3.Code)
	}

	// An unknown id is 404.
	c4, w4 := aiRunCtx("alice", db.RoleAdmin, http.MethodGet, "/", "run-missing")
	if _, _, ok := s.findAuthorizedAIRun(c4); ok {
		t.Fatal("an unknown run id resolved")
	}
	if w4.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w4.Code)
	}
}

// TestHandleAIRunsGetHonoursAuthorization proves the HTTP layer applies the same
// isolation as the finder.
func TestHandleAIRunsGetHonoursAuthorization(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "alice", 1)
	seedAIUser(t, s, "bob", 1)
	seedAIRun(t, s, "run-alice", 1, 1, "alice", "running")

	c, w := aiRunCtx("alice", db.RoleAdmin, http.MethodGet, "/", "run-alice")
	s.handleAIRunsGet(c)
	if w.Code != http.StatusOK {
		t.Fatalf("owner: status = %d body = %s", w.Code, w.Body.String())
	}

	c2, w2 := aiRunCtx("bob", db.RoleAdmin, http.MethodGet, "/", "run-alice")
	s.handleAIRunsGet(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("other user: status = %d, want 404", w2.Code)
	}
	if strings.Contains(w2.Body.String(), "gpt-4") {
		t.Fatalf("the response leaked run details: %s", w2.Body.String())
	}

	// An unauthenticated caller is refused at the permission gate.
	c3, w3 := aiRunCtx("ghost", "", http.MethodGet, "/", "run-alice")
	s.handleAIRunsGet(c3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("anonymous: status = %d, want 403", w3.Code)
	}
}

// TestHandleAIRunsListIsScopedToOwnerAndTenant proves the list never widens to
// other owners or other tenants.
func TestHandleAIRunsListIsScopedToOwnerAndTenant(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "alice", 1)
	seedAIUser(t, s, "bob", 1)
	seedAIUser(t, s, "carol", 2)
	seedAIRun(t, s, "r-alice-1", 1, 1, "alice", "running")
	seedAIRun(t, s, "r-alice-2", 1, 1, "alice", "completed")
	seedAIRun(t, s, "r-bob", 1, 2, "bob", "running")
	seedAIRun(t, s, "r-carol", 2, 3, "carol", "running")

	c, w := aiRunCtx("alice", db.RoleAdmin, http.MethodGet, "/", "")
	s.handleAIRunsList(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "r-alice-1") || !strings.Contains(body, "r-alice-2") {
		t.Fatalf("own runs missing: %s", body)
	}
	for _, foreign := range []string{"r-bob", "r-carol"} {
		if strings.Contains(body, foreign) {
			t.Fatalf("%s leaked into another user's list", foreign)
		}
	}
}

// TestHandleAIRunsListFilters proves the query filters still apply on top of the
// scope rather than replacing it.
func TestHandleAIRunsListFilters(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "alice", 1)
	seedAIRun(t, s, "r-active", 1, 1, "alice", aiRunStatusRunning)
	seedAIRun(t, s, "r-done", 1, 1, "alice", aiRunStatusCompleted)
	seedAIRun(t, s, "r-queued", 1, 1, "alice", aiRunStatusQueued)

	c, w := aiRunCtx("alice", db.RoleAdmin, http.MethodGet, "/?status=active", "")
	s.handleAIRunsList(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "r-active") || !strings.Contains(body, "r-queued") {
		t.Fatalf("active runs missing: %s", body)
	}
	if strings.Contains(body, "r-done") {
		t.Fatalf("a terminal run appeared in the active list: %s", body)
	}

	// A session filter narrows further rather than widening.
	c2, w2 := aiRunCtx("alice", db.RoleAdmin, http.MethodGet, "/?session_id=999", "")
	s.handleAIRunsList(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("status = %d", w2.Code)
	}
	if strings.Contains(w2.Body.String(), "r-active") {
		t.Fatalf("a session filter leaked other sessions' runs: %s", w2.Body.String())
	}
}

// TestHandleAIRunsListRefusesWithoutPermission proves the permission gate is the
// first thing that runs.
func TestHandleAIRunsListRefusesWithoutPermission(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "reader", 1)
	seedAIRun(t, s, "r-x", 1, 1, "reader", "running")

	c, w := aiRunCtx("reader", "auditor", http.MethodGet, "/", "")
	s.handleAIRunsList(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

// ── Run broker ──────────────────────────────────────────────────────────────

// TestAIRunBrokerLifecycle pins subscribe/publish/finish/cancel, including that
// a cancelled run reports the cancellation to its subscribers.
func TestAIRunBrokerLifecycle(t *testing.T) {
	b := newAIRunBroker()
	const runID = "run-1"

	ch, unsubscribe := b.subscribe(runID)
	defer unsubscribe()

	b.publish(runID, aiLiveEvent{Sequence: 1, Type: "started", Payload: "{}"}, false)
	select {
	case ev := <-ch:
		if ev.Sequence != 1 || ev.Type != "started" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("a published event never reached the subscriber")
	}

	// Cancelling an unregistered run is a no-op that reports false.
	if b.cancel("run-unknown") {
		t.Fatal("cancelling an unknown run reported success")
	}

	// finish() unregisters the run's cancel, so cancel() then reports false.
	b.registerCancel(runID, func() {})
	b.finish(runID)
	if b.cancel(runID) {
		t.Fatal("a finished run was still cancellable")
	}
}

// TestAIRunBrokerCancelDelivers proves a registered cancel func runs and
// cancel() reports that it did. The broker itself publishes no event: the
// cancelled run's worker is what emits the terminal event.
func TestAIRunBrokerCancelDelivers(t *testing.T) {
	b := newAIRunBroker()
	const runID = "run-2"

	called := make(chan struct{})
	b.registerCancel(runID, func() { close(called) })
	if !b.cancel(runID) {
		t.Fatal("cancel reported false for a registered run")
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("the registered cancel func was never invoked")
	}

	// A second cancel still invokes the stored func; cancel is idempotent at
	// the registry level.
	b.registerCancel(runID, func() {})
	if !b.cancel(runID) {
		t.Fatal("a re-registered cancel was not honoured")
	}
}

// TestAIRunBrokerUnsubscribeStopsDelivery proves closing the subscription stops
// further delivery without panicking.
func TestAIRunBrokerUnsubscribeStopsDelivery(t *testing.T) {
	b := newAIRunBroker()
	const runID = "run-3"

	ch, unsubscribe := b.subscribe(runID)
	unsubscribe()
	// A late publish must not block or panic on a closed channel.
	b.publish(runID, aiLiveEvent{Sequence: 1, Type: "late"}, false)
	select {
	case ev, ok := <-ch:
		if ok {
			t.Fatalf("event %+v was delivered after unsubscribe", ev)
		}
	default:
	}
}

// ── Profile test rate limit ─────────────────────────────────────────────────

// TestAIProfileTestAllowedLimits pins the per-key limiter: the sixth call in a
// minute is refused and other keys are unaffected.
func TestAIProfileTestAllowedLimits(t *testing.T) {
	const key = "profile-1"
	// Reset the shared limiter so the assertion is deterministic across tests.
	aiProfileTestLimiter.Lock()
	delete(aiProfileTestLimiter.hits, key)
	aiProfileTestLimiter.Unlock()

	for i := 0; i < 5; i++ {
		if !aiProfileTestAllowed(key) {
			t.Fatalf("call %d was refused, want the first five allowed", i+1)
		}
	}
	if aiProfileTestAllowed(key) {
		t.Fatal("the sixth call in a minute was allowed")
	}
	// Another key keeps its own budget.
	if !aiProfileTestAllowed("profile-2") {
		t.Fatal("the limiter blocked an unrelated key")
	}
}

// TestAIProfileTestAllowedWindowExpiry proves the one-minute window prunes old
// hits instead of counting them forever.
func TestAIProfileTestAllowedWindowExpiry(t *testing.T) {
	const key = "profile-3"
	aiProfileTestLimiter.Lock()
	aiProfileTestLimiter.hits[key] = []time.Time{time.Now().Add(-2 * time.Minute)}
	aiProfileTestLimiter.Unlock()

	if !aiProfileTestAllowed(key) {
		t.Fatal("stale hits were counted against a fresh call")
	}
	aiProfileTestLimiter.Lock()
	n := len(aiProfileTestLimiter.hits[key])
	aiProfileTestLimiter.Unlock()
	if n != 1 {
		t.Fatalf("%d hits retained, want the stale entry pruned", n)
	}
}
