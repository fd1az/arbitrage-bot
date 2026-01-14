// Package cache provides a generic in-memory cache with TTL support.
package cache

import (
	"context"
	"sync"
	"time"
)

// Cache is a generic thread-safe in-memory cache with TTL.
type Cache[K comparable, V any] struct {
	items    map[K]*item[V]
	mu       sync.RWMutex
	stopCh   chan struct{}
	stats    Stats
	statsMu  sync.RWMutex
}

type item[V any] struct {
	value     V
	expiresAt time.Time
}

// Stats holds cache statistics.
type Stats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	ItemCount  int64
}

// New creates a new Cache with background cleanup.
func New[K comparable, V any](cleanupInterval time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		items:  make(map[K]*item[V]),
		stopCh: make(chan struct{}),
	}

	go c.cleanup(cleanupInterval)

	return c
}

// Get retrieves a value from the cache.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	it, ok := c.items[key]
	if !ok {
		c.recordMiss()
		var zero V
		return zero, false
	}

	if time.Now().After(it.expiresAt) {
		c.recordMiss()
		var zero V
		return zero, false
	}

	c.recordHit()
	return it.value, true
}

// Set stores a value in the cache with the given TTL.
func (c *Cache[K, V]) Set(ctx context.Context, key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &item[V]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	c.statsMu.Lock()
	c.stats.ItemCount = int64(len(c.items))
	c.statsMu.Unlock()
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(ctx context.Context, key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Stats returns current cache statistics.
func (c *Cache[K, V]) Stats() Stats {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	return c.stats
}

// Close stops the background cleanup goroutine.
func (c *Cache[K, V]) Close() {
	close(c.stopCh)
}

func (c *Cache[K, V]) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.evictExpired()
		}
	}
}

func (c *Cache[K, V]) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evicted := int64(0)
	for key, it := range c.items {
		if now.After(it.expiresAt) {
			delete(c.items, key)
			evicted++
		}
	}

	c.statsMu.Lock()
	c.stats.Evictions += evicted
	c.stats.ItemCount = int64(len(c.items))
	c.statsMu.Unlock()
}

func (c *Cache[K, V]) recordHit() {
	c.statsMu.Lock()
	c.stats.Hits++
	c.statsMu.Unlock()
}

func (c *Cache[K, V]) recordMiss() {
	c.statsMu.Lock()
	c.stats.Misses++
	c.statsMu.Unlock()
}
