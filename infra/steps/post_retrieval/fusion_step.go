package post_retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*RAGFusionStep)(nil)

// RAGFusionStep merges multiple chunk lists from state (e.g., from parallel searches)
// using the Reciprocal Rank Fusion algorithm.
type RAGFusionStep struct {
	fusionEngine retrieval.FusionEngine
	topK         int
}

// NewRAGFusionStep creates a fusion step.
func NewRAGFusionStep(engine retrieval.FusionEngine, topK int) *RAGFusionStep {
	if topK <= 0 {
		topK = 10
	}
	return &RAGFusionStep{
		fusionEngine: engine,
		topK:         topK,
	}
}

func (s *RAGFusionStep) Name() string {
	return "RAGFusionStep"
}

func (s *RAGFusionStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	// Usually, a parallel step before this would populate state with a slice of Chunk arrays.
	// E.g., state.ParallelResults = [][]*entity.Chunk{ denseResults, sparseResults, graphResults }

	// If there are no results to fuse, skip
	if len(state.ParallelResults) == 0 {
		return nil
	}

	// Use the fusion engine to merge results using Reciprocal Rank Fusion
	fusedChunks, err := s.fusionEngine.ReciprocalRankFusion(ctx, state.ParallelResults, s.topK)
	if err != nil {
		return fmt.Errorf("fusion failed: %w", err)
	}

	// Store the fused results back to the state
	state.RetrievedChunks = [][]*entity.Chunk{fusedChunks}

	return nil
}
