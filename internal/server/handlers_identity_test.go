package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildConsentAuthorizeURL(t *testing.T) {
	sess := &consentSession{
		Tenant:      "contoso.onmicrosoft.com",
		ClientID:    "11111111-1111-1111-1111-111111111111",
		RedirectURI: "https://c2.lab/phishing/oauth/callback",
		Scope:       "https://graph.microsoft.com/.default offline_access",
		State:       "abc123",
	}
	got := buildConsentAuthorizeURL(sess)
	if !strings.HasPrefix(got, "https://login.microsoftonline.com/contoso.onmicrosoft.com/oauth2/v2.0/authorize?") {
		t.Fatalf("prefix: %s", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("client_id") != sess.ClientID || q.Get("response_type") != "code" || q.Get("state") != "abc123" {
		t.Fatalf("query: %v", q)
	}
	if q.Get("redirect_uri") != sess.RedirectURI {
		t.Fatalf("redirect %q", q.Get("redirect_uri"))
	}
}

func TestOAuthCallbackCapturesCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := "state-test-1"
	consentMu.Lock()
	consentSess[state] = &consentSession{State: state, Tenant: "organizations"}
	consentMu.Unlock()
	t.Cleanup(func() {
		consentMu.Lock()
		delete(consentSess, state)
		consentMu.Unlock()
	})

	s := &Server{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/phishing/oauth/callback?code=AUTHCODE&state="+state, nil)
	s.handleOAuthCallback(c)
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	consentMu.Lock()
	got := consentSess[state].Code
	consentMu.Unlock()
	if got != "AUTHCODE" {
		t.Fatalf("code %q", got)
	}
}

func TestOAuthCallbackUnknownState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/phishing/oauth/callback?code=x&state=missing", nil)
	s.handleOAuthCallback(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
}
