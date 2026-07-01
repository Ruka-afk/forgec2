package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type TokenBucket struct {
	capacity     float64
	rate         float64
	tokens       float64
	lastRefill   time.Time
	mu           sync.Mutex
}

func NewTokenBucket(capacity, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		rate:       rate,
		tokens:     capacity,
		lastRefill: time.Now(),
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

type APIRateLimiter struct {
	buckets   map[string]*TokenBucket
	mu        sync.Mutex
	capacity  float64
	rate      float64
	whitelist map[string]bool
}

func NewAPIRateLimiter(capacity, rate float64) *APIRateLimiter {
	rl := &APIRateLimiter{
		buckets:   make(map[string]*TokenBucket),
		capacity:  capacity,
		rate:      rate,
		whitelist: make(map[string]bool),
	}
	go rl.cleanup()
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

func (rl *APIRateLimiter) AddWhitelist(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.whitelist[ip] = true
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
	if !exists {
		bucket = NewTokenBucket(rl.capacity, rl.rate)
		rl.buckets[key] = bucket
	}
	return bucket
}

func (rl *APIRateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		for key, bucket := range rl.buckets {
			if bucket.Tokens() >= rl.capacity*0.95 && time.Since(bucket.lastRefill) > 5*time.Minute {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *APIRateLimiter) LimitByUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket upgrades must not be rate-limited (long-lived connections).
		switch c.Request.URL.Path {
		case "/ws", "/ws/chat", "/ws/beacon":
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

func (rl *APIRateLimiter) LimitByIP() gin.HandlerFunc {
	return func(c *gin.Context) {
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

		key := "ip:" + ip
		bucket := rl.GetBucket(key)

		limit := int(rl.capacity)
		remaining := int(bucket.Tokens())
		resetTime := time.Now().Add(time.Duration(float64(limit-remaining+1)/rl.rate) * time.Second).Unix()

		c.Header("X-RateLimit-Limit", toString(limit))
		c.Header("X-RateLimit-Remaining", toString(remaining-1))
		c.Header("X-RateLimit-Reset", toString(resetTime))

		if !bucket.Allow() {
			retryAfter := int((1.0 - bucket.Tokens()) / rl.rate)
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", toString(retryAfter))
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
			"tokens":     bucket.Tokens(),
			"capacity":   rl.capacity,
			"rate":       rl.rate,
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

func (rl *APIRateLimiter) UpdateConfig(capacity, rate float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.capacity = capacity
	rl.rate = rate
	for _, bucket := range rl.buckets {
		bucket.capacity = capacity
		bucket.rate = rate
		if bucket.tokens > capacity {
			bucket.tokens = capacity
		}
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case int:
		return itoa(val)
	case int64:
		return itoa(int(val))
	case uint:
		return itoa(int(val))
	case float64:
		return itoa(int(val))
	case string:
		return val
	default:
		return ""
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

type RouteRateLimiter struct {
	routes map[string]*APIRateLimiter
	mu     sync.Mutex
	defaultLimiter *APIRateLimiter
	whitelist []string
}

func NewRouteRateLimiter(defaultCapacity, defaultRate float64) *RouteRateLimiter {
	return &RouteRateLimiter{
		routes:         make(map[string]*APIRateLimiter),
		defaultLimiter: NewAPIRateLimiter(defaultCapacity, defaultRate),
	}
}

func (rrl *RouteRateLimiter) SetRouteLimit(route string, capacity, rate float64) {
	rrl.mu.Lock()
	defer rrl.mu.Unlock()
	rrl.routes[route] = NewAPIRateLimiter(capacity, rate)
	rrl.routes[route].SetWhitelist(rrl.whitelist)
}

func (rrl *RouteRateLimiter) RemoveRouteLimit(route string) {
	rrl.mu.Lock()
	defer rrl.mu.Unlock()
	delete(rrl.routes, route)
}

func (rrl *RouteRateLimiter) SetWhitelist(ips []string) {
	rrl.mu.Lock()
	defer rrl.mu.Unlock()
	rrl.whitelist = ips
	rrl.defaultLimiter.SetWhitelist(ips)
	for _, rl := range rrl.routes {
		rl.SetWhitelist(ips)
	}
}

func (rrl *RouteRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		rrl.mu.Lock()
		limiter, exists := rrl.routes[route]
		rrl.mu.Unlock()

		if !exists {
			limiter = rrl.defaultLimiter
		}

		handler := limiter.LimitByUser()
		handler(c)
	}
}

func (rrl *RouteRateLimiter) GetStatus() map[string]interface{} {
	rrl.mu.Lock()
	defer rrl.mu.Unlock()
	routes := make(map[string]interface{})
	for route, limiter := range rrl.routes {
		routes[route] = limiter.GetStatus()
	}
	return map[string]interface{}{
		"default": rrl.defaultLimiter.GetStatus(),
		"routes":  routes,
	}
}
