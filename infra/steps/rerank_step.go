package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step = (*RerankStep)(nil)

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

func (s *RerankStep) Execute(ctx context.Context, state *pipeline.State) error {
	query, ok := state.Get("query").(*entity.Query)
	if !ok {
		return fmt.Errorf("RerankStep: 'query' (*entity.Query) not found in state")
	}

	chunks, ok := state.Get("retrieved_chunks").([]*entity.Chunk)
	if !ok || len(chunks) == 0 {
		// Nothing to rerank, just pass through
		return nil
	}

	rerankedChunks, scores, err := s.reranker.Rerank(ctx, query.Text, chunks, s.topK)
	if err != nil {
		return fmt.Errorf("RerankStep failed to rerank: %w", err)
	}

	// Update state with reranked chunks and their new scores
	state.Set("retrieved_chunks", rerankedChunks)
	state.Set("rerank_scores", scores)

	return nil
}
