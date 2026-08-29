//go:build linux || windows || darwin

package main

import "testing"

func TestFormatProcessTreeNesting(t *testing.T) {
	out := formatProcessTree([]procNode{
		{PID: 1, PPID: 0, User: "root", Name: "init"},
		{PID: 10, PPID: 1, User: "root", Name: "sshd"},
		{PID: 20, PPID: 10, User: "user", Name: "bash"},
		{PID: 2, PPID: 0, User: "root", Name: "kthreadd"},
	})
	if out == "" {
		t.Fatal("empty tree")
	}
	if want := "1\t0\troot\tinit"; !containsLine(out, want) {
		t.Fatalf("missing root line %q in:\n%s", want, out)
	}
	if want := "20\t10\tuser\tbash"; !containsLine(out, want) {
		t.Fatalf("missing child line %q in:\n%s", want, out)
	}
}

func containsLine(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		len(s) > 0 && (indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
