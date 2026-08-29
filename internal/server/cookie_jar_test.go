package server

import (
	"strings"
	"testing"
)

func TestParseCookieExportJSON(t *testing.T) {
	out := "=== Chrome COOKIES ===\n--- exported 1 cookies ---\n=== JSON ===\n[{\"domain\":\".example.com\",\"name\":\"sid\",\"path\":\"/\",\"value\":\"abc\",\"expires\":13300000000000000,\"secure\":true}]\n"
	jars := parseCookieExport(out)
	if len(jars) != 1 {
		t.Fatalf("got %d jars", len(jars))
	}
	if jars[0].Name != "sid" || jars[0].Value != "abc" || !jars[0].Secure {
		t.Fatalf("unexpected entry: %+v", jars[0])
	}
}

func TestParseCookieExportTSV(t *testing.T) {
	out := "=== Edge COOKIES ===\nwww.lab.local\tsess\t/\texpires=0\tsecure=1\tvalue=tok123\n--- exported 1 cookies ---\n"
	jars := parseCookieExport(out)
	if len(jars) != 1 || jars[0].Name != "sess" || jars[0].Value != "tok123" {
		t.Fatalf("tsv parse: %+v", jars)
	}
}

func TestCookieMatchesHost(t *testing.T) {
	if !cookieMatchesHost(".example.com", "www.example.com") {
		t.Fatal("suffix match")
	}
	if !cookieMatchesHost("example.com", "example.com:8080") {
		t.Fatal("port strip")
	}
	if cookieMatchesHost(".example.com", "notexample.com") {
		t.Fatal("should not match sibling")
	}
}

func TestCookieHeaderAndNetscape(t *testing.T) {
	jars := []cookieJarEntry{
		{Domain: ".lab.local", Name: "a", Path: "/", Value: "1", Secure: true},
		{Domain: "other.test", Name: "b", Path: "/", Value: "[v20-decrypt-failed]"},
	}
	hdr := cookieHeaderForHost(jars, "app.lab.local")
	if hdr != "a=1" {
		t.Fatalf("header %q", hdr)
	}
	if cookieHeaderForHost(jars, "other.test") != "" {
		t.Fatal("failed decrypt should be skipped")
	}
	ns := netscapeCookieFile(jars)
	if !strings.Contains(ns, "Netscape HTTP Cookie File") || !strings.Contains(ns, "\ta\t1") {
		t.Fatalf("netscape:\n%s", ns)
	}
}

func TestChromeExpiryUnix(t *testing.T) {
	if chromeExpiryUnix(0) != 0 {
		t.Fatal("zero")
	}
	// 13300000000000000 µs since 1601 ≈ 2022-ish
	u := chromeExpiryUnix(13300000000000000)
	if u < 1_600_000_000 || u > 2_000_000_000 {
		t.Fatalf("unexpected unix %d", u)
	}
}
