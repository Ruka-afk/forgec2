package middleware

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIsAllowedLocalhostOrigin(t *testing.T) {
	valid := []string{
		"http://localhost",
		"http://localhost:3000",
		"https://localhost",
		"http://127.0.0.1",
		"https://127.0.0.1:8443",
		"http://[::1]:8080",
		"https://[::1]",
	}
	for _, origin := range valid {
		if !isAllowedLocalhostOrigin(origin) {
			t.Errorf("isAllowedLocalhostOrigin(%q) = false, want true", origin)
		}
	}

	invalid := []string{
		"http://localhost:evil.example.com", // P1-8: host prefix must not be accepted
		"http://localhost.evil.example.com",
		"http://localhost:99999", // port out of range
		"http://localhost:abc",   // non-numeric port
		"http://localhost/path",  // path not allowed
		"http://localhost?q=1",   // query not allowed
		"http://localhost#frag",  // fragment not allowed
		"http://evil.com",
		"ftp://localhost",
		"http://",
		"not-a-url",
		"http://user@localhost", // userinfo not allowed
	}
	for _, origin := range invalid {
		if isAllowedLocalhostOrigin(origin) {
			t.Errorf("isAllowedLocalhostOrigin(%q) = true, want false", origin)
		}
	}
}

func TestIsSecureConnection_RequiresTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer SetTrustedProxyIPs(nil)

	mkReq := func(remoteAddr, xfp string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Request.RemoteAddr = remoteAddr
		if xfp != "" {
			c.Request.Header.Set("X-Forwarded-Proto", xfp)
		}
		return c
	}

	// No trusted proxies configured: X-Forwarded-Proto must be ignored.
	SetTrustedProxyIPs(nil)
	if IsSecureConnection(mkReq("10.0.0.1:1234", "https")) {
		t.Error("X-Forwarded-Proto must not be honored without a trusted proxy")
	}

	// Peer outside the trusted range must not be able to spoof the header.
	SetTrustedProxyIPs([]string{"10.0.0.0/8"})
	if IsSecureConnection(mkReq("192.168.1.5:1234", "https")) {
		t.Error("untrusted peer spoofed X-Forwarded-Proto")
	}

	// Trusted proxy present with https header.
	if !IsSecureConnection(mkReq("10.0.0.1:1234", "https")) {
		t.Error("trusted proxy with X-Forwarded-Proto: https should be secure")
	}

	// Trusted proxy with a non-https header must not be secure.
	if IsSecureConnection(mkReq("10.0.0.1:1234", "http")) {
		t.Error("trusted proxy with X-Forwarded-Proto: http must not be secure")
	}

	// CIDR form must also work.
	SetTrustedProxyIPs([]string{"10.0.0.0/8"})
	if !IsSecureConnection(mkReq("10.1.2.3:9999", "https")) {
		t.Error("CIDR-form trusted proxy not honored")
	}

	// Direct TLS is always secure regardless of proxy config.
	c := mkReq("192.168.1.5:1234", "")
	c.Request.TLS = &tls.ConnectionState{}
	if !IsSecureConnection(c) {
		t.Error("direct TLS should be secure")
	}
}

func TestRequestBodyLimit_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/limit", RequestBodyLimit(64), func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.String(http.StatusOK, "len=%d", len(body))
	})

	// Small body is accepted.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/limit", strings.NewReader("small"))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("small body: got %d, want 200", w.Code)
	}

	// Body over the limit is rejected with 413.
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/limit", strings.NewReader(strings.Repeat("x", 1024)))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: got %d, want 413", w.Code)
	}
}
