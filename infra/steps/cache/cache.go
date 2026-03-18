// Package cache provides semantic cache steps for RAG retrieval pipelines.
package cache

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/service"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// check checks semantic cache for existing answers.
type check struct {
	cacheService *service.SemanticCacheService
	logger       logging.Logger
	metrics      abstraction.Metrics
}

// Check creates a new semantic cache check step with logger and metrics.
//
// Parameters:
//   - cacheService: semantic cache service implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(cache.Check(cacheService, logger, metrics))
func Check(
	cacheService *service.SemanticCacheService,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &check{
		cacheService: cacheService,
		logger:       logger,
		metrics:      metrics,
	}
}

// Name returns the step name
func (s *check) Name() string {
	return "SemanticCacheCheck"
}

// Execute checks semantic cache using infra/service.
func (s *check) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("SemanticCacheCheck: query required")
	}

	s.logger.Debug("checking semantic cache", map[string]interface{}{
		"step":  "SemanticCacheCheck",
		"query": state.Query.Text,
	})

	// Delegate to infra/service
	result, err := s.cacheService.CheckCache(ctx, state.Query)
	if err != nil {
		s.logger.Error("cache check failed", err, map[string]interface{}{
			"step":  "SemanticCacheCheck",
			"query": state.Query.Text,
		})
		return fmt.Errorf("SemanticCacheCheck failed: %w", err)
	}

	// Update state using AgenticMetadata
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	if result.Hit {
		state.Agentic.CacheHit = &result.Hit
		state.Answer = result.Answer
		s.logger.Info("cache hit", map[string]interface{}{
			"step":  "SemanticCacheCheck",
			"query": state.Query.Text,
		})
	} else {
		state.Agentic.SetCacheHit(false)
		s.logger.Debug("cache miss", map[string]interface{}{
			"step":  "SemanticCacheCheck",
			"query": state.Query.Text,
		})
	}

	// Record metrics
	if s.metrics != nil {
		if result.Hit {
			s.metrics.RecordSearchResult("cache_hit", 1)
		} else {
			s.metrics.RecordSearchResult("cache_miss", 1)
		}
	}

	return nil
}

// store stores generated answers in semantic cache.
type store struct {
	cacheService *service.SemanticCacheService
	logger       logging.Logger
	metrics      abstraction.Metrics
}

// Store creates a new semantic cache storage step with logger and metrics.
//
// Parameters:
//   - cacheService: semantic cache service implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(cache.Store(cacheService, logger, metrics))
func Store(
	cacheService *service.SemanticCacheService,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &store{
		cacheService: cacheService,
		logger:       logger,
		metrics:      metrics,
	}
}

// Name returns the step name
func (s *store) Name() string {
	return "CacheResponseWriter"
}

// Execute caches the generated answer using infra/service.
func (s *store) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Answer == "" {
		return nil
	}

	s.logger.Debug("caching response", map[string]interface{}{
		"step":          "CacheResponseWriter",
		"query":         state.Query.Text,
		"answer_length": len(state.Answer),
	})

	// Delegate to infra/service
	err := s.cacheService.CacheResponse(ctx, state.Query, state.Answer)
	if err != nil {
		s.logger.Error("cache response failed", err, map[string]interface{}{
			"step":  "CacheResponseWriter",
			"query": state.Query.Text,
		})
		return fmt.Errorf("CacheResponseWriter failed: %w", err)
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("cache_store", 1)
	}

	s.logger.Info("response cached", map[string]interface{}{
		"step":          "CacheResponseWriter",
		"query":         state.Query.Text,
		"answer_length": len(state.Answer),
	})

	return nil
}
