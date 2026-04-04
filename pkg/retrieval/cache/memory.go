package cache

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
)

var _ core.SemanticCache = (*InMemorySemanticCache)(nil)

type cacheEntry struct {
	Key       string
	QueryText string
	Embedding []float32
	Response  string
	CreatedAt time.Time
	LastHitAt time.Time
	HitCount  int
}

type EvictPolicy string

const (
	EvictLRU  EvictPolicy = "lru"
	EvictFIFO EvictPolicy = "fifo"
	EvictLFU  EvictPolicy = "lfu"
)

type Config struct {
	MaxSize     int
	TTL         time.Duration
	Threshold   float32
	EvictPolicy EvictPolicy
	DBPath      string
}

type CacheOption func(*Config)

func WithMaxSize(n int) CacheOption {
	return func(c *Config) {
		if n > 0 {
			c.MaxSize = n
		}
	}
}

func WithTTL(d time.Duration) CacheOption {
	return func(c *Config) {
		if d > 0 {
			c.TTL = d
		}
	}
}

func WithThreshold(threshold float32) CacheOption {
	return func(c *Config) {
		if threshold > 0 && threshold <= 1.0 {
			c.Threshold = threshold
		}
	}
}

func WithEvictPolicy(p EvictPolicy) CacheOption {
	return func(c *Config) {
		c.EvictPolicy = p
	}
}

func WithDBPath(path string) CacheOption {
	return func(c *Config) {
		c.DBPath = path
	}
}

func defaultConfig() *Config {
	return &Config{
		MaxSize:     10000,
		TTL:         time.Hour,
		Threshold:   0.98,
		EvictPolicy: EvictLRU,
		DBPath:      "gorag_cache.db",
	}
}

type InMemorySemanticCache struct {
	embedder embedding.Provider
	config   *Config
	entries  map[string]*cacheEntry
	order    []string
	lock     sync.RWMutex
}

func NewInMemorySemanticCache(embedder embedding.Provider, opts ...CacheOption) *InMemorySemanticCache {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return &InMemorySemanticCache{
		embedder: embedder,
		config:   cfg,
		entries:  make(map[string]*cacheEntry),
		order:    make([]string, 0, cfg.MaxSize),
	}
}

func NewInMemorySemanticCacheWithConfig(embedder embedding.Provider, cfg *Config) *InMemorySemanticCache {
	if cfg == nil {
		cfg = defaultConfig()
	}

	return &InMemorySemanticCache{
		embedder: embedder,
		config:   cfg,
		entries:  make(map[string]*cacheEntry),
		order:    make([]string, 0, cfg.MaxSize),
	}
}

func (c *InMemorySemanticCache) CheckCache(ctx context.Context, query *core.Query) (*core.CacheResult, error) {
	if query == nil || query.Text == "" {
		return &core.CacheResult{Hit: false}, nil
	}

	queryEmbeddingMatrix, err := c.embedder.Embed(ctx, []string{query.Text})
	if err != nil || len(queryEmbeddingMatrix) == 0 {
		return nil, err
	}
	queryEmbedding := queryEmbeddingMatrix[0]

	c.lock.Lock()
	defer c.lock.Unlock()

	var bestMatchKey string
	var bestMatch *cacheEntry
	var highestSim float32 = -1.0

	now := time.Now()
	for key, entry := range c.entries {
		if c.config.TTL > 0 && now.Sub(entry.CreatedAt) > c.config.TTL {
			continue
		}

		sim := cosineSimilarity(queryEmbedding, entry.Embedding)
		if sim > highestSim {
			highestSim = sim
			bestMatchKey = key
			bestMatch = entry
		}
	}

	if bestMatch != nil && highestSim >= c.config.Threshold {
		bestMatch.LastHitAt = now
		bestMatch.HitCount++

		if c.config.EvictPolicy == EvictLRU {
			c.moveToEnd(bestMatchKey)
		}

		return &core.CacheResult{
			Hit:    true,
			Answer: bestMatch.Response,
		}, nil
	}

	return &core.CacheResult{Hit: false}, nil
}

func (c *InMemorySemanticCache) moveToEnd(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			break
		}
	}
}

func (c *InMemorySemanticCache) CacheResponse(ctx context.Context, query *core.Query, answer *core.Result) error {
	if query == nil || query.Text == "" || answer == nil {
		return nil
	}

	queryEmbeddingMatrix, err := c.embedder.Embed(ctx, []string{query.Text})
	if err != nil || len(queryEmbeddingMatrix) == 0 {
		return err
	}
	queryEmbedding := queryEmbeddingMatrix[0]

	c.lock.Lock()
	defer c.lock.Unlock()

	key := query.Text

	if existing, exists := c.entries[key]; exists {
		existing.Embedding = queryEmbedding
		existing.Response = answer.Answer
		existing.LastHitAt = time.Now()
		return nil
	}

	if len(c.entries) >= c.config.MaxSize {
		c.evict()
	}

	entry := &cacheEntry{
		Key:       key,
		QueryText: query.Text,
		Embedding: queryEmbedding,
		Response:  answer.Answer,
		CreatedAt: time.Now(),
		LastHitAt: time.Now(),
		HitCount:  0,
	}

	c.entries[key] = entry
	c.order = append(c.order, key)

	return nil
}

func (c *InMemorySemanticCache) evict() {
	if len(c.order) == 0 {
		return
	}

	var victim string

	switch c.config.EvictPolicy {
	case EvictLRU:
		victim = c.order[0]
	case EvictFIFO:
		victim = c.order[0]
	case EvictLFU:
		victim = c.findLFUVictim()
	default:
		victim = c.order[0]
	}

	delete(c.entries, victim)
	c.order = c.order[1:]
}

func (c *InMemorySemanticCache) findLFUVictim() string {
	var victim string
	var minHits int = int(^uint(0) >> 1)

	for _, key := range c.order {
		entry := c.entries[key]
		if entry.HitCount < minHits {
			minHits = entry.HitCount
			victim = key
		}
	}

	return victim
}

func (c *InMemorySemanticCache) CleanExpired(ctx context.Context) (int, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.config.TTL <= 0 {
		return 0, nil
	}

	now := time.Now()
	removed := 0
	var newOrder []string

	for _, key := range c.order {
		entry := c.entries[key]
		if now.Sub(entry.CreatedAt) > c.config.TTL {
			delete(c.entries, key)
			removed++
		} else {
			newOrder = append(newOrder, key)
		}
	}

	c.order = newOrder
	return removed, nil
}

func (c *InMemorySemanticCache) Size() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.entries)
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
