package prune

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// prune removes irrelevant chunks using result enhancer.
type prune struct {
	enhancer core.ResultEnhancer
	logger   logging.Logger
	metrics  core.Metrics
}

// Prune creates a new context pruning step.
func Prune(
	enhancer core.ResultEnhancer,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &prune{
		enhancer: enhancer,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *prune) Name() string {
	return "Prune"
}

// Execute enhances retrieval results by pruning.
func (s *prune) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil {
		return fmt.Errorf("Prune: 'query' not found in state")
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
		return fmt.Errorf("Prune: enhance failed: %w", err)
	}

	state.RetrievedChunks = [][]*core.Chunk{enhancedChunks}

	if s.metrics != nil {
		s.metrics.RecordSearchResult("pruning", len(enhancedChunks))
	}

	return nil
}
