package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// newRolesTestServer builds a server with a bare DB (no JWT/CSRF setup, which
// role management does not touch).
func newRolesTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}
}

// roleCtx builds an authenticated context with the given role and a JSON body.
func roleCtx(role, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "role-tester")
	c.Set("user_role", role)
	return c, w
}

// TestRolesCreateRequiresAdmin proves a non-admin operator cannot mint a
// custom role: roles carry permission sets, so this is a privilege-escalation
// surface.
func TestRolesCreateRequiresAdmin(t *testing.T) {
	s := newRolesTestServer(t)

	c, w := roleCtx("operator", `{"name":"sneaky","permissions":["agents.read"]}`)
	s.handleRolesCreate(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 for a non-admin create", w.Code)
	}
	var n int64
	if err := s.db.Model(&db.CustomRole{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("non-admin create landed %d role(s)", n)
	}
}

// TestRolesCreateRejectsMissingName proves the name guard.
func TestRolesCreateRejectsMissingName(t *testing.T) {
	s := newRolesTestServer(t)

	c, w := roleCtx("admin", `{"name":"","permissions":[]}`)
	s.handleRolesCreate(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 for a missing name", w.Code, w.Body.String())
	}
}

// TestRolesCreateAndListRoundTrip proves a created role is persisted with its
// permission set and the list endpoint returns permissions as a decoded array
// (the DB stores them as JSON text; leaking the raw string would break every
// consumer that iterates them).
func TestRolesCreateAndListRoundTrip(t *testing.T) {
	s := newRolesTestServer(t)

	c, w := roleCtx("admin", `{"name":"analyst","description":"read-only","permissions":["agents.read","tasks.read"]}`)
	s.handleRolesCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	var stored db.CustomRole
	if err := s.db.Where("name = ?", "analyst").First(&stored).Error; err != nil {
		t.Fatalf("role not persisted: %v", err)
	}
	if stored.Permissions == "" || stored.Permissions == "[]" {
		t.Fatalf("permissions not persisted: %q", stored.Permissions)
	}

	c2, w2 := roleCtx("admin", "")
	c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleRolesList(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("list status=%d", w2.Code)
	}
	var payload struct {
		Data []struct {
			Name        string   `json:"name"`
			Permissions []string `json:"permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Data) != 1 || payload.Data[0].Name != "analyst" {
		t.Fatalf("list returned %+v, want the analyst role", payload.Data)
	}
	if len(payload.Data[0].Permissions) != 2 {
		t.Fatalf("permissions did not round-trip as an array: %+v", payload.Data[0].Permissions)
	}
}

// TestRolesUpdateNotFound proves a missing role is a 404 (not a silent create)
// and that a non-admin is refused before any lookup happens.
func TestRolesUpdateNotFound(t *testing.T) {
	s := newRolesTestServer(t)

	c, w := roleCtx("admin", `{"description":"x"}`)
	c.Params = gin.Params{{Key: "id", Value: "999999"}}
	s.handleRolesUpdate(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for a missing role", w.Code)
	}

	c2, w2 := roleCtx("operator", `{"description":"x"}`)
	c2.Params = gin.Params{{Key: "id", Value: "1"}}
	s.handleRolesUpdate(c2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 for a non-admin update", w2.Code)
	}
}

// TestRolesUpdateKeepsNameWhenOmitted proves the partial-update contract: an
// empty name field preserves the stored one instead of blanking it.
func TestRolesUpdateKeepsNameWhenOmitted(t *testing.T) {
	s := newRolesTestServer(t)
	seed := db.CustomRole{Name: "keepme", Permissions: `["agents.read"]`}
	if err := s.db.Create(&seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, w := roleCtx("admin", `{"description":"updated only"}`)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	s.handleRolesUpdate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var reloaded db.CustomRole
	if err := s.db.First(&reloaded, seed.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Name != "keepme" {
		t.Fatalf("name was blanked by a partial update: %q", reloaded.Name)
	}
	if reloaded.Description != "updated only" {
		t.Fatalf("description not updated: %q", reloaded.Description)
	}
}

// TestRolesDeleteRefusesBuiltin proves the builtin admin/user roles cannot be
// deleted through the API: losing them would lock every operator out.
func TestRolesDeleteRefusesBuiltin(t *testing.T) {
	s := newRolesTestServer(t)
	for _, name := range []string{"admin", "user"} {
		if err := s.db.Create(&db.CustomRole{Name: name, Permissions: "[]"}).Error; err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	c, w := roleCtx("admin", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	s.handleRolesDelete(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 when deleting the builtin admin role", w.Code)
	}
	var n int64
	if err := s.db.Model(&db.CustomRole{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Fatalf("builtin roles were modified: %d remain", n)
	}
}

// TestRolesDeleteCustomAndAdminGate proves a custom role deletes cleanly and a
// non-admin is refused first.
func TestRolesDeleteCustomAndAdminGate(t *testing.T) {
	s := newRolesTestServer(t)
	if err := s.db.Create(&db.CustomRole{Name: "doomed", Permissions: "[]"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, w := roleCtx("admin", "")
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	s.handleRolesDelete(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 deleting a custom role", w.Code)
	}
	var n int64
	if err := s.db.Model(&db.CustomRole{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("custom role survived delete: %d remain", n)
	}

	c2, w2 := roleCtx("operator", "")
	c2.Params = gin.Params{{Key: "id", Value: "1"}}
	s.handleRolesDelete(c2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 for a non-admin delete", w2.Code)
	}
}

// TestPermissionMarshalRoundTrip pins the storage format helpers: a nil slice
// must marshal to a valid empty array, and malformed stored JSON must parse
// back to an empty set rather than erroring into a 500 on list.
func TestPermissionMarshalRoundTrip(t *testing.T) {
	if got := marshalPermissions(nil); got != "[]" {
		t.Fatalf("marshalPermissions(nil) = %q, want []", got)
	}
	if got := marshalPermissions([]string{"a", "b"}); got != `["a","b"]` {
		t.Fatalf("marshalPermissions = %q", got)
	}
	if got := parsePermissions(`["x","y"]`); len(got) != 2 || got[0] != "x" {
		t.Fatalf("parsePermissions = %+v", got)
	}
	if got := parsePermissions("not json"); len(got) != 0 {
		t.Fatalf("parsePermissions(malformed) = %+v, want empty", got)
	}
	if got := parsePermissions(""); len(got) != 0 {
		t.Fatalf("parsePermissions(empty) = %+v, want empty", got)
	}
}
