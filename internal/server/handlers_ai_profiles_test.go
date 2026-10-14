package server

import (
	"fmt"

	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Credential containment ──────────────────────────────────────────────────

// TestAIProfileViewNeverCarriesTheAPIKey is the structural guarantee behind the
// HasAPIKey boolean: no field on the view type can hold key material, whatever
// a caller does with the profile row.
func TestAIProfileViewNeverCarriesTheAPIKey(t *testing.T) {
	viewType := reflect.TypeOf(aiProfileView{})
	for i := 0; i < viewType.NumField(); i++ {
		field := viewType.Field(i)
		switch strings.ToLower(field.Tag.Get("json")) {
		case "api_key", "apikey":
			t.Fatalf("aiProfileView exposes %s", field.Name)
		}
		if field.Name == "APIKey" {
			t.Fatalf("aiProfileView embeds the raw field %s", field.Name)
		}
	}

	profile := db.AIProviderProfile{
		Name: "prod", Provider: "openai", Model: "gpt-4",
		APIKey: "sk-super-secret-key-material",
	}
	view := makeAIProfileView(profile, true)
	if !view.HasAPIKey {
		t.Fatal("a stored key is not reported as present")
	}
	if strings.Contains(reflect.ValueOf(view).String(), "sk-super-secret-key-material") {
		t.Fatal("the view carries the key material")
	}

	// A blank key reports absence rather than a truthy default.
	if makeAIProfileView(db.AIProviderProfile{}, true).HasAPIKey {
		t.Fatal("an absent key reported itself as present")
	}
	if makeAIProfileView(db.AIProviderProfile{APIKey: "   "}, true).HasAPIKey {
		t.Fatal("a whitespace-only key reported itself as present")
	}
}

// TestAIProfileViewDiagnosticsAreOptIn pins that health fields only appear when
// the caller asked for diagnostics.
func TestAIProfileViewDiagnosticsAreOptIn(t *testing.T) {
	profile := db.AIProviderProfile{
		Name: "prod", Provider: "openai", Model: "gpt-4",
		LastHealthStatus: "error", LastHealthError: "401 unauthorized",
		LastHealthLatency: 42,
	}
	if got := makeAIProfileView(profile, false); got.LastHealthStatus != "" || got.LastHealthError != "" || got.LastHealthLatencyMS != 0 {
		t.Fatalf("diagnostics were populated without the opt-in: %+v", got)
	}
	got := makeAIProfileView(profile, true)
	if got.LastHealthStatus != "error" || got.LastHealthError != "401 unauthorized" || got.LastHealthLatencyMS != 42 {
		t.Fatalf("diagnostics with the opt-in = %+v", got)
	}
}

// ── Input normalisation ─────────────────────────────────────────────────────

// TestNormalizeAIProfileInput pins the shared validation every profile write
// funnels through, including the SSRF guard on the endpoint.
func TestNormalizeAIProfileInput(t *testing.T) {
	s := setupAIRunStorageTest(t)
	// A local model is legitimate, but only when the operator allowlisted it.
	s.cfg.AI.AllowedEndpoints = []string{"127.0.0.1:11434"}

	valid := []aiProfileInput{
		{Name: "prod", Provider: "openai", Model: "gpt-4"},
		{Name: "  spaced  ", Provider: "  OpenAI ", Model: " gpt-4 "},
		{Name: "local", Provider: "ollama", Model: "llama3", Endpoint: "http://127.0.0.1:11434"},
		{Name: "local", Provider: "deepseek", Model: "deepseek-chat"},
	}
	for i, in := range valid {
		if _, err := s.normalizeAIProfileInput(in); err != nil {
			t.Fatalf("valid input %d rejected: %v", i, err)
		}
	}

	// The name and provider are normalised.
	out, err := s.normalizeAIProfileInput(aiProfileInput{Name: "  Prod  ", Provider: " OpenAI ", Model: " m "})
	if err != nil {
		t.Fatalf("normalise: %v", err)
	}
	if out.Name != "Prod" || out.Provider != "openai" || out.Model != "m" {
		t.Fatalf("normalised = %+v", out)
	}
	if out.ContextLimit == 0 || out.OutputLimit == 0 {
		t.Fatal("token limits were left unset")
	}

	invalid := []struct {
		name string
		in   aiProfileInput
	}{
		{"missing name", aiProfileInput{Provider: "openai", Model: "m"}},
		{"missing model", aiProfileInput{Name: "n", Provider: "openai"}},
		{"name too long", aiProfileInput{Name: strings.Repeat("a", 121), Provider: "openai", Model: "m"}},
		{"model too long", aiProfileInput{Name: "n", Provider: "openai", Model: strings.Repeat("m", 201)}},
		{"unknown provider", aiProfileInput{Name: "n", Provider: "not-a-provider", Model: "m"}},
		{"needs explicit endpoint", aiProfileInput{Name: "n", Provider: "custom", Model: "m"}},
		{"endpoint too long", aiProfileInput{Name: "n", Provider: "openai", Model: "m", Endpoint: "https://x/" + strings.Repeat("a", 2100)}},
		{"loopback endpoint not allowlisted", aiProfileInput{Name: "n", Provider: "ollama", Model: "m", Endpoint: "http://127.0.0.1:11434"}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			// The loopback case is only invalid when nothing is allowlisted.
			saved := s.cfg.AI.AllowedEndpoints
			if tc.name == "loopback endpoint not allowlisted" {
				s.cfg.AI.AllowedEndpoints = nil
			}
			if _, err := s.normalizeAIProfileInput(tc.in); err == nil {
				t.Fatal("an invalid profile was accepted")
			}
			s.cfg.AI.AllowedEndpoints = saved
		})
	}
}

// ── Handler gating ──────────────────────────────────────────────────────────

// TestAIProfileHandlersRequireAIUse proves the profile endpoints go through the
// principal check first: an unauthenticated caller cannot enumerate profiles.
func TestAIProfileHandlersRequireAIUse(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.AIProviderProfile{Name: "mine", TenantID: 1, Provider: "openai", Model: "m"})

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	// No user at all: the list and every id-taking endpoint must refuse.
	for _, invoke := range []func(*Server, *gin.Context){
		(*Server).handleAIProfilesList,
		(*Server).handleAIProfileUpdate,
		(*Server).handleAIProfileDelete,
		(*Server).handleAIProfileTest,
	} {
		w := httptest.NewRecorder()
		c2, _ := gin.CreateTestContext(w)
		c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c2.Params = gin.Params{{Key: "id", Value: "1"}}
		invoke(s, c2)
		if w.Code != http.StatusForbidden {
			t.Fatalf("anonymous status = %d, want 403", w.Code)
		}
	}
	_ = c
}

// TestLoadTenantAIProfileIsTenantScoped proves the profile loader refuses a
// profile belonging to another tenant.
func TestLoadTenantAIProfileIsTenantScoped(t *testing.T) {
	s := setupAIRunStorageTest(t)
	mine := db.AIProviderProfile{Name: "mine", TenantID: 1, Provider: "openai", Model: "m", APIKey: "k1"}
	theirs := db.AIProviderProfile{Name: "theirs", TenantID: 2, Provider: "openai", Model: "m", APIKey: "k2"}
	if err := s.db.Create(&mine).Error; err != nil {
		t.Fatalf("seed mine: %v", err)
	}
	if err := s.db.Create(&theirs).Error; err != nil {
		t.Fatalf("seed theirs: %v", err)
	}

	own := aiPrincipal{UserID: 1, Username: "alice", TenantID: 1, Role: db.RoleUser}
	foreign := aiPrincipal{UserID: 2, Username: "carol", TenantID: 2, Role: db.RoleUser}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: itoaSmall(mine.ID)}}
	profile, ok := s.loadTenantAIProfile(c, own)
	if !ok {
		t.Fatal("a caller could not load a profile in their own tenant")
	}
	if profile.Name != "mine" {
		t.Fatalf("loaded %q, want mine", profile.Name)
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c2.Params = gin.Params{{Key: "id", Value: itoaSmall(mine.ID)}}
	if _, ok := s.loadTenantAIProfile(c2, foreign); ok {
		t.Fatal("a caller loaded a profile belonging to another tenant")
	}
}

// TestAIProfileHandlersHideForeignProfiles proves the list is tenant-scoped.
func TestAIProfileHandlersHideForeignProfiles(t *testing.T) {
	s := setupAIRunStorageTest(t)
	seedAIUser(t, s, "root", 1)
	s.db.Create(&db.AIProviderProfile{Name: "own-profile", TenantID: 1, Provider: "openai", Model: "m", APIKey: "k-own"})
	s.db.Create(&db.AIProviderProfile{Name: "foreign-profile", TenantID: 2, Provider: "openai", Model: "m", APIKey: "k-foreign"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user", "root")
	c.Set("user_role", db.RoleAdmin)
	s.handleAIProfilesList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "foreign-profile") {
		t.Fatalf("another tenant's profile leaked: %s", body)
	}
	if !strings.Contains(body, "own-profile") {
		t.Fatalf("own profile missing: %s", body)
	}
	if strings.Contains(body, "k-own") || strings.Contains(body, "k-foreign") {
		t.Fatalf("API key material leaked into the list: %s", body)
	}
}

func itoaSmall(v uint) string {
	return fmt.Sprint(v)
}
