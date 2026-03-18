// Package prune provides context pruning steps for RAG retrieval pipelines.
package prune

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// prune removes irrelevant chunks using LLM-based result enhancer.
type prune struct {
	enhancer retrieval.ResultEnhancer
	logger   logging.Logger
	metrics  abstraction.Metrics
}

// Prune creates a new context pruning step with logger and metrics.
//
// Parameters:
//   - enhancer: LLM-based result enhancer implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(prune.Prune(enhancer, logger, metrics))
func Prune(
	enhancer retrieval.ResultEnhancer,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
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
	return "ContextPruning"
}

// Execute enhances retrieval results by pruning irrelevant context.
func (s *prune) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("ContextPruning: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("ContextPruning: no chunks to prune", map[string]interface{}{
			"step": "ContextPruning",
		})
		return nil
	}

	// Flatten chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		return nil
	}

	// Create retrieval result for enhancement
	result := entity.NewRetrievalResult(
		state.Query.ID,
		"",
		allChunks,
		make([]float32, len(allChunks)),
		nil,
	)

	s.logger.Info("ContextPruning: starting pruning", map[string]interface{}{
		"step":        "ContextPruning",
		"chunk_count": len(allChunks),
	})

	// Apply context pruning
	enhancedResult, err := s.enhancer.Enhance(ctx, result)
	if err != nil {
		s.logger.Error("context pruning failed", err, map[string]interface{}{
			"step":        "ContextPruning",
			"chunk_count": len(allChunks),
		})
		return fmt.Errorf("ContextPruning: enhance failed: %w", err)
	}

	// Update state with pruned chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("pruning", len(enhancedResult.Chunks))
	}

	s.logger.Info("ContextPruning: completed pruning", map[string]interface{}{
		"step":           "ContextPruning",
		"original_count": len(allChunks),
		"pruned_count":   len(enhancedResult.Chunks),
	})

	return nil
}
