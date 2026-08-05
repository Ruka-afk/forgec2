package totp

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestVerifyCode_TTLAcceptsCurrentWindow(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !VerifyCode(secret, code) {
		t.Errorf("current-window code %q rejected", code)
	}
}

func TestVerifyCode_TTLRejectsStaleCodes(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	// A code from 2+ windows in the past is beyond the accepted skew and
	// must be rejected.
	stale := time.Now().Add(-time.Duration(PeriodSeconds*(SkewWindows+2)) * time.Second)
	code, err := totp.GenerateCode(secret, stale)
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if VerifyCode(secret, code) {
		t.Errorf("stale code %q accepted (TTL exceeded)", code)
	}
}

func TestVerifyCode_RejectsMalformedInput(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	for _, bad := range []string{"", "abc123", "12345", "1234567"} {
		if VerifyCode(secret, bad) {
			t.Errorf("malformed code %q accepted", bad)
		}
	}
}

func TestGenerateSecret_ValidBase32(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	// 16 raw bytes encode to 26 base32 chars plus padding.
	if len(s) < 26 {
		t.Errorf("secret too short: %q", s)
	}
	trimmed := strings.TrimRight(s, "=")
	if len(trimmed) != 26 || !strings.ContainsAny(trimmed, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") {
		t.Errorf("unexpected secret shape: %q", s)
	}
	if !VerifyCode(s, mustCode(t, s)) {
		t.Errorf("round-trip verification failed for generated secret")
	}
}

func mustCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	return code
}
