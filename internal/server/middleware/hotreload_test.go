package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestRateLimiterSetLimits verifies live quota updates preserve state.
func TestRateLimiterSetLimits(t *testing.T) {
	rl := NewRateLimiter(context.Background(), 2, time.Minute)
	defer rl.Stop()
	rl.SetLimits(10, 2*time.Minute)
	if limit, window := rl.Snapshot(); limit != 10 || window != 2*time.Minute {
		t.Fatalf("snapshot = (%d, %v)", limit, window)
	}
	rl.SetLimits(-1, -time.Second)
	if limit, _ := rl.Snapshot(); limit != 10 {
		t.Fatalf("invalid values must not apply, limit = %d", limit)
	}
}

// TestAPIRateLimiterSetCapacityRate verifies live budget updates.
func TestAPIRateLimiterSetCapacityRate(t *testing.T) {
	rl := NewAPIRateLimiter(context.Background(), 100, 10)
	defer rl.Stop()
	rl.SetCapacityRate(20, 5)
	if cap, rate := rl.Snapshot(); cap != 20 || rate != 5 {
		t.Fatalf("snapshot = (%v, %v)", cap, rate)
	}
	rl.SetCapacityRate(0, 0)
	if cap, _ := rl.Snapshot(); cap != 20 {
		t.Fatalf("invalid values must not apply, capacity = %v", cap)
	}
}

// TestJWTRotationGrace verifies pre-rotation tokens stay valid inside the
// window and die after it.
func TestJWTRotationGrace(t *testing.T) {
	mkToken := func(secret string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{Username: "op"})
		signed, err := tok.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return signed
	}
	oldSecret := "old-secret-0123456789abcdef-old!!"
	newSecret := "new-secret-0123456789abcdef-new!!"
	jwtSecret.Store([]byte(oldSecret))
	before := mkToken(oldSecret)

	jwtRotateSecret(newSecret)
	if _, err := parseClaimsWithGrace(before, &Claims{}); err != nil {
		t.Fatalf("pre-rotation token must validate in grace window: %v", err)
	}
	if _, err := parseClaimsWithGrace(mkToken(newSecret), &Claims{}); err != nil {
		t.Fatalf("new token must validate: %v", err)
	}
	if _, err := parseClaimsWithGrace(mkToken("bogus-secret-0123456789abcdef!"), &Claims{}); err == nil {
		t.Fatal("foreign token must not validate")
	}

	// Expire the grace entry manually: old token must now fail.
	prevJWTSecret.Store(jwtGraceEntry{key: []byte(oldSecret), expires: time.Now().Add(-time.Second)})
	if _, err := parseClaimsWithGrace(before, &Claims{}); err == nil {
		t.Fatal("expired grace token must not validate")
	}

	// Idempotent rotation with the same value must not create a grace entry.
	jwtRotateSecret(newSecret)
	if _, err := parseClaimsWithGrace(mkToken(newSecret), &Claims{}); err != nil {
		t.Fatalf("same-value rotation broke validation: %v", err)
	}
}
