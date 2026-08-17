package db

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	key       string
	data      T
	timestamp time.Time
}

type TTLCache[T any] struct {
	mu      sync.RWMutex
	items   map[string]*list.Element
	order   *list.List
	maxSize int
	ttl     time.Duration
}

func NewTTLCache[T any](maxSize int, ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *TTLCache[T]) Get(key string) (T, bool) {
	var zero T
	c.mu.RLock()
	elem, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		return zero, false
	}
	entry := elem.Value.(*cacheEntry[T])
	if time.Since(entry.timestamp) >= c.ttl {
		c.mu.RUnlock()
		c.mu.Lock()
		// Re-check under the write lock: another goroutine may have
		// refreshed the entry between RUnlock and Lock (TOCTOU).
		if elem, ok := c.items[key]; ok {
			entry := elem.Value.(*cacheEntry[T])
			if time.Since(entry.timestamp) >= c.ttl {
				c.order.Remove(elem)
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
		return zero, false
	}
	data := entry.data
	c.mu.RUnlock()
	return data, true
}

func (c *TTLCache[T]) Set(key string, data T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value.(*cacheEntry[T]).data = data
		elem.Value.(*cacheEntry[T]).timestamp = time.Now()
		c.order.MoveToFront(elem)
		return
	}

	for len(c.items) >= c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		entry := back.Value.(*cacheEntry[T])
		delete(c.items, entry.key)
		c.order.Remove(back)
	}

	elem := c.order.PushFront(&cacheEntry[T]{key: key, data: data, timestamp: time.Now()})
	c.items[key] = elem
}

func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.Remove(elem)
		delete(c.items, key)
	}
}

func (c *TTLCache[T]) InvalidateByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			if elem, ok := c.items[k]; ok {
				c.order.Remove(elem)
			}
			delete(c.items, k)
		}
	}
}

func (c *TTLCache[T]) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.maxSize)
	c.order = list.New()
	c.mu.Unlock()
}

func (c *TTLCache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}