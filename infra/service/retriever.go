package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.Retriever = (*retriever)(nil)

// retriever is the infrastructure implementation of retrieval.Retriever.
type retriever struct {
	vectorStore abstraction.VectorStore
	embedder    embedding.Provider
	defaultTopK int
	logger      logging.Logger
	collector   observability.Collector
}

// Option configures a retriever instance.
type RetrieverOption func(*retriever)

// WithTopK sets the default top-K when the caller passes topK <= 0.
func WithTopK(k int) RetrieverOption {
	return func(r *retriever) {
		if k > 0 {
			r.defaultTopK = k
		}
	}
}

// WithRetrieverLogger sets a structured logger.
func WithRetrieverLogger(logger logging.Logger) RetrieverOption {
	return func(r *retriever) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// WithRetrieverCollector sets an observability collector.
func WithRetrieverCollector(collector observability.Collector) RetrieverOption {
	return func(r *retriever) {
		if collector != nil {
			r.collector = collector
		}
	}
}

// NewRetriever creates a new retriever.
//
// Required: vectorStore, embedder.
// Optional (via options): WithTopK (default 5), WithRetrieverLogger, WithRetrieverCollector.
func NewRetriever(vectorStore abstraction.VectorStore, embedder embedding.Provider, opts ...RetrieverOption) *retriever {
	r := &retriever{
		vectorStore: vectorStore,
		embedder:    embedder,
		defaultTopK: 5,
		logger:      logging.NewNoopLogger(),
		collector:   observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// retrievalResult holds internal retrieval result.
type retrievalResult struct {
	query  string
	chunks []*entity.Chunk
	scores []float32
	err    error
}

// Retrieve performs parallel retrieval for multiple queries.
func (r *retriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*retrieval.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		r.collector.RecordDuration("retrieval", time.Since(start), nil)
	}()

	if len(queries) == 0 {
		r.logger.Error("retrieve failed", fmt.Errorf("queries required"), map[string]interface{}{
			"operation": "retrieval",
		})
		r.collector.RecordCount("retrieval", "error", nil)
		return nil, fmt.Errorf("retriever: queries required")
	}

	if topK <= 0 {
		topK = r.defaultTopK
	}

	r.logger.Debug("starting retrieval", map[string]interface{}{
		"query_count": len(queries),
		"top_k":       topK,
	})

	// Single query optimization
	if len(queries) == 1 {
		result, err := r.retrieveSingle(ctx, queries[0], topK)
		if err != nil {
			r.collector.RecordCount("retrieval", "error", nil)
			return nil, err
		}
		r.logger.Info("retrieval completed", map[string]interface{}{
			"results_count": 1,
			"query":         queries[0],
		})
		r.collector.RecordCount("retrieval", "success", nil)
		return []*retrieval.RetrievalResult{result}, nil
	}

	// Parallel retrieval for multiple queries
	r.logger.Info("starting parallel retrieval", map[string]interface{}{
		"query_count": len(queries),
	})

	resultChan := make(chan retrievalResult, len(queries))
	var wg sync.WaitGroup

	// Launch goroutines for parallel retrieval
	for _, query := range queries {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			result, err := r.retrieveSingle(ctx, q, topK)
			resultChan <- retrievalResult{
				query:  q,
				chunks: result.Chunks,
				scores: result.Scores,
				err:    err,
			}
		}(query)
	}

	// Wait and close channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []*retrieval.RetrievalResult
	successCount := 0
	errorCount := 0
	for res := range resultChan {
		if res.err != nil {
			r.logger.Warn("retrieval error for query", map[string]interface{}{
				"query": res.query,
				"error": res.err,
			})
			errorCount++
			continue // Skip failing queries
		}
		results = append(results, &retrieval.RetrievalResult{
			Chunks: res.chunks,
			Scores: res.scores,
		})
		successCount++
	}

	r.logger.Info("retrieval completed", map[string]interface{}{
		"total_queries":    len(queries),
		"successful":       successCount,
		"failed":           errorCount,
		"results_returned": len(results),
	})
	r.collector.RecordCount("retrieval", "success", nil)

	return results, nil
}

// retrieveSingle performs retrieval for a single query.
func (r *retriever) retrieveSingle(ctx context.Context, query string, topK int) (*retrieval.RetrievalResult, error) {
	embeddings, err := r.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("retriever: embed failed: %w", err)
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("retriever: embed returned empty result")
	}

	vectors, scores, err := r.vectorStore.Search(ctx, embeddings[0], topK, nil)
	if err != nil {
		return nil, fmt.Errorf("retriever: Search failed: %w", err)
	}

	// Convert Vector to Chunk
	// Note: Vector only contains ID and Metadata, content comes from document store
	chunks := make([]*entity.Chunk, len(vectors))
	for i, vector := range vectors {
		chunks[i] = &entity.Chunk{
			ID:       vector.ID,
			Metadata: vector.Metadata,
		}
	}

	return &retrieval.RetrievalResult{
		Chunks: chunks,
		Scores: scores,
	}, nil
}
