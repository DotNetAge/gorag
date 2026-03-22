package cache

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type check struct {
	cache   core.SemanticCache
	logger  logging.Logger
	metrics core.Metrics
}

func Check(cache core.SemanticCache, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &check{cache: cache, logger: logger, metrics: metrics}
}

func (s *check) Name() string { return "SemanticCacheCheck" }

func (s *check) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil || state.Query.Text == "" {
		return nil
	}

	result, err := s.cache.CheckCache(ctx, state.Query)
	if err != nil {
		s.logger.Debug("cache check failed", map[string]any{"error": err.Error()})
		return err
	}

	if result.Hit {
		s.logger.Debug("cache hit", map[string]any{
			"query": state.Query.Text,
		})
		state.Answer = &core.Result{
			Answer: result.Answer,
			Score:  1.0,
		}
		if state.Agentic == nil {
			state.Agentic = core.NewAgenticState()
		}
		state.Agentic.CacheHit = true
	}

	if s.metrics != nil {
		s.metrics.RecordSearchResult("cache", 1)
	}

	return nil
}

type store struct {
	cache   core.SemanticCache
	logger  logging.Logger
	metrics core.Metrics
}

func Store(cache core.SemanticCache, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &store{cache: cache, logger: logger, metrics: metrics}
}

func (s *store) Name() string { return "SemanticCacheStore" }

func (s *store) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil || state.Query.Text == "" {
		return nil
	}

	if state.Answer == nil || state.Answer.Answer == "" {
		return nil
	}

	err := s.cache.CacheResponse(ctx, state.Query, state.Answer)
	if err != nil {
		s.logger.Debug("cache store failed", map[string]any{"error": err.Error()})
		return err
	}

	s.logger.Debug("response cached", map[string]any{
		"query": state.Query.Text,
	})

	if s.metrics != nil {
		s.metrics.RecordVectorStoreOperations("cache_set", 1)
	}

	return nil
}
