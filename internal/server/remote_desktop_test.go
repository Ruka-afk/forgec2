package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRDAPIFrameCrossTenant404 proves screen frames honor tenant scope
// (previously unscoped: any authenticated operator could read another
// tenant's latest RD frame by agent ID).
func TestRDAPIFrameCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rd-x-tenant", 2)
	frameBufferMu.Lock()
	frameBuffers["rd-x-tenant"] = []byte{0x89, 0x50, 0x4E, 0x47}
	frameBufferMu.Unlock()
	t.Cleanup(func() {
		frameBufferMu.Lock()
		delete(frameBuffers, "rd-x-tenant")
		frameBufferMu.Unlock()
	})

	c, w := tenantScopedAdminContext(s, t, "rd-t-viewer", 1)
	c.Params = gin.Params{{Key: "id", Value: "rd-x-tenant"}}
	s.handleRDAPIGetFrame(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant frame: got %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

// TestRDAPIFrameSameTenantOK proves same-tenant operators still get frames.
func TestRDAPIFrameSameTenantOK(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rd-same", 1)
	frameBufferMu.Lock()
	frameBuffers["rd-same"] = []byte{0x89, 0x50}
	frameBufferMu.Unlock()
	t.Cleanup(func() {
		frameBufferMu.Lock()
		delete(frameBuffers, "rd-same")
		frameBufferMu.Unlock()
	})

	c, w := tenantScopedAdminContext(s, t, "rd-s-ok", 1)
	c.Params = gin.Params{{Key: "id", Value: "rd-same"}}
	s.handleRDAPIGetFrame(c)
	if w.Code != http.StatusOK {
		t.Fatalf("same-tenant frame: got %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// TestRDAPIFrameViewerForbidden proves viewer role is rejected (requireOperator).
func TestRDAPIFrameViewerForbidden(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rd-viewer", 1)

	c, w := tenantScopedAdminContext(s, t, "rd-v", 1)
	c.Set("user_role", "viewer")
	c.Params = gin.Params{{Key: "id", Value: "rd-viewer"}}
	s.handleRDAPIGetFrame(c)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer frame: got %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// TestRDAPIFrameMissingAgent404 proves unknown agents 404 (tenant helper path).
func TestRDAPIFrameMissingAgent404(t *testing.T) {
	s := mustTenantServer(t)

	c, w := tenantScopedAdminContext(s, t, "rd-miss", 1)
	c.Params = gin.Params{{Key: "id", Value: "rd-no-such-agent"}}
	s.handleRDAPIGetFrame(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing agent frame: got %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

// TestRDWebSocketEmptyAgentID400 proves the handler rejects missing agent_id
// before attempting a WebSocket upgrade.
func TestRDWebSocketEmptyAgentID400(t *testing.T) {
	s := mustTenantServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/ws/remote-desktop", nil)
	c.Set("user", "rd-ws")
	c.Set("user_role", "admin")
	c.Set("user_id", uint(1))

	s.handleRDWebSocket(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty agent_id: got %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

// TestRDWebSocketCrossTenant404 proves tenant scope is checked before upgrade
// (previously upgraded first, then only checked non-empty query params).
func TestRDWebSocketCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rd-ws-x", 2)

	c, w := tenantScopedAdminContext(s, t, "rd-ws-t", 1)
	c.Set("user_id", uint(1))
	c.Request, _ = http.NewRequest(http.MethodGet, "/ws/remote-desktop?agent_id=rd-ws-x", nil)

	s.handleRDWebSocket(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant ws: got %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

// TestRDAPIScreenshotCrossTenant404 proves screenshot tasks cannot be issued
// against another tenant's agent.
func TestRDAPIScreenshotCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rd-shot-x", 2)

	c, w := tenantScopedAdminContext(s, t, "rd-shot-t", 1)
	c.Params = gin.Params{{Key: "id", Value: "rd-shot-x"}}
	s.handleRDAPIScreenshot(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant screenshot: got %d, want 404; body=%s", w.Code, w.Body.String())
	}
}
