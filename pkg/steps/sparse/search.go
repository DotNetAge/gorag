// Package sparse provides sparse retrieval steps using BM25 algorithm.
package sparse

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// Searcher defines the interface for sparse search operations.
type Searcher interface {
	Search(ctx context.Context, query string, topK int) ([]*Result, error)
}

// Result represents a sparse search result.
type Result struct {
	Chunk *core.Chunk
	Score float64
}

// search retrieves relevant chunks using BM25 sparse core.
type search struct {
	searcher Searcher
	topK     int
	logger   logging.Logger
	metrics  core.Metrics
}

// Search creates a new BM25 sparse search step with logger and metrics.
//
// Parameters:
//   - searcher: sparse search implementation (BM25, etc.)
//   - topK: number of results to retrieve (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(sparse.Search(searcher, 20, logger, metrics))
func Search(
	searcher Searcher,
	topK int,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.RetrievalContext] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &search{
		searcher: searcher,
		topK:     topK,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *search) Name() string {
	return "SparseSearch"
}

// Execute retrieves relevant chunks using BM25 algorithm.
func (s *search) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil {
		return fmt.Errorf("SparseSearch: 'query' not found in state")
	}

	// Retrieve using sparse searcher
	results, err := s.searcher.Search(ctx, state.Query.Text, s.topK)
	if err != nil {
		return fmt.Errorf("SparseSearch failed to retrieve: %w", err)
	}

	// Convert results to chunks
	var chunks []*core.Chunk
	for _, r := range results {
		chunks = append(chunks, r.Chunk)
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("sparse", len(chunks))
	}

	s.logger.Debug("sparse search completed", map[string]interface{}{
		"step":    "SparseSearch",
		"query":   state.Query.Text,
		"results": len(chunks),
		"topK":    s.topK,
	})

	// Store retrieved chunks in state
	if state.RetrievedChunks == nil {
		state.RetrievedChunks = make([][]*core.Chunk, 0)
	}
	state.RetrievedChunks = append(state.RetrievedChunks, chunks)

	return nil
}
