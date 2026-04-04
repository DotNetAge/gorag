package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	bolt "go.etcd.io/bbolt"
)

var _ core.SemanticCache = (*BoltSemanticCache)(nil)

const (
	cacheBucket = "semantic_cache"
)

type BoltSemanticCache struct {
	embedder  embedding.Provider
	config    *Config
	db        *bolt.DB
	threshold float32
}

type BoltOption func(*BoltSemanticCache)

func WithBoltDBPath(path string) BoltOption {
	return func(c *BoltSemanticCache) {
		if path != "" {
			c.config.DBPath = path
		}
	}
}

func WithBoltThreshold(threshold float32) BoltOption {
	return func(c *BoltSemanticCache) {
		if threshold > 0 && threshold <= 1.0 {
			c.threshold = threshold
		}
	}
}

func DefaultBoltSemanticCache(embedder embedding.Provider) (*BoltSemanticCache, error) {
	return NewBoltSemanticCache(embedder, WithBoltDBPath("gorag_cache.db"))
}

func NewBoltSemanticCache(embedder embedding.Provider, opts ...BoltOption) (*BoltSemanticCache, error) {
	cfg := defaultConfig()
	cache := &BoltSemanticCache{
		embedder:  embedder,
		config:    cfg,
		threshold: cfg.Threshold,
	}

	for _, opt := range opts {
		opt(cache)
	}

	dir := filepath.Dir(cfg.DBPath)
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
		}
	}

	db, err := bolt.Open(cfg.DBPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}
	cache.db = db

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(cacheBucket))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize bucket: %w", err)
	}

	return cache, nil
}

func (c *BoltSemanticCache) CheckCache(ctx context.Context, query *core.Query) (*core.CacheResult, error) {
	if query == nil || query.Text == "" {
		return &core.CacheResult{Hit: false}, nil
	}

	queryEmbeddingMatrix, err := c.embedder.Embed(ctx, []string{query.Text})
	if err != nil || len(queryEmbeddingMatrix) == 0 {
		return nil, err
	}
	queryEmbedding := queryEmbeddingMatrix[0]

	var bestMatch *boltCacheEntry
	var highestSim float32 = -1.0

	err = c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cacheBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		now := time.Now()

		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var entry boltCacheEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}

			if c.config.TTL > 0 && now.Sub(entry.CreatedAt) > c.config.TTL {
				continue
			}

			sim := cosineSimilarity(queryEmbedding, entry.Embedding)
			if sim > highestSim {
				highestSim = sim
				bestMatch = &entry
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if bestMatch != nil && highestSim >= c.threshold {
		return &core.CacheResult{
			Hit:    true,
			Answer: bestMatch.Response,
		}, nil
	}

	return &core.CacheResult{Hit: false}, nil
}

func (c *BoltSemanticCache) CacheResponse(ctx context.Context, query *core.Query, answer *core.Result) error {
	if query == nil || query.Text == "" || answer == nil {
		return nil
	}

	queryEmbeddingMatrix, err := c.embedder.Embed(ctx, []string{query.Text})
	if err != nil || len(queryEmbeddingMatrix) == 0 {
		return err
	}
	queryEmbedding := queryEmbeddingMatrix[0]

	entry := boltCacheEntry{
		Key:       query.Text,
		QueryText: query.Text,
		Embedding: queryEmbedding,
		Response:  answer.Answer,
		CreatedAt: time.Now(),
		LastHitAt: time.Now(),
		HitCount:  0,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cacheBucket))
		return bucket.Put([]byte(query.Text), data)
	})
}

func (c *BoltSemanticCache) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *BoltSemanticCache) Size() (int, error) {
	var count int
	err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cacheBucket))
		if bucket == nil {
			return nil
		}
		count = bucket.Stats().KeyN
		return nil
	})
	return count, err
}

func (c *BoltSemanticCache) CleanExpired(ctx context.Context) (int, error) {
	if c.config.TTL <= 0 {
		return 0, nil
	}

	removed := 0
	now := time.Now()

	err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cacheBucket))
		if bucket == nil {
			return nil
		}

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var entry boltCacheEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				continue
			}

			if now.Sub(entry.CreatedAt) > c.config.TTL {
				if err := bucket.Delete(k); err != nil {
					return err
				}
				removed++
			}
		}
		return nil
	})

	return removed, err
}

type boltCacheEntry struct {
	Key       string
	QueryText string
	Embedding []float32
	Response  string
	CreatedAt time.Time
	LastHitAt time.Time
	HitCount  int
}
