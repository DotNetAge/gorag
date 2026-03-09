package rag

import (
	"context"
	"sync"
	"time"
)

// Cache defines the interface for query caching
type Cache interface {
	// Get retrieves a cached response for the given key
	Get(ctx context.Context, key string) (*Response, bool)
	// Set stores a response in the cache with the given key and expiration
	Set(ctx context.Context, key string, value *Response, expiration time.Duration)
	// Delete removes a cached response for the given key
	Delete(ctx context.Context, key string)
	// Clear removes all cached responses
	Clear(ctx context.Context)
	// Close stops the cache and releases resources
	Close() error
}

// MemoryCache implements a simple in-memory cache
type MemoryCache struct {
	cache  map[string]*cacheItem
	expiry time.Duration
	mu     sync.RWMutex
	done   chan struct{}
	closed bool
}

type cacheItem struct {
	value      *Response
	expiration time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(expiry time.Duration) *MemoryCache {
	cache := &MemoryCache{
		cache:  make(map[string]*cacheItem),
		expiry: expiry,
		done:   make(chan struct{}),
	}

	// Start a goroutine to clean up expired items
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cache.cleanup()
			case <-cache.done:
				return
			}
		}
	}()

	return cache
}

// Get retrieves a cached response for the given key
func (c *MemoryCache) Get(ctx context.Context, key string) (*Response, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

// Set stores a response in the cache with the given key and expiration
func (c *MemoryCache) Set(ctx context.Context, key string, value *Response, expiration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	if expiration == 0 {
		expiration = c.expiry
	}

	c.cache[key] = &cacheItem{
		value:      value,
		expiration: time.Now().Add(expiration),
	}
}

// Delete removes a cached response for the given key
func (c *MemoryCache) Delete(ctx context.Context, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

// Clear removes all cached responses
func (c *MemoryCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheItem)
}

// Close stops the cache cleanup goroutine and releases resources
func (c *MemoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.done)
	return nil
}

// cleanup removes expired items from the cache
func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.cache {
		if now.After(item.expiration) {
			delete(c.cache, key)
		}
	}
}
