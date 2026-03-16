package retrieval

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/graph"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*GraphLocalSearchStep)(nil)

// GraphLocalSearchStep is a pipeline step that performs local graph search
// by traversing N-Hop relationships from specific entities.
type GraphLocalSearchStep struct {
	searcher *graph.LocalSearcher
	maxHops  int
	topK     int
	logger   logging.Logger
}

// NewGraphLocalSearchStep creates a new graph local search step.
//
// Parameters:
// - searcher: The local searcher instance
// - maxHops: Maximum number of hops to traverse (default: 2)
// - topK: Maximum number of results to return (default: 10)
//
// Returns:
// - A new GraphLocalSearchStep instance
func NewGraphLocalSearchStep(searcher *graph.LocalSearcher, maxHops, topK int) *GraphLocalSearchStep {
	if maxHops <= 0 {
		maxHops = 2
	}
	if topK <= 0 {
		topK = 10
	}
	return &GraphLocalSearchStep{
		searcher: searcher,
		maxHops:  maxHops,
		topK:     topK,
		logger:   logging.NewNoopLogger(),
	}
}

// Name returns the step name
func (s *GraphLocalSearchStep) Name() string {
	return "GraphLocalSearchStep"
}

// Execute performs local graph search using entities from the query or previous extraction.
// It requires entity IDs to be present in query metadata.
func (s *GraphLocalSearchStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("GraphLocalSearchStep: 'query' not found in state")
	}

	// Get entity IDs from AgenticMetadata (strongly-typed, no blackboard pattern)
	var entityIDs []string

	if state.Agentic != nil && len(state.Agentic.EntityIDs) > 0 {
		entityIDs = state.Agentic.EntityIDs
	} else if queryText := state.Query.Text; queryText != "" {
		// If no entities, use query text as entity ID (simple case)
		entityIDs = []string{queryText}
	} else {
		return fmt.Errorf("GraphLocalSearchStep: no entity IDs available for graph search")
	}

	// Perform local search
	results, err := s.searcher.Search(ctx, entityIDs, s.maxHops, s.topK)
	if err != nil {
		return fmt.Errorf("GraphLocalSearchStep failed to search graph: %w", err)
	}

	// Store results as retrieved chunks
	chunk := &entity.Chunk{
		ID:      fmt.Sprintf("graph_local_%s", strings.Join(entityIDs, "_")),
		Content: results,
		Metadata: map[string]any{
			"source":     "graph_local_search",
			"entity_ids": entityIDs,
			"max_hops":   s.maxHops,
		},
	}

	state.RetrievedChunks = append(state.RetrievedChunks, []*entity.Chunk{chunk})

	s.logger.Info("GraphLocalSearchStep completed", map[string]interface{}{
		"step":         "GraphLocalSearchStep",
		"entity_count": len(entityIDs),
	})
	return nil
}
