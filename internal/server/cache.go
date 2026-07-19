package server

import (
	"sync"
	"time"
)

// ttlCache is a small thread-safe value cache with a fixed TTL. It absorbs
// repeated dashboard/poll requests without serving stale data for long.
type ttlCache struct {
	mu       sync.RWMutex
	value    any
	cachedAt time.Time
	ttl      time.Duration
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl}
}

// get returns the cached value if it is still fresh; otherwise it calls
// compute, caches the result, and returns it. compute runs only on a miss.
func (c *ttlCache) get(compute func() (any, error)) (any, error) {
	c.mu.RLock()
	fresh := c.cachedAt != (time.Time{}) && time.Since(c.cachedAt) < c.ttl
	if fresh {
		v := c.value
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedAt != (time.Time{}) && time.Since(c.cachedAt) < c.ttl {
		return c.value, nil
	}
	v, err := compute()
	if err != nil {
		return nil, err
	}
	c.value = v
	c.cachedAt = time.Now()
	return v, nil
}

// invalidate forces the next get() to recompute.
func (c *ttlCache) invalidate() {
	c.mu.Lock()
	c.cachedAt = time.Time{}
	c.mu.Unlock()
}

// ttlCacheMap is a keyed variant of ttlCache for endpoints whose result
// depends on a query parameter (e.g. a time range). Each key gets its own
// TTL entry. Expired entries are periodically pruned to prevent memory leaks.
type ttlCacheMap struct {
	mu      sync.RWMutex
	m       map[string]*ttlCache
	ttl     time.Duration
	stopCh  chan struct{}
}

func newTTLCacheMap(ttl time.Duration) *ttlCacheMap {
	c := &ttlCacheMap{
		m:      make(map[string]*ttlCache),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

func (c *ttlCacheMap) get(key string, compute func() (any, error)) (any, error) {
	c.mu.RLock()
	ent, ok := c.m[key]
	c.mu.RUnlock()

	if !ok {
		c.mu.Lock()
		ent, ok = c.m[key]
		if !ok {
			ent = newTTLCache(c.ttl)
			c.m[key] = ent
		}
		c.mu.Unlock()
	}
	return ent.get(compute)
}

// invalidate forces the next get() for the given key to recompute.
func (c *ttlCacheMap) invalidate(key string) {
	c.mu.RLock()
	if ent, ok := c.m[key]; ok {
		ent.invalidate()
	}
	c.mu.RUnlock()
}

// invalidatePrefix forces recompute for all keys starting with prefix.
func (c *ttlCacheMap) invalidatePrefix(prefix string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, ent := range c.m {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			ent.invalidate()
		}
	}
}

// cleanupLoop periodically removes expired entries from the map.
func (c *ttlCacheMap) cleanupLoop() {
	ticker := time.NewTicker(CacheCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			for k, ent := range c.m {
				ent.mu.RLock()
				expired := ent.cachedAt != (time.Time{}) && time.Since(ent.cachedAt) >= ent.ttl*2
				ent.mu.RUnlock()
				if expired {
					delete(c.m, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Close stops the background cleanup goroutine.
func (c *ttlCacheMap) Close() {
	close(c.stopCh)
}
