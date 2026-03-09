package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// MemoryCache implements the Cache interface for query results
type MemoryCache struct {
	data       map[string]*cacheEntry
	mu         sync.RWMutex
	defaultTTL time.Duration
	maxSize    int
}

// cacheEntry represents a cached entry
type cacheEntry struct {
	value       *Response
	expiresAt   time.Time
	accessedAt  time.Time
	accessCount int
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(defaultTTL time.Duration) *MemoryCache {
	return &MemoryCache{
		data:       make(map[string]*cacheEntry),
		defaultTTL: defaultTTL,
		maxSize:    1000,
	}
}

// NewMemoryCacheWithSize creates a new in-memory cache with size limit
func NewMemoryCacheWithSize(defaultTTL time.Duration, maxSize int) *MemoryCache {
	return &MemoryCache{
		data:       make(map[string]*cacheEntry),
		defaultTTL: defaultTTL,
		maxSize:    maxSize,
	}
}

// Get retrieves a value from the cache
// Implements the Cache interface
func (c *MemoryCache) Get(ctx context.Context, key string) (*Response, bool) {
	c.mu.RLock()
	entry, exists := c.data[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return nil, false
	}

	// Update access stats
	c.mu.Lock()
	entry.accessedAt = time.Now()
	entry.accessCount++
	c.mu.Unlock()

	return entry.value, true
}

// Set stores a value in the cache
func (c *MemoryCache) Set(ctx context.Context, key string, value *Response, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.data) >= c.maxSize {
		c.evictLRU()
	}

	c.data[key] = &cacheEntry{
		value:       value,
		expiresAt:   time.Now().Add(ttl),
		accessedAt:  time.Now(),
		accessCount: 1,
	}
}

// evictLRU evicts the least recently used entry
func (c *MemoryCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.data {
		if oldestKey == "" || entry.accessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.accessedAt
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear clears all entries from the cache
func (c *MemoryCache) Clear(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]*cacheEntry)
}

// Size returns the number of entries in the cache
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// Stats returns cache statistics
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalAccesses int
	var expired int
	now := time.Now()

	for _, entry := range c.data {
		totalAccesses += entry.accessCount
		if now.After(entry.expiresAt) {
			expired++
		}
	}

	return CacheStats{
		Size:          len(c.data),
		MaxSize:       c.maxSize,
		Expired:       expired,
		TotalAccesses: totalAccesses,
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	Size          int
	MaxSize       int
	Expired       int
	TotalAccesses int
}

// GenerateCacheKey generates a cache key from a question and options
func GenerateCacheKey(question string, opts QueryOptions) string {
	data := fmt.Sprintf("%s|%s|%d|%v|%v|%d",
		question,
		opts.PromptTemplate,
		opts.TopK,
		opts.UseMultiHopRAG,
		opts.UseAgenticRAG,
		opts.MaxHops,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
