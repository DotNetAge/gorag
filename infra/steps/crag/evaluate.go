// Package crag provides evaluation steps for RAG retrieval quality assessment.
package crag

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// evaluate evaluates the quality of retrieved context using CRAG.
type evaluate struct {
	evaluator retrieval.CRAGEvaluator
	logger    logging.Logger
	metrics   abstraction.Metrics
}

// Evaluate creates a new CRAG evaluator step with logger and metrics.
//
// Parameters:
//   - evaluator: CRAG evaluator implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(crag.Evaluate(evaluator, logger, metrics))
func Evaluate(
	evaluator retrieval.CRAGEvaluator,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &evaluate{
		evaluator: evaluator,
		logger:    logger,
		metrics:   metrics,
	}
}

// Name returns the step name
func (s *evaluate) Name() string {
	return "CRAGEvaluate"
}

// Execute runs the CRAG evaluation on retrieved chunks.
func (s *evaluate) Execute(ctx context.Context, state *entity.PipelineState) error {
	query := state.Query
	if query == nil || query.Text == "" {
		s.logger.Debug("CRAGEvaluate: no query available, skipping", map[string]interface{}{
			"step": "CRAGEvaluate",
		})
		return nil
	}

	// Get retrieved chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		// No chunks to evaluate, skip
		s.logger.Debug("CRAGEvaluate: no chunks to evaluate, skipping", map[string]interface{}{
			"step": "CRAGEvaluate",
		})
		return nil
	}

	// Evaluate retrieval quality
	evaluation, err := s.evaluator.Evaluate(ctx, query, allChunks)
	if err != nil {
		s.logger.Error("evaluation failed", err, map[string]interface{}{
			"step":        "CRAGEvaluate",
			"query":       query.Text,
			"chunk_count": len(allChunks),
		})
		return fmt.Errorf("CRAGEvaluatorStep.Evaluate: %w", err)
	}

	// Store evaluation result in Agentic state for downstream steps
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.Custom["crag_evaluation"] = evaluation

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("crag_evaluation", 1)
	}

	s.logger.Info("CRAGEvaluate completed", map[string]interface{}{
		"step":        "CRAGEvaluate",
		"query":       query.Text,
		"chunk_count": len(allChunks),
		"relevance":   evaluation.Relevance,
		"label":       evaluation.Label,
	})

	return nil
}
