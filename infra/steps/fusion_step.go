package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step = (*RAGFusionStep)(nil)

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

func (s *RAGFusionStep) Execute(ctx context.Context, state *pipeline.State) error {
	// Usually, a parallel step before this would populate state with a slice of Chunk arrays.
	// E.g., state.Set("parallel_results", [][]*entity.Chunk{ denseResults, sparseResults, graphResults })
	
	// For this demo, let's assume "parallel_results" exists in state.
	// If it doesn't, we just skip fusion.
	
	// Type assertion is tricky with slice of slices in any, doing it safely:
	rawResults := state.Get("parallel_results")
	if rawResults == nil {
		return nil // Nothing to fuse
	}

	// This assumes the parallel step actually put the correct type.
	// You might need a more robust cast depending on how parallel steps are implemented.
	// For compilation sake in this generic step:
	// ... (Implementation detail depends on parallel step state layout)
	
	return fmt.Errorf("RAGFusionStep requires a specific state contract from ParallelSearch (not fully defined here)")
}
