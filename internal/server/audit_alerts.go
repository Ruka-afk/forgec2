package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

type loginLockoutState struct {
	attempts    int
	windowStart time.Time
	lockedUntil time.Time
	// ips tracks distinct source IPs for account-level entries so that a
	// single-source attacker cannot trivially lock another user's account.
	ips map[string]struct{}
}

type loginLockoutTracker struct {
	mu             sync.Mutex
	entries        map[string]*loginLockoutState
	accountEntries map[string]*loginLockoutState
}

const maxLockoutEntries = 10000

func newLoginLockoutTracker() *loginLockoutTracker {
	return &loginLockoutTracker{
		entries:        make(map[string]*loginLockoutState),
		accountEntries: make(map[string]*loginLockoutState),
	}
}

// startCleanup periodically removes expired lockout entries to prevent unbounded memory growth.
func (t *loginLockoutTracker) startCleanup(ctx context.Context, windowSec int) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		window := time.Duration(windowSec) * time.Second
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				func() {
					t.mu.Lock()
					defer t.mu.Unlock()
					now := time.Now()
					for ip, entry := range t.entries {
						if (!entry.lockedUntil.IsZero() && now.After(entry.lockedUntil.Add(5*time.Minute))) ||
							(entry.lockedUntil.IsZero() && now.After(entry.windowStart.Add(window))) {
							delete(t.entries, ip)
						}
					}
					for user, entry := range t.accountEntries {
						if (!entry.lockedUntil.IsZero() && now.After(entry.lockedUntil.Add(5*time.Minute))) ||
							(entry.lockedUntil.IsZero() && now.After(entry.windowStart.Add(window))) {
							delete(t.accountEntries, user)
						}
					}
				}()
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
		if len(t.entries) >= maxLockoutEntries {
			oldestIP := ""
			var oldestTime time.Time
			for k, v := range t.entries {
				if oldestIP == "" || v.windowStart.Before(oldestTime) {
					oldestIP = k
					oldestTime = v.windowStart
				}
			}
			if oldestIP != "" {
				delete(t.entries, oldestIP)
			}
		}
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

func (t *loginLockoutTracker) isAccountLocked(username string, now time.Time) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.accountEntries[username]
	if !ok || entry.lockedUntil.IsZero() || now.After(entry.lockedUntil) {
		return false, 0
	}
	retryAfter := int(entry.lockedUntil.Sub(now).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}
	return true, retryAfter
}

func (t *loginLockoutTracker) recordAccountFailure(username, ip string, maxAttempts, windowSec, lockoutSec int, now time.Time) (locked bool, retryAfter int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entry, ok := t.accountEntries[username]
	if !ok {
		if len(t.accountEntries) >= maxLockoutEntries {
			oldestUser := ""
			var oldestTime time.Time
			for k, v := range t.accountEntries {
				if oldestUser == "" || v.windowStart.Before(oldestTime) {
					oldestUser = k
					oldestTime = v.windowStart
				}
			}
			if oldestUser != "" {
				delete(t.accountEntries, oldestUser)
			}
		}
		entry = &loginLockoutState{windowStart: now, ips: make(map[string]struct{})}
		t.accountEntries[username] = entry
	}
	if entry.ips == nil {
		entry.ips = make(map[string]struct{})
	}
	if ip != "" {
		entry.ips[ip] = struct{}{}
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
		if len(entry.ips) >= 2 {
			entry.lockedUntil = now.Add(time.Duration(lockoutSec) * time.Second)
			entry.attempts = 0
			retryAfter = lockoutSec
			if retryAfter < 1 {
				retryAfter = 1
			}
			return true, retryAfter
		}
		// All failures came from a single source: the per-IP lockout already
		// handles that case. Restart the window so one attacker cannot lock a
		// legitimate user's account from a single IP.
		entry.attempts = 0
		entry.windowStart = now
	}
	return false, 0
}

func (t *loginLockoutTracker) resetAccount(username string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.accountEntries, username)
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
	s.configMu.RLock()
	whitelist := s.cfg.RateLimit.Login.Whitelist
	s.configMu.RUnlock()
	for _, wl := range whitelist {
		if wl == ip {
			slog.Debug("Login lockout check bypassed for whitelisted IP", "ip", ip)
			return false, 0
		}
	}
	return s.loginLockout.isLocked(ip, time.Now())
}

func (s *Server) recordLoginFailure(ip, username string) (locked bool, retryAfter int) {
	if s.loginLockout == nil {
		return false, 0
	}
	s.configMu.RLock()
	whitelist := s.cfg.RateLimit.Login.Whitelist
	maxAttempts := s.cfg.RateLimit.Login.MaxAttempts
	windowSec := s.cfg.RateLimit.Login.Window
	lockoutSec := s.cfg.RateLimit.Login.LockoutTime
	s.configMu.RUnlock()
	for _, wl := range whitelist {
		if wl == ip {
			slog.Warn("Login failure from whitelisted IP — lockout bypassed", "ip", ip, "username", username)
			return false, 0
		}
	}

	if maxAttempts < 1 {
		maxAttempts = DefaultMaxLoginAttempts
	}
	if windowSec < 1 {
		windowSec = DefaultLoginWindowSec
	}
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

func (s *Server) checkAccountLockout(username string) (bool, int) {
	if s.loginLockout == nil {
		return false, 0
	}
	return s.loginLockout.isAccountLocked(username, time.Now())
}

func (s *Server) recordAccountLoginFailure(username, ip string) (locked bool, retryAfter int) {
	if s.loginLockout == nil {
		return false, 0
	}
	s.configMu.RLock()
	maxAttempts := s.cfg.RateLimit.Login.MaxAttempts
	windowSec := s.cfg.RateLimit.Login.Window
	lockoutSec := s.cfg.RateLimit.Login.LockoutTime
	s.configMu.RUnlock()
	if maxAttempts < 1 {
		maxAttempts = DefaultMaxLoginAttempts
	}
	if windowSec < 1 {
		windowSec = DefaultLoginWindowSec
	}
	if lockoutSec < 1 {
		lockoutSec = DefaultLockoutTimeSec
	}

	locked, retryAfter = s.loginLockout.recordAccountFailure(username, ip, maxAttempts, windowSec, lockoutSec, time.Now())
	if locked {
		msg := fmt.Sprintf("Account lockout for user %s. Retry after %ds.", username, retryAfter)
		s.broadcastSystemAlert("Account Lockout", msg, "account_lockout")
		slog.Warn("Account lockout triggered", "username", username, "retry_after", retryAfter)
	}
	return locked, retryAfter
}

func (s *Server) broadcastBulkAgentDeleteAlert(operator string, count int) {
	if count <= 1 {
		return
	}
	msg := fmt.Sprintf("%s bulk-deleted %d agents", operator, count)
	s.broadcastSystemAlert("Bulk Agent Delete", msg, "bulk_agent_delete")
}
