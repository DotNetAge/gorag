package post_retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*cragEvaluator)(nil)

// cragEvaluator is a thin adapter that delegates to infra/service.
type cragEvaluator struct {
	evaluator retrieval.CRAGEvaluator
	logger    logging.Logger
}

// NewCRAGEvaluator creates a new CRAG evaluator step with logger.
func NewCRAGEvaluator(evaluator retrieval.CRAGEvaluator, logger logging.Logger) *cragEvaluator {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &cragEvaluator{
		evaluator: evaluator,
		logger:    logger,
	}
}

// Name returns the step name
func (s *cragEvaluator) Name() string {
	return "CRAGEvaluator"
}

// Execute evaluates retrieved chunks using infra/service.
// This is a thin adapter (<30 lines).
func (s *cragEvaluator) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || len(state.RetrievedChunks) == 0 {
		return fmt.Errorf("cragEvaluator: query or retrieval results required")
	}

	s.logger.Debug("evaluating retrieved chunks", map[string]interface{}{
		"step":         "CRAGEvaluator",
		"chunks_count": len(state.RetrievedChunks),
	})

	// Flatten RetrievedChunks to []*entity.Chunk
	var chunks []*entity.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		chunks = append(chunks, chunkGroup...)
	}

	// Delegate to infra/service (thick business logic)
	result, err := s.evaluator.Evaluate(ctx, state.Query, chunks)
	if err != nil {
		s.logger.Error("evaluate failed", err, map[string]interface{}{
			"step":  "CRAGEvaluator",
			"query": state.Query.Text,
		})
		return fmt.Errorf("cragEvaluator: Evaluate failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 职责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.CRAGEvaluation = string(result.Label)

	s.logger.Info("CRAG evaluation completed", map[string]interface{}{
		"step":      "CRAGEvaluator",
		"label":     result.Label,
		"relevance": result.Relevance,
		"query":     state.Query.Text,
	})

	return nil
}

// GetCRAGEvaluation retrieves the CRAG evaluation label from state.
// It reads from state.Agentic.CRAGEvaluation (strongly-typed field).
func GetCRAGEvaluation(state *entity.PipelineState) retrieval.CRAGEvaluation {
	if state.Agentic != nil && state.Agentic.CRAGEvaluation != "" {
		return retrieval.CRAGEvaluation{
			Label: retrieval.CRAGLabel(state.Agentic.CRAGEvaluation),
		}
	}

	// No evaluation performed
	return retrieval.CRAGEvaluation{
		Relevance: 0.0,
		Label:     retrieval.CRAGIrrelevant,
		Reason:    "No evaluation performed",
	}
}

// NeedsRefinement checks if the context needs query refinement (CRAG ambiguous).
func NeedsRefinement(state *entity.PipelineState) bool {
	if state.Agentic != nil {
		return state.Agentic.CRAGEvaluation == string(retrieval.CRAGAmbiguous)
	}
	return false
}

// NeedsFallback checks if the context needs fallback to external search (CRAG irrelevant).
func NeedsFallback(state *entity.PipelineState) bool {
	if state.Agentic != nil {
		return state.Agentic.CRAGEvaluation == string(retrieval.CRAGIrrelevant)
	}
	return false
}
