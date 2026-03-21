package cache

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"time"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// Cache provides semantic caching functionality.
type Cache struct {
	cache     SemanticCache
	threshold float32
	logger    logging.Logger
	collector observability.Collector
}

// Option configures a Cache instance.
type Option func(*Cache)

// WithCacheThreshold sets the similarity threshold for cache hits.
func WithCacheThreshold(threshold float32) Option {
	return func(s *Cache) {
		if threshold > 0 {
			s.threshold = threshold
		}
	}
}

// WithSemanticCacheLogger sets a structured logger.
func WithLogger(logger logging.Logger) Option {
	return func(s *Cache) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithSemanticCacheCollector sets an observability collector.
func WithCollector(collector observability.Collector) Option {
	return func(s *Cache) {
		if collector != nil {
			s.collector = collector
		}
	}
}

// New creates a new cache service.
//
// Required: cache.
// Optional (via options): WithCacheThreshold (default 0.98), WithLogger, WithCollector.
func New(cache SemanticCache, opts ...Option) *Cache {
	s := &Cache{
		cache:     cache,
		threshold: 0.98,
		logger:    logging.DefaultNoopLogger(),
		collector: observability.DefaultNoopCollector(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CheckResult holds the result of a cache check.
type CheckResult struct {
	Hit    bool
	Answer string
}

// Check checks if the query has a cached response.
func (s *Cache) Check(ctx context.Context, query *core.Query) (*CheckResult, error) {
	start := time.Now()
	defer func() {
		s.collector.RecordDuration("cache_check", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return &CheckResult{Hit: false}, nil
	}

	// Get embedding from metadata
	embedding, ok := query.Metadata["embedding"].([]float32)
	if !ok || len(embedding) == 0 {
		return &CheckResult{Hit: false}, nil
	}

	// Check cache
	cachedResponse, found, err := s.cache.Get(ctx, embedding, s.threshold)
	if err != nil {
		s.logger.Warn("cache get error", map[string]interface{}{
			"error": err,
			"query": query.Text,
		})
		s.collector.RecordCount("cache_check", "error", nil)
		return nil, fmt.Errorf("cache: get error: %w", err)
	}

	if found {
		s.logger.Info("cache hit", map[string]interface{}{
			"query": query.Text,
		})
		s.collector.RecordCount("cache_check", "hit", nil)
	} else {
		s.logger.Debug("cache miss", map[string]interface{}{
			"query": query.Text,
		})
		s.collector.RecordCount("cache_check", "miss", nil)
	}

	return &CheckResult{
		Hit:    found,
		Answer: cachedResponse,
	}, nil
}

// Set caches a query-answer pair.
func (s *Cache) Set(ctx context.Context, query *core.Query, answer string) error {
	start := time.Now()
	defer func() {
		s.collector.RecordDuration("cache_set", time.Since(start), nil)
	}()

	if query == nil || answer == "" {
		return nil
	}

	// Get embedding from metadata
	embedding, ok := query.Metadata["embedding"].([]float32)
	if !ok || len(embedding) == 0 {
		return nil
	}

	// Cache the response
	err := s.cache.Set(ctx, embedding, answer)
	if err != nil {
		s.logger.Error("cache set error", err, map[string]interface{}{
			"query":         query.Text,
			"answer_length": len(answer),
		})
		s.collector.RecordCount("cache_set", "error", nil)
		return fmt.Errorf("cache: set error: %w", err)
	}

	s.logger.Info("response cached", map[string]interface{}{
		"query":         query.Text,
		"answer_length": len(answer),
	})
	s.collector.RecordCount("cache_set", "success", nil)

	return nil
}
