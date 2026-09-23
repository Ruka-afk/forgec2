package server

import "testing"

// TestShouldLogActionSensitiveWrites proves the middleware path whitelist
// covers the real sensitive write routes — /users/ and /settings/ used to be
// missed because only the /api/* prefixes were listed.
func TestShouldLogActionSensitiveWrites(t *testing.T) {
	mustLog := []string{
		"/users/add",
		"/users/2/password",
		"/users/2/force-logout",
		"/users/2/sessions/9/revoke",
		"/users/2/sessions/revoke-all",
		"/settings/password",
		"/settings/malleable",
		"/settings/db/backup",
		"/settings/db/restore",
		"/settings/jwt/regenerate",
		"/settings/totp/enable",
		"/api/listeners",
		"/api/listeners/3/disable",
		"/api/api-keys",
		"/api/api-keys/abc/rotate",
		"/api/scripts",
		"/api/scripts/execute",
		"/api/automation/rules",
		"/api/webhooks",
		"/api/update-check/hot-update",
		"/extc2/slack",
		"/integrations",
		"/agents/x/kill",
		"/api/credentials/export",
	}
	for _, p := range mustLog {
		if !shouldLogAction(p) {
			t.Errorf("shouldLogAction(%q) = false, want true", p)
		}
	}

	// Unrelated / non-sensitive paths must stay out of the middleware audit path
	// (they may still have explicit LogAuditRecord calls elsewhere).
	mustNot := []string{
		"/dashboard",
		"/api/agents",
		"/favicon.ico",
		"/lang/set",
		"/api/saved-views",
	}
	for _, p := range mustNot {
		if shouldLogAction(p) {
			t.Errorf("shouldLogAction(%q) = true, want false", p)
		}
	}
}

func TestGetActionTypeSensitiveWrites(t *testing.T) {
	cases := map[string]string{
		"/users/add":             "user_management",
		"/settings/password":     "settings_change",
		"/api/listeners":         "listener_manage",
		"/api/api-keys/1/rotate": "api_key_manage",
		"/agents/a/kill":         "agent_action",
		"/something/else":        "api_access",
	}
	for path, want := range cases {
		if got := getActionType(path); got != want {
			t.Errorf("getActionType(%q) = %q, want %q", path, got, want)
		}
	}
}
