package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

// stagerMTLSForm builds a DNS stager form (no listener lookup needed) with the
// given mTLS material and transport hardening fields.
func stagerMTLSForm(cert, key, ca string) url.Values {
	form := url.Values{
		"dns_domain": {"c2.example.com"},
		"user_agent": {"Mozilla/5.0 (Test)"},
		"proxy":      {"http://127.0.0.1:3128"},
	}
	if cert != "" || key != "" || ca != "" {
		form.Set("mtls_cert", cert)
		form.Set("mtls_key", key)
		form.Set("mtls_ca", ca)
	}
	return form
}

// TestBindStagerFormRejectsInvalidMTLSMaterial pins the API-boundary refusal
// for the stager: material that could never complete a handshake must be
// rejected before any build work starts, exactly like the direct generator.
func TestBindStagerFormRejectsInvalidMTLSMaterial(t *testing.T) {
	s := mustTenantServer(t)
	cert, key := testMTLSPair(t)
	_, otherKey := testMTLSPair(t)

	cases := []struct {
		name string
		form url.Values
	}{
		{"key without certificate", stagerMTLSForm("", key, "")},
		{"certificate without key", stagerMTLSForm(cert, "", "")},
		{"mismatched pair", stagerMTLSForm(cert, otherKey, "")},
		{"garbage material", stagerMTLSForm("not a pem", "not a key", "")},
		{"garbage CA", stagerMTLSForm(cert, key, "not a ca bundle")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := tenantCtx(http.MethodPost, "/api/generate/stager", "stager-op", tc.form.Encode())
			if form, err := s.bindStagerForm(c); err == nil || form != nil {
				t.Fatal("invalid mTLS material was accepted")
			}
		})
	}
}

// TestBindStagerFormAcceptsAndTrimsMTLSMaterial covers the happy paths: a bare
// pair, a pair with CA, and material pasted with surrounding whitespace (the
// common case when copying from a PEM file) which must be normalized.
func TestBindStagerFormAcceptsAndTrimsMTLSMaterial(t *testing.T) {
	s := mustTenantServer(t)
	cert, key := testMTLSPair(t)
	wantCert, wantKey := strings.TrimSpace(cert), strings.TrimSpace(key)

	c, _ := tenantCtx(http.MethodPost, "/api/generate/stager", "stager-op", stagerMTLSForm("", "", "").Encode())
	form, err := s.bindStagerForm(c)
	if err != nil {
		t.Fatalf("material-free form rejected: %v", err)
	}
	if form.MTLSCert != "" || form.MTLSKey != "" || form.MTLSCA != "" {
		t.Fatal("empty material was not left empty")
	}

	c, _ = tenantCtx(http.MethodPost, "/api/generate/stager", "stager-op", stagerMTLSForm(cert, key, cert).Encode())
	form, err = s.bindStagerForm(c)
	if err != nil {
		t.Fatalf("valid pair+CA rejected: %v", err)
	}
	if form.MTLSCert != wantCert || form.MTLSKey != wantKey || form.MTLSCA != wantCert {
		t.Fatal("material not carried into the bound form")
	}

	c, _ = tenantCtx(http.MethodPost, "/api/generate/stager", "stager-op", stagerMTLSForm("\n  "+cert+"\n", key+"  \n", "").Encode())
	form, err = s.bindStagerForm(c)
	if err != nil {
		t.Fatalf("whitespace-padded material rejected: %v", err)
	}
	if form.MTLSCert != wantCert || form.MTLSKey != wantKey {
		t.Fatal("surrounding whitespace was not trimmed")
	}
}

// TestStagerFetchCarriesTransportHardening ties the bound form to the fetch
// descriptor the token stager builder consumes. Skip-verify, proxy and user
// agent used to be dropped here, so a stager generated for a self-signed
// listener failed its stage fetch no matter what the operator selected.
func TestStagerFetchCarriesTransportHardening(t *testing.T) {
	form := &stagerGenerateForm{
		C2URL:         "https://c2.example.com:8443/api/v1/beacon",
		SkipTLSVerify: true,
		Proxy:         "http://127.0.0.1:3128",
		UserAgent:     "Mozilla/5.0 (Test)",
		MTLSCert:      "CERT-PEM",
		MTLSKey:       "KEY-PEM",
		MTLSCA:        "CA-PEM",
	}
	fetch := stagerFetch(form, "tok", "sig", "keyhex")
	if fetch.BaseURL != "https://c2.example.com:8443" {
		t.Fatalf("BaseURL = %q, want origin only", fetch.BaseURL)
	}
	if fetch.Token != "tok" || fetch.Sig != "sig" || fetch.KeyHex != "keyhex" {
		t.Fatal("token material not carried into the fetch descriptor")
	}
	if !fetch.SkipTLSVerify {
		t.Fatal("skip_tls_verify was dropped")
	}
	if fetch.Proxy != form.Proxy || fetch.UserAgent != form.UserAgent {
		t.Fatal("proxy or user agent was dropped")
	}
	if fetch.MTLSCertPEM != form.MTLSCert || fetch.MTLSKeyPEM != form.MTLSKey || fetch.MTLSCAPEM != form.MTLSCA {
		t.Fatal("mTLS material was not carried into the fetch descriptor")
	}
}

// TestStagerRegisterRefusesMTLSListener pins the lazy-path guard: the token
// row persists no client material (private keys are never stored), so a
// listener that verifies client certificates could never be served by the lazy
// flow. Registering must answer 400 instead of handing out a dead URL.
func TestStagerRegisterRefusesMTLSListener(t *testing.T) {
	s := mustTenantServer(t)
	l := db.Listener{Name: "stager-mtls-listener", Host: "127.0.0.1", Port: 8443, Scheme: "https", RequireClientCert: true}
	if err := s.db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}

	body := `{"listener_id":` + strconv.FormatUint(uint64(l.ID), 10) + `}`
	c, w := tenantCtx(http.MethodPost, "/api/stager/register", "stager-op", body)
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleAPIStagerRegister(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "client certificates") {
		t.Fatalf("refusal does not explain the mTLS requirement: %s", w.Body.String())
	}

	// A listener without the requirement keeps working.
	plain := db.Listener{Name: "stager-plain-listener", Host: "127.0.0.1", Port: 8080, Scheme: "http"}
	if err := s.db.Create(&plain).Error; err != nil {
		t.Fatal(err)
	}
	body = `{"listener_id":` + strconv.FormatUint(uint64(plain.ID), 10) + `}`
	c, w = tenantCtx(http.MethodPost, "/api/stager/register", "stager-op", body)
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleAPIStagerRegister(c)
	if w.Code != http.StatusOK {
		t.Fatalf("plain listener register: status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}
