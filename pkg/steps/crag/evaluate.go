// Package crag provides evaluation steps for RAG retrieval quality assessment.
package crag

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// evaluate evaluates the quality of retrieved context using CRAG.
type evaluate struct {
	evaluator core.CRAGEvaluator
	logger    logging.Logger
	metrics   core.Metrics
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
	evaluator core.CRAGEvaluator,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
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
func (s *evaluate) Execute(ctx context.Context, state *core.RetrievalContext) error {
	query := state.Query
	if query == nil || query.Text == "" {
		s.logger.Debug("CRAGEvaluate: no query available, skipping", map[string]interface{}{
			"step": "CRAGEvaluate",
		})
		return nil
	}

	// Get retrieved chunks
	var allChunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		allChunks = append(allChunks, chunkGroup...)
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
		state.Agentic = core.NewAgenticState()
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
	})

	return nil
}
