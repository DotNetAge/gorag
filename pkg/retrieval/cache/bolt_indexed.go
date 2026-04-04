package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/store/vector/govector"
	bolt "go.etcd.io/bbolt"
)

var _ core.SemanticCache = (*IndexedBoltSemanticCache)(nil)

// IndexedBoltSemanticCache is an optimized version of BoltSemanticCache with vector indexing.
// It uses govector for fast similarity search instead of full scan.
type IndexedBoltSemanticCache struct {
	embedder  embedding.Provider
	config    *Config
	db        *bolt.DB
	vectorStore core.VectorStore
	threshold float32
	mu        sync.RWMutex
}

type IndexedBoltOption func(*IndexedBoltSemanticCache)

func WithIndexedDBPath(path string) IndexedBoltOption {
	return func(c *IndexedBoltSemanticCache) {
		if path != "" {
			c.config.DBPath = path
		}
	}
}

func WithIndexedThreshold(threshold float32) IndexedBoltOption {
	return func(c *IndexedBoltSemanticCache) {
		if threshold > 0 && threshold <= 1.0 {
			c.threshold = threshold
		}
	}
}

// NewIndexedBoltSemanticCache creates a new indexed bolt cache with vector indexing for O(log n) search.
func NewIndexedBoltSemanticCache(embedder embedding.Provider, opts ...IndexedBoltOption) (*IndexedBoltSemanticCache, error) {
	cfg := defaultConfig()
	cache := &IndexedBoltSemanticCache{
		embedder:  embedder,
		config:    cfg,
		threshold: cfg.Threshold,
	}

	for _, opt := range opts {
		opt(cache)
	}

	// Create directory if needed
	dir := filepath.Dir(cache.config.DBPath)
	if dir != "" && dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
		}
	}

	// Initialize BoltDB for metadata
	db, err := bolt.Open(cache.config.DBPath, 0600, nil)
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

	// Initialize vector store for fast similarity search
	vecPath := cache.config.DBPath + ".vectors"
	vectorStore, err := govector.NewStore(
		govector.WithDBPath(vecPath),
		govector.WithDimension(embedder.Dimension()),
		govector.WithCollection("semantic_cache"),
		govector.WithHNSW(true),
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize vector store: %w", err)
	}
	cache.vectorStore = vectorStore

	return cache, nil
}

func (c *IndexedBoltSemanticCache) CheckCache(ctx context.Context, query *core.Query) (*core.CacheResult, error) {
	if query == nil || query.Text == "" {
		return &core.CacheResult{Hit: false}, nil
	}

	queryEmbeddingMatrix, err := c.embedder.Embed(ctx, []string{query.Text})
	if err != nil || len(queryEmbeddingMatrix) == 0 {
		return nil, err
	}
	queryEmbedding := queryEmbeddingMatrix[0]

	// Use vector search instead of full scan (O(log n) vs O(n))
	vectors, scores, err := c.vectorStore.Search(ctx, queryEmbedding, 1, nil)
	if err != nil || len(vectors) == 0 {
		return &core.CacheResult{Hit: false}, nil
	}

	// Check if similarity exceeds threshold
	if scores[0] < c.threshold {
		return &core.CacheResult{Hit: false}, nil
	}

	// Retrieve full entry from BoltDB
	var response string
	err = c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cacheBucket))
		if bucket == nil {
			return nil
		}
		data := bucket.Get([]byte(vectors[0].ID))
		if data == nil {
			return nil
		}
		var entry boltCacheEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}
		
		// Check TTL
		if c.config.TTL > 0 && time.Since(entry.CreatedAt) > c.config.TTL {
			return nil
		}
		
		response = entry.Response
		return nil
	})

	if err != nil || response == "" {
		return &core.CacheResult{Hit: false}, nil
	}

	return &core.CacheResult{
		Hit:    true,
		Answer: response,
	}, nil
}

func (c *IndexedBoltSemanticCache) CacheResponse(ctx context.Context, query *core.Query, answer *core.Result) error {
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

	// Store in BoltDB
	err = c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(cacheBucket))
		return bucket.Put([]byte(query.Text), data)
	})
	if err != nil {
		return err
	}

	// Store in vector index
	vector := &core.Vector{
		ID:       query.Text,
		Values:   queryEmbedding,
		Metadata: map[string]any{"created_at": entry.CreatedAt.Unix()},
	}
	return c.vectorStore.Upsert(ctx, []*core.Vector{vector})
}

func (c *IndexedBoltSemanticCache) Close() error {
	var errs []error
	
	if c.vectorStore != nil {
		if err := c.vectorStore.Close(context.Background()); err != nil {
			errs = append(errs, err)
		}
	}
	
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing cache: %v", errs)
	}
	return nil
}

func (c *IndexedBoltSemanticCache) Size() (int, error) {
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

func (c *IndexedBoltSemanticCache) CleanExpired(ctx context.Context) (int, error) {
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
				// Delete from BoltDB
				if err := bucket.Delete(k); err != nil {
					return err
				}
				
				// Delete from vector store
				if err := c.vectorStore.Delete(ctx, string(k)); err != nil {
					// Log but continue
					continue
				}
				
				removed++
			}
		}
		return nil
	})

	return removed, err
}
