//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "testing"

func TestC2DialHostPort(t *testing.T) {
	cases := []struct {
		in         string
		wantHost   string
		wantScheme string
		wantOK     bool
	}{
		{"http://10.0.0.1:8443", "10.0.0.1:8443", "http", true},
		{"https://host:443/path", "host:443", "https", true},
		{"tcp://host:4444", "host:4444", "tcp", true},
		{"tls://c2.example:8443", "c2.example:8443", "tls", true},
		{"host:8080", "host:8080", "", true},
		{"[::1]:8443", "[::1]:8443", "", true},
		{"http://[::1]:8443", "[::1]:8443", "http", true},
		{"http://10.0.0.1:8443,tcp://10.0.0.1:4444", "10.0.0.1:8443", "http", true},
		{"", "", "", false},
		{"   ", "", "", false},
		{"http://", "", "", false},
	}
	for _, tc := range cases {
		host, scheme, ok := c2DialHostPort(tc.in)
		if ok != tc.wantOK || host != tc.wantHost || scheme != tc.wantScheme {
			t.Errorf("c2DialHostPort(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.in, host, scheme, ok, tc.wantHost, tc.wantScheme, tc.wantOK)
		}
	}
}

func TestHostnameFromHostPort(t *testing.T) {
	if got := hostnameFromHostPort("10.0.0.1:8443"); got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
	if got := hostnameFromHostPort("[::1]:8443"); got != "::1" {
		t.Fatalf("ipv6 got %q", got)
	}
	if got := hostnameFromHostPort("example.com"); got != "example.com" {
		t.Fatalf("bare host got %q", got)
	}
}

func TestSchemeMatchesTransport(t *testing.T) {
	if !schemeMatchesTransport("https", "http") {
		t.Fatal("https should match http family")
	}
	if !schemeMatchesTransport("tls", "tcp") {
		t.Fatal("tls should match tcp family")
	}
	if schemeMatchesTransport("http", "tcp") {
		t.Fatal("http must not match tcp")
	}
	if schemeMatchesTransport("", "tcp") {
		t.Fatal("empty scheme is not a configured tcp endpoint")
	}
}
