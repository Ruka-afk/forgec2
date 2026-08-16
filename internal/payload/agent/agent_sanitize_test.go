//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "testing"

func TestSanitizeLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"ForgeC2", "ForgeC2"},
		{"my agent", "my.agent"},
		{"bad/name:with*chars", "bad.name.with.chars"},
		{"---leading", "---leading"},
		{"trailing---", "trailing---"},
		{"a..b", "a..b"},
		{"", "agent"},
		{"!!!", "agent"},
		{"...", "..."},
		{"com.example.agent", "com.example.agent"},
		{"weird_under-score-dash", "weird_under-score-dash"},
	}
	for _, c := range cases {
		if got := sanitizeLabel(c.in); got != c.want {
			t.Errorf("sanitizeLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
