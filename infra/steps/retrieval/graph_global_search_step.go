package retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/graph"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*GraphGlobalSearchStep)(nil)

// GraphGlobalSearchStep is a pipeline step that performs global graph search
// by synthesizing community summaries for macro-level questions.
type GraphGlobalSearchStep struct {
	searcher       *graph.GlobalSearcher
	communityLevel int
	logger         logging.Logger
}

// NewGraphGlobalSearchStep creates a new graph global search step.
//
// Parameters:
// - searcher: The global searcher instance
// - communityLevel: The level of community summaries to use (default: 1)
//
// Returns:
// - A new GraphGlobalSearchStep instance
func NewGraphGlobalSearchStep(searcher *graph.GlobalSearcher, communityLevel int) *GraphGlobalSearchStep {
	if communityLevel <= 0 {
		communityLevel = 1
	}
	return &GraphGlobalSearchStep{
		searcher:       searcher,
		communityLevel: communityLevel,
		logger:         logging.NewNoopLogger(),
	}
}

// Name returns the step name
func (s *GraphGlobalSearchStep) Name() string {
	return "GraphGlobalSearchStep"
}

// Execute performs global graph search by synthesizing community summaries.
// It's suitable for answering broad, macro-level questions about the domain.
func (s *GraphGlobalSearchStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("GraphGlobalSearchStep: 'query' not found in state")
	}

	// Perform global search
	results, err := s.searcher.Search(ctx, state.Query.Text, s.communityLevel)
	if err != nil {
		return fmt.Errorf("GraphGlobalSearchStep failed to search graph: %w", err)
	}

	// Store results as retrieved chunks
	chunk := &entity.Chunk{
		ID:      fmt.Sprintf("graph_global_level_%d", s.communityLevel),
		Content: results,
		Metadata: map[string]any{
			"source":          "graph_global_search",
			"community_level": s.communityLevel,
		},
	}

	state.RetrievedChunks = append(state.RetrievedChunks, []*entity.Chunk{chunk})

	s.logger.Info("GraphGlobalSearchStep completed", map[string]interface{}{
		"step":            "GraphGlobalSearchStep",
		"community_level": s.communityLevel,
	})
	return nil
}
