package db

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value with its expiration time.
type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

// TTLCache is a thread-safe in-memory cache with TTL expiration and max size.
type TTLCache struct {
	mu       sync.RWMutex
	items    map[string]cacheEntry
	maxSize  int
	ttl      time.Duration
}

// NewTTLCache creates a cache with the given max entries and TTL.
// When the cache is full, the oldest entry is evicted.
func NewTTLCache(maxSize int, ttl time.Duration) *TTLCache {
	return &TTLCache{
		items:   make(map[string]cacheEntry, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a value from the cache. Returns the value and true if found and not expired.
func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}
	if time.Since(entry.timestamp) >= c.ttl {
		c.Delete(key)
		return nil, false
	}
	return entry.data, true
}

// Set stores a value in the cache. Evicts the oldest entry if at capacity.
func (c *TTLCache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing key — no eviction needed.
	if _, ok := c.items[key]; ok {
		c.items[key] = cacheEntry{data: data, timestamp: time.Now()}
		return
	}

	// Evict oldest if at capacity.
	if len(c.items) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.items {
			if oldestKey == "" || v.timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.timestamp
			}
		}
		if oldestKey != "" {
			delete(c.items, oldestKey)
		}
	}

	c.items[key] = cacheEntry{data: data, timestamp: time.Now()}
}

// Delete removes a key from the cache.
func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// InvalidateByPrefix removes all keys that start with the given prefix.
func (c *TTLCache) InvalidateByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
}

// Clear removes all entries from the cache.
func (c *TTLCache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]cacheEntry, c.maxSize)
	c.mu.Unlock()
}

// Len returns the current number of entries in the cache.
func (c *TTLCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
