package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*RerankStep)(nil)

// RerankStep uses a Cross-Encoder to re-score and re-order chunks retrieved in previous steps.
type RerankStep struct {
	reranker abstraction.Reranker
	topK     int
}

// NewRerankStep creates a new reranking step.
func NewRerankStep(reranker abstraction.Reranker, topK int) *RerankStep {
	if topK <= 0 {
		topK = 5
	}
	return &RerankStep{
		reranker: reranker,
		topK:     topK,
	}
}

func (s *RerankStep) Name() string {
	return "RerankStep"
}

func (s *RerankStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("RerankStep: 'query' not found in state")
	}

	if len(state.RetrievedChunks) == 0 {
		// Nothing to rerank, just pass through
		return nil
	}

	// Flatten the retrieved chunks
	var allChunks []*entity.Chunk
	for _, chunks := range state.RetrievedChunks {
		allChunks = append(allChunks, chunks...)
	}

	if len(allChunks) == 0 {
		// Nothing to rerank, just pass through
		return nil
	}

	rerankedChunks, scores, err := s.reranker.Rerank(ctx, state.Query.Text, allChunks, s.topK)
	if err != nil {
		return fmt.Errorf("RerankStep failed to rerank: %w", err)
	}

	// Update state with reranked chunks and their new scores
	state.RetrievedChunks = [][]*entity.Chunk{rerankedChunks}
	state.RerankScores = scores

	return nil
}
