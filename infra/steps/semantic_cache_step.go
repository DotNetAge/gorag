package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/service"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*semanticCacheChecker)(nil)

// semanticCacheChecker is a thin adapter that checks cache using infra/service.
type semanticCacheChecker struct {
	cacheService *service.SemanticCacheService
	logger       logging.Logger
}

// NewSemanticCacheChecker creates a new semantic cache check step with logger.
func NewSemanticCacheChecker(cacheService *service.SemanticCacheService, logger logging.Logger) *semanticCacheChecker {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &semanticCacheChecker{
		cacheService: cacheService,
		logger:       logger,
	}
}

// Name returns the step name
func (s *semanticCacheChecker) Name() string {
	return "SemanticCacheChecker"
}

// Execute checks cache using infra/service.
// This is a thin adapter (<30 lines).
func (s *semanticCacheChecker) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("semanticCacheChecker: query required")
	}

	s.logger.Debug("checking semantic cache", map[string]interface{}{
		"step":  "SemanticCacheChecker",
		"query": state.Query.Text,
	})

	// Delegate to infra/service
	result, err := s.cacheService.CheckCache(ctx, state.Query)
	if err != nil {
		s.logger.Error("cache check failed", err, map[string]interface{}{
			"step":  "SemanticCacheChecker",
			"query": state.Query.Text,
		})
		return fmt.Errorf("semanticCacheChecker: CheckCache failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 职责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	if result.Hit {
		state.Agentic.CacheHit = &result.Hit
		state.Answer = result.Answer
		s.logger.Info("cache hit", map[string]interface{}{
			"step":  "SemanticCacheChecker",
			"query": state.Query.Text,
		})
	} else {
		state.Agentic.SetCacheHit(false)
		s.logger.Debug("cache miss", map[string]interface{}{
			"step":  "SemanticCacheChecker",
			"query": state.Query.Text,
		})
	}

	return nil
}

// cacheResponseWriter is a thin adapter that caches responses using infra/service.
type cacheResponseWriter struct {
	cacheService *service.SemanticCacheService
	logger       logging.Logger
}

// NewCacheResponseWriter creates a new cache response step with logger.
func NewCacheResponseWriter(cacheService *service.SemanticCacheService, logger logging.Logger) *cacheResponseWriter {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &cacheResponseWriter{
		cacheService: cacheService,
		logger:       logger,
	}
}

// Name returns the step name
func (s *cacheResponseWriter) Name() string {
	return "CacheResponseWriter"
}

// Execute caches the generated answer using infra/service.
// This is a thin adapter (<30 lines).
func (s *cacheResponseWriter) Execute(ctx context.Context, state *entity.PipelineState) error {
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
		return fmt.Errorf("cacheResponseWriter: CacheResponse failed: %w", err)
	}

	s.logger.Info("response cached", map[string]interface{}{
		"step":          "CacheResponseWriter",
		"query":         state.Query.Text,
		"answer_length": len(state.Answer),
	})

	return nil
}
