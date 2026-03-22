package cache

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type retrieverWithCache struct {
	retriever core.Retriever
	cache     core.SemanticCache
	logger    logging.Logger
}

func NewRetrieverWithCache(retriever core.Retriever, semanticCache core.SemanticCache, logger logging.Logger) core.Retriever {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &retrieverWithCache{
		retriever: retriever,
		cache:     semanticCache,
		logger:    logger,
	}
}

func (r *retrieverWithCache) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	results := make([]*core.RetrievalResult, 0, len(queries))

	for _, q := range queries {
		query := core.NewQuery("", q, nil)

		result, err := r.cache.CheckCache(ctx, query)
		if err != nil {
			r.logger.Debug("cache check failed", map[string]any{"error": err.Error()})
		}

		if result != nil && result.Hit {
			r.logger.Debug("cache hit, skipping retrieval", map[string]any{"query": q})
			results = append(results, &core.RetrievalResult{
				Query:  q,
				Chunks: nil,
				Answer: result.Answer,
				Metadata: map[string]any{
					"cache_hit": true,
				},
			})
			continue
		}

		retResults, err := r.retriever.Retrieve(ctx, []string{q}, topK)
		if err != nil {
			return nil, err
		}

		if len(retResults) > 0 {
			res := retResults[0]
			if res.Answer != "" {
				answer := &core.Result{Answer: res.Answer}
				if err := r.cache.CacheResponse(ctx, query, answer); err != nil {
					r.logger.Debug("cache store failed", map[string]any{"error": err.Error()})
				}
			}
			results = append(results, res)
		}
	}

	return results, nil
}
