//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "testing"

// TestValidateEgressURL gates operator download URLs against SSRF into
// cloud metadata and link-local targets. No network access is required:
// literals and scheme/host checks are deterministic, and the unresolvable
// host case must fail closed with or without DNS.
func TestValidateEgressURL(t *testing.T) {
	allowed := []string{
		"http://example.com/payload.bin",
		"https://example.com:8443/a/b?x=1",
		"http://127.0.0.1:8000/stage",
		"http://10.0.0.5/share/tool.exe",
		"http://[::1]:8000/stage",
	}
	for _, u := range allowed {
		if err := validateEgressURL(u); err != nil {
			t.Errorf("validateEgressURL(%q) = %v, want nil", u, err)
		}
	}
	denied := []string{
		"ftp://example.com/x",
		"file:///etc/passwd",
		"gopher://example.com/x",
		"http://",
		"http://169.254.169.254/latest/meta-data/",
		"http://169.254.169.254:80/",
		"http://[fe80::1]/x",
		"http://[fd00::1]/x",
		"http://[::ffff:169.254.169.254]/x",
		"http://nonexistent.invalid/x",
		"",
		"::::",
	}
	for _, u := range denied {
		if err := validateEgressURL(u); err == nil {
			t.Errorf("validateEgressURL(%q) = nil, want error", u)
		}
	}
}
