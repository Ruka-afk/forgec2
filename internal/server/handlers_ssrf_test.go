package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidateExternalURLRejectsInternalTargets(t *testing.T) {
	cases := []string{
		"http://127.0.0.1:8080/x",
		"https://localhost/x",
		"http://10.0.0.1/",
		"http://172.16.0.5/",
		"http://192.168.1.1/bof.o",
		"http://[::1]/",
		"http://169.254.169.254/latest/meta-data/", // cloud metadata SSRF probe
		"ftp://example.com/a.o",
		"http://0.0.0.0/a.o",
		"http:///nohost",
		"file:///etc/passwd",
	}
	for _, u := range cases {
		if err := validateExternalURL(u); err == nil {
			t.Errorf("validateExternalURL(%q) should fail", u)
		}
	}
}

func TestValidateExternalURLAcceptsPublic(t *testing.T) {
	for _, u := range []string{
		"https://8.8.8.8/",          // public literal IP
		"http://example.com/bof.o",  // public hostname (may need DNS)
		"https://1.1.1.1/x",         // public literal
	} {
		if err := validateExternalURL(u); err != nil {
			t.Errorf("validateExternalURL(%q) should pass, got %v", u, err)
		}
	}
}

// TestSSRFSafeClientRejectsRedirectToInternal ensures the CheckRedirect hook
// refuses a redirect that pivots to a loopback address.
func TestSSRFSafeClientRejectsRedirectToInternal(t *testing.T) {
	c := ssrfSafeClient(nil)
	err := c.CheckRedirect(
		&http.Request{URL: mustURL("http://127.0.0.1:53/internal")},
		[]*http.Request{&http.Request{}},
	)
	if err == nil {
		t.Fatal("redirect to loopback must be rejected")
	}

	err = c.CheckRedirect(
		&http.Request{URL: mustURL("https://8.8.8.8/public")},
		[]*http.Request{
			{URL: mustURL("https://example.com")},
			{URL: mustURL("https://example.com")},
			{URL: mustURL("https://example.com")},
			{URL: mustURL("https://example.com")},
			{URL: mustURL("https://example.com")},
		},
	)
	if err == nil {
		t.Fatal("redirect count must be capped")
	}
}

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func ssrfOperatorContext(taskURL, dest string) (*Server, *gin.Context, *httptest.ResponseRecorder) {
	s := &Server{}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	form := url.Values{}
	form.Set("url", taskURL)
	form.Set("dest", dest)
	form.Set("path", dest)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "admin")
	return s, c, w
}

// TestDownloadURLRejectsInternalTarget proves the implant-fetch instruction
// is validated server-side: cloud-metadata URLs never become tasks.
func TestDownloadURLRejectsInternalTarget(t *testing.T) {
	for _, u := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:8080/payload.exe",
		"file:///etc/passwd",
	} {
		s, c, w := ssrfOperatorContext(u, "C:\\temp\\x.exe")
		s.handleDownloadURL(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("handleDownloadURL(%q) status=%d, want 400", u, w.Code)
		}
	}
}

// TestHandleDownloadRejectsInternalTarget covers the sibling implant-fetch
// endpoint with the same gate.
func TestHandleDownloadRejectsInternalTarget(t *testing.T) {
	s, c, w := ssrfOperatorContext("http://169.254.169.254/latest/meta-data/", "C:\\temp\\x.exe")
	s.handleDownload(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("handleDownload status=%d, want 400", w.Code)
	}
}

// TestFrontCheckDomainRejectsInternal proves the domain-front probe refuses
// internal targets without touching the network.
func TestFrontCheckDomainRejectsInternal(t *testing.T) {
	s := &Server{}
	for _, d := range []string{"169.254.169.254", "127.0.0.1", "localhost"} {
		st := s.frontCheckDomain(d)
		if st.Healthy {
			t.Errorf("frontCheckDomain(%q) healthy, want refused", d)
		}
		if st.Error == "" {
			t.Errorf("frontCheckDomain(%q) has no error text", d)
		}
	}
}