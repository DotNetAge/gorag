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
var _ pipeline.Step[*entity.PipelineState] = (*ContextPruningStep)(nil)

// ContextPruningStep uses LLM to prune irrelevant chunks and control token usage.
type ContextPruningStep struct {
	pruner retrieval.ResultEnhancer
	logger logging.Logger
}

// NewContextPruningStep creates a new context pruning step.
func NewContextPruningStep(pruner retrieval.ResultEnhancer, logger logging.Logger) *ContextPruningStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &ContextPruningStep{
		pruner: pruner,
		logger: logger,
	}
}

// Name returns the step name
func (s *ContextPruningStep) Name() string {
	return "ContextPruningStep"
}

// Execute enhances retrieval results by pruning irrelevant context
func (s *ContextPruningStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("ContextPruningStep: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("ContextPruningStep: no chunks to prune", map[string]interface{}{
			"step": "ContextPruningStep",
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

	s.logger.Info("ContextPruningStep: starting pruning", map[string]interface{}{
		"step":        "ContextPruningStep",
		"chunk_count": len(allChunks),
	})

	// Apply context pruning
	enhancedResult, err := s.pruner.Enhance(ctx, result)
	if err != nil {
		return fmt.Errorf("ContextPruningStep: enhance failed: %w", err)
	}

	// Update state with pruned chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	s.logger.Info("ContextPruningStep: completed pruning", map[string]interface{}{
		"step":           "ContextPruningStep",
		"original_count": len(allChunks),
		"pruned_count":   len(enhancedResult.Chunks),
	})

	return nil
}
