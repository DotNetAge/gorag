// Package fuse provides result fusion steps for RAG retrieval pipelines.
package fuse

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// rrf merges multiple chunk lists using Reciprocal Rank Fusion algorithm.
type rrf struct {
	fusionEngine retrieval.FusionEngine
	topK         int
	logger       logging.Logger
	metrics      abstraction.Metrics
}

// RRF creates a new RRF fusion step with logger and metrics.
//
// Parameters:
//   - engine: fusion engine implementation (default: built-in k=60)
//   - topK: number of results to keep after fusion (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(fuse.RRF(engine, 20, logger, metrics))
func RRF(
	engine retrieval.FusionEngine,
	topK int,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &rrf{
		fusionEngine: engine,
		topK:         topK,
		logger:       logger,
		metrics:      metrics,
	}
}

// Name returns the step name
func (s *rrf) Name() string {
	return "RAGFusion"
}

// Execute merges multiple chunk lists from state using the Reciprocal Rank Fusion algorithm.
func (s *rrf) Execute(ctx context.Context, state *entity.PipelineState) error {
	// Usually, a parallel step before this would populate state with a slice of Chunk arrays.
	// E.g., state.ParallelResults = [][]*entity.Chunk{ denseResults, sparseResults, graphResults }

	// If there are no results to fuse, skip
	if len(state.ParallelResults) == 0 {
		s.logger.Debug("RRF: no ParallelResults found, skipping", map[string]interface{}{
			"step": "RAGFusion",
		})
		return nil
	}

	// Use the fusion engine to merge results using Reciprocal Rank Fusion
	fusedChunks, err := s.fusionEngine.ReciprocalRankFusion(ctx, state.ParallelResults, s.topK)
	if err != nil {
		s.logger.Error("fusion failed", err, map[string]interface{}{
			"step":        "RAGFusion",
			"input_lists": len(state.ParallelResults),
			"topK":        s.topK,
		})
		return fmt.Errorf("RAGFusion failed: %w", err)
	}

	// Store the fused results back to the state
	state.RetrievedChunks = [][]*entity.Chunk{fusedChunks}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("fusion", len(fusedChunks))
	}

	s.logger.Info("RAGFusion completed", map[string]interface{}{
		"step":          "RAGFusion",
		"input_lists":   len(state.ParallelResults),
		"fused_results": len(fusedChunks),
		"topK":          s.topK,
	})

	return nil
}
