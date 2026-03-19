// Package rerank provides reranking steps for RAG retrieval pipelines.
package rerank

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// score re-scores and re-orders chunks using a Cross-Encoder reranker.
type score struct {
	reranker core.Reranker
	topK     int
	logger   logging.Logger
	metrics  core.Metrics
}

// Score creates a new reranking step with logger and metrics.
//
// Parameters:
//   - reranker: cross-encoder reranker implementation
//   - topK: number of results to keep after reranking (default: 5)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(rerank.Score(reranker, 10, logger, metrics))
func Score(
	reranker core.Reranker,
	topK int,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if topK <= 0 {
		topK = 5
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &score{
		reranker: reranker,
		topK:     topK,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *score) Name() string {
	return "Rerank"
}

// Execute re-scores and re-orders chunks retrieved in previous steps.
func (s *score) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil {
		return fmt.Errorf("Rerank: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("Rerank: no chunks to rerank", map[string]interface{}{
			"step": "Rerank",
		})
		return nil
	}

	// Flatten the retrieved chunks
	var allChunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		allChunks = append(allChunks, chunkGroup...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	s.logger.Info("Rerank: starting reranking", map[string]interface{}{
		"step":        "Rerank",
		"chunk_count": len(allChunks),
		"topK":        s.topK,
	})

	// Rerank chunks
	rerankedChunks, err := s.reranker.Rerank(ctx, state.Query, allChunks)
	if err != nil {
		s.logger.Error("reranking failed", err, map[string]interface{}{
			"step":        "Rerank",
			"chunk_count": len(allChunks),
			"topK":        s.topK,
		})
		return fmt.Errorf("Rerank failed to rerank: %w", err)
	}

	// Update state with reranked chunks and their new scores
	state.RetrievedChunks = [][]*core.Chunk{rerankedChunks}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("rerank", len(rerankedChunks))
	}

	s.logger.Info("Rerank: completed reranking", map[string]interface{}{
		"step":           "Rerank",
		"original_count": len(allChunks),
		"reranked_count": len(rerankedChunks),
		"topK":           s.topK,
	})

	return nil
}
