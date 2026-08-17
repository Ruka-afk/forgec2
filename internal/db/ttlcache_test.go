package db

import (
	"sync"
	"testing"
	"time"
)

func TestTTLCache_GetSet(t *testing.T) {
	c := NewTTLCache[any](10, time.Minute)

	// Get on empty cache
	if _, ok := c.Get("key1"); ok {
		t.Fatal("expected miss on empty cache")
	}

	// Set and Get
	c.Set("key1", "value1")
	if v, ok := c.Get("key1"); !ok || v != "value1" {
		t.Fatalf("expected value1, got %v (ok=%v)", v, ok)
	}

	// Overwrite
	c.Set("key1", "value2")
	if v, ok := c.Get("key1"); !ok || v != "value2" {
		t.Fatalf("expected value2, got %v", v)
	}
}

func TestTTLCache_Expiration(t *testing.T) {
	c := NewTTLCache[any](10, 50*time.Millisecond)

	c.Set("expire", "soon")
	if _, ok := c.Get("expire"); !ok {
		t.Fatal("expected hit before expiry")
	}

	time.Sleep(80 * time.Millisecond)

	if _, ok := c.Get("expire"); ok {
		t.Fatal("expected miss after expiry")
	}
}

func TestTTLCache_Eviction(t *testing.T) {
	c := NewTTLCache[any](3, time.Minute)

	c.Set("a", 1)
	time.Sleep(time.Millisecond) // ensure different timestamps
	c.Set("b", 2)
	time.Sleep(time.Millisecond)
	c.Set("c", 3)

	// At capacity; adding one more should evict oldest ("a")
	time.Sleep(time.Millisecond)
	c.Set("d", 4)

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected 'a' to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Fatal("expected 'b' to still exist")
	}
	if _, ok := c.Get("d"); !ok {
		t.Fatal("expected 'd' to exist")
	}
}

func TestTTLCache_Delete(t *testing.T) {
	c := NewTTLCache[any](10, time.Minute)
	c.Set("x", 10)
	c.Delete("x")
	if _, ok := c.Get("x"); ok {
		t.Fatal("expected miss after delete")
	}
}

func TestTTLCache_InvalidateByPrefix(t *testing.T) {
	c := NewTTLCache[any](10, time.Minute)
	c.Set("users:1", "alice")
	c.Set("users:2", "bob")
	c.Set("agents:1", "implant1")

	c.InvalidateByPrefix("users:")

	if _, ok := c.Get("users:1"); ok {
		t.Fatal("expected users:1 to be invalidated")
	}
	if _, ok := c.Get("users:2"); ok {
		t.Fatal("expected users:2 to be invalidated")
	}
	if _, ok := c.Get("agents:1"); !ok {
		t.Fatal("expected agents:1 to still exist")
	}
}

func TestTTLCache_Clear(t *testing.T) {
	c := NewTTLCache[any](10, time.Minute)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("expected empty cache, got %d entries", c.Len())
	}
}

func TestTTLCache_Len(t *testing.T) {
	c := NewTTLCache[any](10, time.Minute)
	if c.Len() != 0 {
		t.Fatal("expected 0")
	}
	c.Set("a", 1)
	c.Set("b", 2)
	if c.Len() != 2 {
		t.Fatalf("expected 2, got %d", c.Len())
	}
}

func TestTTLCache_ConcurrentAccess(t *testing.T) {
	c := NewTTLCache[any](100, time.Minute)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set("key", n)
			c.Get("key")
			c.Delete("key")
		}(i)
	}
	wg.Wait()
}

func TestTTLCache_WrapperFunctions(t *testing.T) {
	// Test that the wrapper functions work with the global queryCache
	ClearCache()

	SetCache("test:1", "hello")
	if v, ok := GetFromCache("test:1"); !ok || v != "hello" {
		t.Fatalf("expected 'hello', got %v", v)
	}

	SetCache("test:2", "world")
	InvalidateCache("test:")

	if _, ok := GetFromCache("test:1"); ok {
		t.Fatal("expected test:1 to be invalidated")
	}
	if _, ok := GetFromCache("test:2"); ok {
		t.Fatal("expected test:2 to be invalidated")
	}

	ClearCache()
}
