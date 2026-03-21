package cache

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// SemanticCache is an interface for cache operations.
type SemanticCache interface {
	CheckCache(ctx context.Context, query *core.Query) (*CacheResult, error)
	CacheResponse(ctx context.Context, query *core.Query, answer *core.Result) error
}

type CacheResult struct {
	Hit    bool
	Answer string
}

type check struct {
	cache   SemanticCache
	logger  logging.Logger
	metrics core.Metrics
}

func Check(cache SemanticCache, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
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
		return nil
	}

	if result.Hit {
		state.Agentic.CacheHit = true
		state.Answer = &core.Result{Answer: result.Answer}
	} else {
		state.Agentic.CacheHit = false
	}

	return nil
}

type store struct {
	cache   SemanticCache
	logger  logging.Logger
	metrics core.Metrics
}

func Store(cache SemanticCache, logger logging.Logger, metrics core.Metrics) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &store{cache: cache, logger: logger, metrics: metrics}
}

func (s *store) Name() string { return "SemanticCacheStore" }

func (s *store) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil || state.Answer == nil || state.Answer.Answer == "" {
		return nil
	}
	return s.cache.CacheResponse(ctx, state.Query, state.Answer)
}
