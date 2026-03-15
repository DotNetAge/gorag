package service

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// SemanticCacheService provides semantic caching functionality.
type SemanticCacheService struct {
	cache     abstraction.SemanticCache
	threshold float32
	logger    logging.Logger
	collector observability.Collector
}

// NewSemanticCacheService creates a new semantic cache service with logger and metrics.
func NewSemanticCacheService(cache abstraction.SemanticCache, threshold float32, logger logging.Logger, collector observability.Collector) *SemanticCacheService {
	if threshold <= 0 {
		threshold = 0.98 // Default high threshold for exact match caching
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if collector == nil {
		collector = observability.NewNoopCollector()
	}
	return &SemanticCacheService{
		cache:     cache,
		threshold: threshold,
		logger:    logger,
		collector: collector,
	}
}

// CacheCheckResult holds the result of a cache check.
type CacheCheckResult struct {
	Hit    bool
	Answer string
}

// CheckCache checks if the query has a cached response.
func (s *SemanticCacheService) CheckCache(ctx context.Context, query *entity.Query) (*CacheCheckResult, error) {
	start := time.Now()
	defer func() {
		s.collector.RecordDuration("cache_check", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return &CacheCheckResult{Hit: false}, nil
	}

	// Get embedding from metadata
	embedding, ok := query.Metadata["embedding"].([]float32)
	if !ok || len(embedding) == 0 {
		return &CacheCheckResult{Hit: false}, nil
	}

	// Check cache
	cachedResponse, found, err := s.cache.Get(ctx, embedding, s.threshold)
	if err != nil {
		s.logger.Warn("cache get error", map[string]interface{}{
			"error": err,
			"query": query.Text,
		})
		s.collector.RecordCount("cache_check", "error", nil)
		return nil, fmt.Errorf("SemanticCacheService: cache get error: %w", err)
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

	return &CacheCheckResult{
		Hit:    found,
		Answer: cachedResponse,
	}, nil
}

// CacheResponse caches a query-answer pair.
func (s *SemanticCacheService) CacheResponse(ctx context.Context, query *entity.Query, answer string) error {
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
		return fmt.Errorf("SemanticCacheService: cache set error: %w", err)
	}

	s.logger.Info("response cached", map[string]interface{}{
		"query":         query.Text,
		"answer_length": len(answer),
	})
	s.collector.RecordCount("cache_set", "success", nil)

	return nil
}
