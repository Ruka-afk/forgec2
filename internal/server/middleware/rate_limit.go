package middleware

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type TokenBucket struct {
	capacity   float64
	rate       float64
	tokens     float64
	lastRefill time.Time
	accessedAt time.Time
	mu         sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	now := time.Now()
	return &TokenBucket{
		capacity:   capacity,
		rate:       rate,
		tokens:     capacity,
		lastRefill: now,
		accessedAt: now,
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now
	tb.accessedAt = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokens := tb.tokens + elapsed*tb.rate
	if tokens > tb.capacity {
		tokens = tb.capacity
	}
	return tokens
}

const maxBuckets = 10000

type APIRateLimiter struct {
	buckets   map[string]*TokenBucket
	mu        sync.Mutex
	capacity  float64
	rate      float64
	whitelist map[string]bool
	stop      chan struct{}
}

func NewAPIRateLimiter(ctx context.Context, capacity, rate float64) *APIRateLimiter {
	rl := &APIRateLimiter{
		buckets:   make(map[string]*TokenBucket),
		capacity:  capacity,
		rate:      rate,
		whitelist: make(map[string]bool),
		stop:      make(chan struct{}),
	}
	go rl.cleanup(ctx)
	return rl
}

func (rl *APIRateLimiter) SetWhitelist(ips []string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.whitelist = make(map[string]bool)
	for _, ip := range ips {
		rl.whitelist[ip] = true
	}
}

func (rl *APIRateLimiter) RemoveWhitelist(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.whitelist, ip)
}

func (rl *APIRateLimiter) GetBucket(key string) *TokenBucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	bucket, exists := rl.buckets[key]
	if exists {
		bucket.mu.Lock()
		bucket.accessedAt = time.Now()
		bucket.mu.Unlock()
		return bucket
	}
	if len(rl.buckets) >= maxBuckets {
		rl.evictOldest()
	}
	bucket = NewTokenBucket(rl.capacity, rl.rate)
	rl.buckets[key] = bucket
	return bucket
}

func (rl *APIRateLimiter) evictOldest() {
	now := time.Now()
	// First pass: evict stale entries (>5min untouched)
	for key, bucket := range rl.buckets {
		bucket.mu.Lock()
		accessed := bucket.accessedAt
		bucket.mu.Unlock()
		if now.Sub(accessed) > 5*time.Minute {
			delete(rl.buckets, key)
			return
		}
	}
	// Second pass: evict any single entry (O(1) fallback)
	for key := range rl.buckets {
		delete(rl.buckets, key)
		return
	}
}

func (rl *APIRateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.mu.Lock()
			for key, bucket := range rl.buckets {
				if bucket.Tokens() >= rl.capacity*0.95 && time.Since(bucket.lastRefill) > 5*time.Minute {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Stop terminates the cleanup goroutine.
func (rl *APIRateLimiter) Stop() {
	close(rl.stop)
}

func (rl *APIRateLimiter) LimitByUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket upgrades must not be rate-limited (long-lived connections).
		switch c.Request.URL.Path {
		case "/ws", "/ws/beacon":
			c.Next()
			return
		}

		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		rl.mu.Lock()
		if rl.whitelist[ip] {
			rl.mu.Unlock()
			c.Next()
			return
		}
		rl.mu.Unlock()

		userID, exists := c.Get("user_id")
		var key string
		if exists {
			key = "user:" + toString(userID)
		} else {
			key = "ip:" + ip
		}

		bucket := rl.GetBucket(key)

		limit := int(rl.capacity)
		remaining := int(bucket.Tokens())
		resetTime := time.Now().Add(time.Duration(float64(limit-remaining+1)/rl.rate) * time.Second).Unix()

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining-1))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))

		if !bucket.Allow() {
			retryAfter := int((1.0 - bucket.Tokens()) / rl.rate)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.Header("X-RateLimit-Remaining", "0")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests. Please try again later.",
				"retry_after": retryAfter,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (rl *APIRateLimiter) GetStatus() map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	buckets := make(map[string]interface{})
	for key, bucket := range rl.buckets {
		buckets[key] = map[string]interface{}{
			"tokens":      bucket.Tokens(),
			"capacity":    rl.capacity,
			"rate":        rl.rate,
			"last_refill": bucket.lastRefill,
		}
	}
	return map[string]interface{}{
		"capacity":  rl.capacity,
		"rate":      rl.rate,
		"buckets":   buckets,
		"whitelist": rl.getWhitelistSlice(),
	}
}

func (rl *APIRateLimiter) getWhitelistSlice() []string {
	ips := make([]string, 0, len(rl.whitelist))
	for ip := range rl.whitelist {
		ips = append(ips, ip)
	}
	return ips
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case float64:
		return strconv.Itoa(int(val))
	case string:
		return val
	default:
		return ""
	}
}
