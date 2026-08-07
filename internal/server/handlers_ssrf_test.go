package server

import (
	"net/http"
	"net/url"
	"testing"
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