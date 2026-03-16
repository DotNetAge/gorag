package retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*CRAGEvaluatorStep)(nil)

// CRAGEvaluatorStep evaluates the quality of retrieved context using CRAG.
type CRAGEvaluatorStep struct {
	evaluator retrieval.CRAGEvaluator // CRAG evaluator
}

// NewCRAGEvaluatorStep creates a new CRAG evaluator step.
func NewCRAGEvaluatorStep(evaluator retrieval.CRAGEvaluator) *CRAGEvaluatorStep {
	return &CRAGEvaluatorStep{
		evaluator: evaluator,
	}
}

// Name returns the name of this step.
func (s *CRAGEvaluatorStep) Name() string {
	return "CRAGEvaluatorStep"
}

// Execute runs the CRAG evaluation on retrieved chunks.
func (s *CRAGEvaluatorStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	query := state.Query
	if query == nil || query.Text == "" {
		return fmt.Errorf("CRAGEvaluatorStep: no query available")
	}

	// Get retrieved chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		// No chunks to evaluate, skip
		return nil
	}

	// Evaluate retrieval quality
	evaluation, err := s.evaluator.Evaluate(ctx, query, allChunks)
	if err != nil {
		return fmt.Errorf("CRAGEvaluatorStep.Evaluate: %w", err)
	}

	// Store evaluation result in Agentic state for downstream steps
	if state.Agentic == nil {
		state.Agentic = &entity.AgenticMetadata{Custom: make(map[string]interface{})}
	}
	state.Agentic.Custom["crag_evaluation"] = evaluation

	return nil
}
