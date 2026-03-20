package rerank

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// order re-orders chunks using result enhancer.
type order struct {
	enhancer core.ResultEnhancer
	logger   logging.Logger
	metrics  core.Metrics
}

// Order creates a new reranking step.
func Order(
	enhancer core.ResultEnhancer,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &order{
		enhancer: enhancer,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *order) Name() string {
	return "Rerank"
}

// Execute enhances retrieval results.
func (s *order) Execute(ctx context.Context, state *core.RetrievalContext) error {
	if state.Query == nil {
		return fmt.Errorf("Rerank: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		return nil
	}

	// Flatten chunks from RetrievedChunks
	var allChunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		allChunks = append(allChunks, chunkGroup...)
	}

	enhancedChunks, err := s.enhancer.Enhance(ctx, state.Query, allChunks)
	if err != nil {
		return fmt.Errorf("Rerank: enhance failed: %w", err)
	}

	state.RetrievedChunks = [][]*core.Chunk{enhancedChunks}

	if s.metrics != nil {
		s.metrics.RecordSearchResult("rerank", len(enhancedChunks))
	}

	return nil
}
