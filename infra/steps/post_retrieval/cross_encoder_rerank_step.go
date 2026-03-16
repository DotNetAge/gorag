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
var _ pipeline.Step[*entity.PipelineState] = (*CrossEncoderRerankStep)(nil)

// CrossEncoderRerankStep uses LLM-based CrossEncoder to rerank retrieval results.
type CrossEncoderRerankStep struct {
	reranker retrieval.ResultEnhancer
	logger   logging.Logger
}

// NewCrossEncoderRerankStep creates a new cross-encoder reranking step.
func NewCrossEncoderRerankStep(reranker retrieval.ResultEnhancer, logger logging.Logger) *CrossEncoderRerankStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &CrossEncoderRerankStep{
		reranker: reranker,
		logger:   logger,
	}
}

// Name returns the step name
func (s *CrossEncoderRerankStep) Name() string {
	return "CrossEncoderRerankStep"
}

// Execute enhances retrieval results using cross-encoder reranking
func (s *CrossEncoderRerankStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("CrossEncoderRerankStep: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		s.logger.Debug("CrossEncoderRerankStep: no chunks to rerank", map[string]interface{}{
			"step": "CrossEncoderRerankStep",
		})
		return nil
	}

	// Flatten chunks from all retrieval sources
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
		"", // Empty document ID for pipeline context
		allChunks,
		make([]float32, len(allChunks)), // Initial scores
		nil,
	)

	// Apply cross-encoder reranking
	s.logger.Info("CrossEncoderRerankStep: starting reranking", map[string]interface{}{
		"step":        "CrossEncoderRerankStep",
		"chunk_count": len(allChunks),
	})

	enhancedResult, err := s.reranker.Enhance(ctx, result)
	if err != nil {
		return fmt.Errorf("CrossEncoderRerankStep: enhance failed: %w", err)
	}

	// Update state with reranked chunks
	state.RetrievedChunks = [][]*entity.Chunk{enhancedResult.Chunks}

	s.logger.Info("CrossEncoderRerankStep: completed reranking", map[string]interface{}{
		"step":           "CrossEncoderRerankStep",
		"original_count": len(allChunks),
		"reranked_count": len(enhancedResult.Chunks),
	})

	return nil
}
