package steps

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

// CRAGEvaluation represents the result of CRAG evaluation
type CRAGEvaluation struct {
	// Relevance is the relevance score (0.0-1.0)
	Relevance float32 `json:"relevance"`
	// Label is the categorical evaluation
	Label CRAGLabel `json:"label"`
	// Reason explains why this evaluation was given
	Reason string `json:"reason"`
}

// CRAGLabel represents the categorical evaluation of retrieved context
type CRAGLabel string

const (
	// CRAGRelevant means the context is highly relevant and sufficient
	CRAGRelevant CRAGLabel = "relevant"
	// CRAGIrrelevant means the context is not relevant, need fallback
	CRAGIrrelevant CRAGLabel = "irrelevant"
	// CRAGAmbiguous means the context is partially relevant or ambiguous, need refinement
	CRAGAmbiguous CRAGLabel = "ambiguous"
)

// GetCRAGEvaluation retrieves the CRAG evaluation from state
func GetCRAGEvaluation(state *entity.PipelineState) CRAGEvaluation {
	labelStr, labelOk := state.Query.Metadata["crag_label"].(string)
	relevance, _ := state.Query.Metadata["crag_relevance"].(float32)
	reason, _ := state.Query.Metadata["crag_reason"].(string)

	if !labelOk {
		return CRAGEvaluation{
			Relevance: 0.0,
			Label:     CRAGIrrelevant,
			Reason:    "No evaluation performed",
		}
	}

	return CRAGEvaluation{
		Relevance: relevance,
		Label:     CRAGLabel(labelStr),
		Reason:    reason,
	}
}

// NeedsRefinement checks if the context needs query refinement
func NeedsRefinement(state *entity.PipelineState) bool {
	needsRefine, ok := state.Query.Metadata["needs_refinement"].(bool)
	return ok && needsRefine
}

// NeedsFallback checks if the context needs fallback to external search
func NeedsFallback(state *entity.PipelineState) bool {
	needsFallback, ok := state.Query.Metadata["needs_fallback"].(bool)
	return ok && needsFallback
}
