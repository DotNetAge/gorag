package cache

import (
	"context"
	"math"
	"sync"
)

var _ SemanticCache = (*InMemorySemanticCache)(nil)

type cacheEntry struct {
	Embedding []float32
	Response  string
}

// InMemorySemanticCache is a lightweight, concurrent-safe semantic cache.
// For production, this could be backed by Redis + Vector Search (like RedisVL).
type InMemorySemanticCache struct {
	entries []cacheEntry
	lock    sync.RWMutex
}

func NewInMemorySemanticCache() *InMemorySemanticCache {
	return &InMemorySemanticCache{
		entries: make([]cacheEntry, 0),
	}
}

func (c *InMemorySemanticCache) Get(ctx context.Context, queryEmbedding []float32, threshold float32) (string, bool, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	var bestMatch string
	var highestSim float32 = -1.0

	for _, entry := range c.entries {
		sim := cosineSimilarity(queryEmbedding, entry.Embedding)
		if sim > highestSim {
			highestSim = sim
			bestMatch = entry.Response
		}
	}

	if highestSim >= threshold {
		return bestMatch, true, nil
	}

	return "", false, nil
}

func (c *InMemorySemanticCache) Set(ctx context.Context, queryEmbedding []float32, response string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.entries = append(c.entries, cacheEntry{
		Embedding: queryEmbedding,
		Response:  response,
	})
	return nil
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
