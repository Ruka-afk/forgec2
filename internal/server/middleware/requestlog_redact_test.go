package middleware

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedactRequestPath(t *testing.T) {
	cases := []struct {
		raw      string
		want     string
		leakSubs []string
	}{
		{
			raw:      "/stage/abc123secrettoken",
			want:     "/stage/[redacted]",
			leakSubs: []string{"abc123secrettoken"},
		},
		{
			raw:      "/phishing/l/phish-token-xyz",
			want:     "/phishing/l/[redacted]",
			leakSubs: []string{"phish-token-xyz"},
		},
		{
			raw:      "/payloads/7/deadbeef.payload",
			want:     "/payloads/[redacted]",
			leakSubs: []string{"deadbeef"},
		},
		{
			raw:      "/api/agents?token=sekret&page=1",
			want:     "/api/agents?page=1&token=%2A",
			leakSubs: []string{"sekret"},
		},
		{
			raw:  "/dashboard",
			want: "/dashboard",
		},
		{
			raw:  "/api/tasks?status=done",
			want: "/api/tasks?status=done",
		},
	}

	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.raw, err)
		}
		got := redactRequestPath(u)
		if got != tc.want {
			t.Errorf("redactRequestPath(%q) = %q, want %q", tc.raw, got, tc.want)
		}
		for _, sub := range tc.leakSubs {
			if strings.Contains(got, sub) {
				t.Errorf("redactRequestPath(%q) leaked %q in %q", tc.raw, sub, got)
			}
		}
	}
}

func TestRedactRequestPathNil(t *testing.T) {
	if got := redactRequestPath(nil); got != "" {
		t.Fatalf("nil URL: got %q, want empty", got)
	}
}
