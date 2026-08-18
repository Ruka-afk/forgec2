//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func runCredCheck(t *testing.T, command string) (TaskResult, *credCheckOutput) {
	t.Helper()
	res := TaskResult{}
	handleCredCheck(Task{Command: command}, &res)
	if res.Error != "" {
		return res, nil
	}
	raw, err := base64.StdEncoding.DecodeString(res.Output)
	if err != nil {
		t.Fatalf("output is not base64: %v", err)
	}
	out := &credCheckOutput{}
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("output is not credCheckOutput JSON: %v", err)
	}
	return res, out
}

func withFakeCredCheckAuth(t *testing.T, fn func(domain, user, password, dc string) (string, string)) {
	t.Helper()
	old := credCheckAuth
	credCheckAuth = fn
	t.Cleanup(func() { credCheckAuth = old })
}

func TestCredCheckMalformedCommands(t *testing.T) {
	withFakeCredCheckAuth(t, func(_, _, _, _ string) (string, string) { return "valid", "" })
	for _, cmd := range []string{"", "user|domain", "|domain|pass", "user||pass", "user|domain|"} {
		res := TaskResult{}
		handleCredCheck(Task{Command: cmd}, &res)
		if res.Error == "" {
			t.Errorf("command %q: expected error, got output %q", cmd, res.Output)
		}
	}
}

func TestCredCheckValidResetsFuse(t *testing.T) {
	const domain = "credcheckvalid.local"
	defer credCheckResetFailures(domain)

	withFakeCredCheckAuth(t, func(d, _, _, dc string) (string, string) {
		if d != domain {
			t.Errorf("expected domain %q, got %q", domain, d)
		}
		return "valid", "password expired"
	})
	res, out := runCredCheck(t, "jdoe|"+domain+"|P@ssw0rd!|10.0.0.5")
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if out.Summary.Valid != 1 || out.Summary.Total != 1 {
		t.Fatalf("unexpected summary: %+v", out.Summary)
	}
	if len(out.Results) != 1 || out.Results[0].User != "jdoe" || out.Results[0].Status != "valid" || out.Results[0].Error == "" {
		t.Fatalf("unexpected result: %+v", out.Results)
	}
	if credCheckFuseTripped(domain) {
		t.Fatal("fuse should not be tripped after a valid result")
	}
}

func TestCredCheckFuseTripsAfterFiveFailures(t *testing.T) {
	const domain = "credcheckfuse.local"
	defer credCheckResetFailures(domain)

	withFakeCredCheckAuth(t, func(_, _, _, _ string) (string, string) { return "invalid", "" })

	for i := 0; i < credCheckMaxFailures; i++ {
		res, out := runCredCheck(t, "jdoe|"+domain+"|wrong"+strings.Repeat("x", i))
		if res.Error != "" {
			t.Fatalf("attempt %d: unexpected error: %s", i+1, res.Error)
		}
		if out.Summary.Invalid != 1 {
			t.Fatalf("attempt %d: expected invalid count 1, got %+v", i+1, out.Summary)
		}
	}
	if !credCheckFuseTripped(domain) {
		t.Fatal("fuse should be tripped after five failures")
	}

	res, _ := runCredCheck(t, "jdoe|"+domain+"|wrong")
	if res.Error != "fuse_tripped" {
		t.Fatalf("expected fuse_tripped error, got %q", res.Error)
	}
}

func TestCredCheckLockedCountsTowardFuseErrorsDoNot(t *testing.T) {
	const domain = "credchecklock.local"
	defer credCheckResetFailures(domain)

	withFakeCredCheckAuth(t, func(d, _, _, _ string) (string, string) {
		switch d {
		case domain:
			return "locked", "account locked"
		default:
			return "error", "transport failure"
		}
	})

	res, out := runCredCheck(t, "jdoe|"+domain+"|wrong")
	if res.Error != "" || out.Summary.Locked != 1 {
		t.Fatalf("locked handling failed: %+v %+v", res, out)
	}
	for i := 1; i < credCheckMaxFailures; i++ {
		runCredCheck(t, "jdoe|"+domain+"|wrongx")
	}
	if !credCheckFuseTripped(domain) {
		t.Fatal("locked should count toward the fuse")
	}

	const errDomain = "credcheckerr.local"
	defer credCheckResetFailures(errDomain)
	for i := 0; i < credCheckMaxFailures+2; i++ {
		res, out := runCredCheck(t, "jdoe|"+errDomain+"|x")
		if res.Error != "" || out.Summary.Errors != 1 {
			t.Fatalf("error-status handling failed: %+v %+v", res, out)
		}
	}
	if credCheckFuseTripped(errDomain) {
		t.Fatal("transport errors must not trip the fuse")
	}
}

func TestCredCheckDCPassthrough(t *testing.T) {
	const domain = "credcheckdc.local"
	defer credCheckResetFailures(domain)

	gotDC := ""
	withFakeCredCheckAuth(t, func(_, _, _, dc string) (string, string) {
		gotDC = dc
		return "invalid", ""
	})
	res, _ := runCredCheck(t, "jdoe|"+domain+"|pass|192.168.1.10")
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if gotDC != "192.168.1.10" {
		t.Fatalf("dc not passed through: got %q", gotDC)
	}
}