package service

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"sync"
	"time"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.Retriever = (*retriever)(nil)

// retriever implements core.Retriever interface.
type retriever struct {
	vectorStore core.VectorStore
	embedder    core.Embedder
	defaultTopK int
	logger      logging.Logger
	collector   observability.Collector
}

// Option configures a retriever instance.
type Option func(*retriever)

// WithTopK sets the default top-K.
func WithTopK(k int) Option {
	return func(r *retriever) {
		if k > 0 {
			r.defaultTopK = k
		}
	}
}

// WithLogger sets a structured logger.
func WithLogger(l logging.Logger) Option {
	return func(r *retriever) {
		if l != nil {
			r.logger = l
		}
	}
}

// WithCollector sets an observability collector.
func WithCollector(c observability.Collector) Option {
	return func(r *retriever) {
		if c != nil {
			r.collector = c
		}
	}
}

// New creates a new retriever.
func New(vectorStore core.VectorStore, embedder core.Embedder, opts ...Option) core.Retriever {
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

// retrieveResult internal helper struct
type retrieveResult struct {
	query  string
	chunks []*core.Chunk
	scores []float32
	err    error
}

// Retrieve performs parallel retrieval for multiple queries.
func (r *retriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	start := time.Now()
	defer func() {
		r.collector.RecordDuration("retrieval", time.Since(start), nil)
	}()

	if len(queries) == 0 {
		return nil, fmt.Errorf("retriever: queries required")
	}

	if topK <= 0 {
		topK = r.defaultTopK
	}

	r.logger.Debug("starting retrieval", map[string]interface{}{
		"query_count": len(queries),
		"top_k":       topK,
	})

	if len(queries) == 1 {
		result, err := r.retrieveSingle(ctx, queries[0], topK)
		if err != nil {
			r.collector.RecordCount("retrieval", "error", nil)
			return nil, err
		}
		r.collector.RecordCount("retrieval", "success", nil)
		return []*core.RetrievalResult{result}, nil
	}

	resultChan := make(chan retrieveResult, len(queries))
	var wg sync.WaitGroup

	for _, query := range queries {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			result, err := r.retrieveSingle(ctx, q, topK)
			if err != nil {
				resultChan <- retrieveResult{query: q, err: err}
				return
			}
			resultChan <- retrieveResult{
				query:  q,
				chunks: result.Chunks,
				scores: result.Scores,
			}
		}(query)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var results []*core.RetrievalResult
	for res := range resultChan {
		if res.err != nil {
			r.logger.Warn("retrieval error for query", map[string]interface{}{
				"query": res.query,
				"error": res.err,
			})
			continue
		}
		results = append(results, &core.RetrievalResult{
			Chunks: res.chunks,
			Scores: res.scores,
		})
	}

	r.collector.RecordCount("retrieval", "success", nil)
	return results, nil
}

// retrieveSingle performs retrieval for a single query.
func (r *retriever) retrieveSingle(ctx context.Context, query string, topK int) (*core.RetrievalResult, error) {
	embeddings, err := r.embedder.EmbedBatch(ctx, []string{query})
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

	chunks := make([]*core.Chunk, len(vectors))
	for i, vector := range vectors {
		chunks[i] = &core.Chunk{
			ID:       vector.ID,
			Metadata: vector.Metadata,
		}
	}

	return &core.RetrievalResult{
		Chunks: chunks,
		Scores: scores,
	}, nil
}
