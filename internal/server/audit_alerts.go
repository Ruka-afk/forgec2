package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

type loginLockoutState struct {
	attempts    int
	windowStart time.Time
	lockedUntil time.Time
}

type loginLockoutTracker struct {
	mu      sync.Mutex
	entries map[string]*loginLockoutState
}

func newLoginLockoutTracker() *loginLockoutTracker {
	return &loginLockoutTracker{entries: make(map[string]*loginLockoutState)}
}

// startCleanup periodically removes expired lockout entries to prevent unbounded memory growth.
func (t *loginLockoutTracker) startCleanup(ctx context.Context) {
	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.mu.Lock()
				now := time.Now()
				for ip, entry := range t.entries {
					if entry.lockedUntil.IsZero() || now.After(entry.lockedUntil.Add(5*time.Minute)) {
						delete(t.entries, ip)
					}
				}
				t.mu.Unlock()
			}
		}
	}()
}

func (t *loginLockoutTracker) isLocked(ip string, now time.Time) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.entries[ip]
	if !ok || entry.lockedUntil.IsZero() || now.After(entry.lockedUntil) {
		return false, 0
	}
	retryAfter := int(entry.lockedUntil.Sub(now).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return true, retryAfter
}

func (t *loginLockoutTracker) recordFailure(ip string, maxAttempts, windowSec, lockoutSec int, now time.Time) (locked bool, retryAfter int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.entries[ip]
	if !ok {
		entry = &loginLockoutState{windowStart: now}
		t.entries[ip] = entry
	}

	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		retryAfter = int(entry.lockedUntil.Sub(now).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		return true, retryAfter
	}

	if now.Sub(entry.windowStart) > time.Duration(windowSec)*time.Second {
		entry.windowStart = now
		entry.attempts = 0
		entry.lockedUntil = time.Time{}
	}

	entry.attempts++
	if entry.attempts >= maxAttempts {
		entry.lockedUntil = now.Add(time.Duration(lockoutSec) * time.Second)
		entry.attempts = 0
		retryAfter = lockoutSec
		if retryAfter < 1 {
			retryAfter = 1
		}
		return true, retryAfter
	}
	return false, 0
}

func (t *loginLockoutTracker) reset(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, ip)
}

// broadcastSystemAlert sends a system_alert event to all connected WebSocket clients.
func (s *Server) broadcastSystemAlert(title, message, alertType string) {
	payload := map[string]interface{}{
		"type":       "system_alert",
		"title":      title,
		"message":    message,
		"alert_type": alertType,
	}
	msg, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal system alert", "err", err)
		return
	}
	s.broadcastToClients(msg)
}

func (s *Server) checkLoginLockout(ip string) (locked bool, retryAfter int) {
	if s.loginLockout == nil {
		return false, 0
	}
	for _, wl := range s.cfg.RateLimit.Login.Whitelist {
		if wl == ip {
			return false, 0
		}
	}
	return s.loginLockout.isLocked(ip, time.Now())
}

func (s *Server) recordLoginFailure(ip, username string) (locked bool, retryAfter int) {
	if s.loginLockout == nil {
		return false, 0
	}
	for _, wl := range s.cfg.RateLimit.Login.Whitelist {
		if wl == ip {
			return false, 0
		}
	}

	maxAttempts := s.cfg.RateLimit.Login.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = DefaultMaxLoginAttempts
	}
	windowSec := s.cfg.RateLimit.Login.Window
	if windowSec < 1 {
		windowSec = DefaultLoginWindowSec
	}
	lockoutSec := s.cfg.RateLimit.Login.LockoutTime
	if lockoutSec < 1 {
		lockoutSec = DefaultLockoutTimeSec
	}

	locked, retryAfter = s.loginLockout.recordFailure(ip, maxAttempts, windowSec, lockoutSec, time.Now())
	if locked {
		msg := fmt.Sprintf("Login lockout from %s (user: %s). Retry after %ds.", ip, username, retryAfter)
		s.broadcastSystemAlert("Login Lockout", msg, "login_lockout")
		slog.Warn("Login lockout triggered", "ip", ip, "username", username, "retry_after", retryAfter)
	}
	return locked, retryAfter
}

func (s *Server) clearLoginLockout(ip string) {
	if s.loginLockout != nil {
		s.loginLockout.reset(ip)
	}
}

func (s *Server) broadcastBulkAgentDeleteAlert(operator string, count int) {
	if count <= 1 {
		return
	}
	msg := fmt.Sprintf("%s bulk-deleted %d agents", operator, count)
	s.broadcastSystemAlert("Bulk Agent Delete", msg, "bulk_agent_delete")
}