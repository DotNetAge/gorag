package rag

import (
	"context"
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
}

// MemoryCache implements a simple in-memory cache
type MemoryCache struct {
	cache    map[string]*cacheItem
	expiry   time.Duration
	mutex    chan struct{}
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
		mutex:  make(chan struct{}, 1),
	}

	// Start a goroutine to clean up expired items
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			cache.cleanup()
		}
	}()

	return cache
}

// Get retrieves a cached response for the given key
func (c *MemoryCache) Get(ctx context.Context, key string) (*Response, bool) {
	c.lock()
	defer c.unlock()

	item, ok := c.cache[key]
	if !ok {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		delete(c.cache, key)
		return nil, false
	}

	return item.value, true
}

// Set stores a response in the cache with the given key and expiration
func (c *MemoryCache) Set(ctx context.Context, key string, value *Response, expiration time.Duration) {
	c.lock()
	defer c.unlock()

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
	c.lock()
	defer c.unlock()

	delete(c.cache, key)
}

// Clear removes all cached responses
func (c *MemoryCache) Clear(ctx context.Context) {
	c.lock()
	defer c.unlock()

	c.cache = make(map[string]*cacheItem)
}

// cleanup removes expired items from the cache
func (c *MemoryCache) cleanup() {
	c.lock()
	defer c.unlock()

	now := time.Now()
	for key, item := range c.cache {
		if now.After(item.expiration) {
			delete(c.cache, key)
		}
	}
}

// lock acquires the mutex
func (c *MemoryCache) lock() {
	c.mutex <- struct{}{}
}

// unlock releases the mutex
func (c *MemoryCache) unlock() {
	<-c.mutex
}
