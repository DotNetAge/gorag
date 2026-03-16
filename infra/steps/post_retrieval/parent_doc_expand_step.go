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
var _ pipeline.Step[*entity.PipelineState] = (*ParentDocExpandStep)(nil)

// ParentDocExpandStep expands child chunks to their parent documents.
type ParentDocExpandStep struct {
	expander retrieval.ResultEnhancer
	logger   logging.Logger
}

// NewParentDocExpandStep creates a new parent document expansion step.
func NewParentDocExpandStep(expander retrieval.ResultEnhancer, logger logging.Logger) *ParentDocExpandStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &ParentDocExpandStep{
		expander: expander,
		logger:   logger,
	}
}

// Name returns the step name
func (s *ParentDocExpandStep) Name() string {
	return "ParentDocExpandStep"
}

// Execute enhances retrieval results by expanding child chunks to parent documents
func (s *ParentDocExpandStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("ParentDocExpandStep: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("ParentDocExpandStep: no chunks to expand", map[string]interface{}{
			"step": "ParentDocExpandStep",
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

	s.logger.Info("ParentDocExpandStep: starting expansion", map[string]interface{}{
		"step":        "ParentDocExpandStep",
		"chunk_count": len(allChunks),
	})

	// Apply parent document expansion
	enhancedResult, err := s.expander.Enhance(ctx, result)
	if err != nil {
		return fmt.Errorf("ParentDocExpandStep: enhance failed: %w", err)
	}

	// Update state with expanded chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	s.logger.Info("ParentDocExpandStep: completed expansion", map[string]interface{}{
		"step":           "ParentDocExpandStep",
		"original_count": len(allChunks),
		"expanded_count": len(enhancedResult.Chunks),
	})

	return nil
}
