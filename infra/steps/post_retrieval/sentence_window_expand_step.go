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
var _ pipeline.Step[*entity.PipelineState] = (*SentenceWindowExpandStep)(nil)

// SentenceWindowExpandStep expands chunks with surrounding sentence context.
type SentenceWindowExpandStep struct {
	expander retrieval.ResultEnhancer
	logger   logging.Logger
}

// NewSentenceWindowExpandStep creates a new sentence window expansion step.
func NewSentenceWindowExpandStep(expander retrieval.ResultEnhancer, logger logging.Logger) *SentenceWindowExpandStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &SentenceWindowExpandStep{
		expander: expander,
		logger:   logger,
	}
}

// Name returns the step name
func (s *SentenceWindowExpandStep) Name() string {
	return "SentenceWindowExpandStep"
}

// Execute enhances retrieval results by expanding chunks with sentence window context
func (s *SentenceWindowExpandStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("SentenceWindowExpandStep: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("SentenceWindowExpandStep: no chunks to expand", map[string]interface{}{
			"step": "SentenceWindowExpandStep",
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

	s.logger.Info("SentenceWindowExpandStep: starting expansion", map[string]interface{}{
		"step":        "SentenceWindowExpandStep",
		"chunk_count": len(allChunks),
	})

	// Apply sentence window expansion
	enhancedResult, err := s.expander.Enhance(ctx, result)
	if err != nil {
		return fmt.Errorf("SentenceWindowExpandStep: enhance failed: %w", err)
	}

	// Update state with expanded chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	s.logger.Info("SentenceWindowExpandStep: completed expansion", map[string]interface{}{
		"step":           "SentenceWindowExpandStep",
		"original_count": len(allChunks),
		"expanded_count": len(enhancedResult.Chunks),
	})

	return nil
}
