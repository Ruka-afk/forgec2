package db

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry struct {
	key       string
	data      interface{}
	timestamp time.Time
}

type TTLCache struct {
	mu      sync.RWMutex
	items   map[string]*list.Element
	order   *list.List
	maxSize int
	ttl     time.Duration
}

func NewTTLCache(maxSize int, ttl time.Duration) *TTLCache {
	return &TTLCache{
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *TTLCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	elem, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		return nil, false
	}
	entry := elem.Value.(*cacheEntry)
	if time.Since(entry.timestamp) >= c.ttl {
		c.mu.RUnlock()
		c.Delete(key)
		return nil, false
	}
	data := entry.data
	c.mu.RUnlock()
	return data, true
}

func (c *TTLCache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value.(*cacheEntry).data = data
		elem.Value.(*cacheEntry).timestamp = time.Now()
		c.order.MoveToFront(elem)
		return
	}

	for len(c.items) >= c.maxSize {
		back := c.order.Back()
		if back == nil {
			break
		}
		entry := back.Value.(*cacheEntry)
		delete(c.items, entry.key)
		c.order.Remove(back)
	}

	elem := c.order.PushFront(&cacheEntry{key: key, data: data, timestamp: time.Now()})
	c.items[key] = elem
}

func (c *TTLCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.Remove(elem)
		delete(c.items, key)
	}
}

func (c *TTLCache) InvalidateByPrefix(prefix string) {
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

func (c *TTLCache) Clear() {
	c.mu.Lock()
	c.items = make(map[string]*list.Element, c.maxSize)
	c.order = list.New()
	c.mu.Unlock()
}

func (c *TTLCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
