// Package graph provides graph retrieval steps for RAG pipelines.
package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/graph"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// local performs local graph search by traversing N-Hop relationships from specific entities.
type local struct {
	searcher *graph.LocalSearcher
	maxHops  int
	topK     int
	logger   logging.Logger
	metrics  abstraction.Metrics
}

// Local creates a new local graph search step with logger and metrics.
//
// Parameters:
//   - searcher: local graph searcher implementation
//   - maxHops: maximum hops for traversal (default: 2)
//   - topK: number of results to retrieve (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(graph.Local(searcher, 3, 20, logger, metrics))
func Local(
	searcher *graph.LocalSearcher,
	maxHops int,
	topK int,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if maxHops <= 0 {
		maxHops = 2
	}
	if topK <= 0 {
		topK = 10
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &local{
		searcher: searcher,
		maxHops:  maxHops,
		topK:     topK,
		logger:   logger,
		metrics:  metrics,
	}
}

// Name returns the step name
func (s *local) Name() string {
	return "GraphLocalSearch"
}

// Execute performs local graph search using entities from the query or previous extraction.
func (s *local) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("GraphLocalSearch: 'query' not found in state")
	}

	// Get entity IDs from AgenticMetadata
	var entityIDs []string
	if state.Agentic != nil && len(state.Agentic.EntityIDs) > 0 {
		entityIDs = state.Agentic.EntityIDs
	} else if queryText := state.Query.Text; queryText != "" {
		// If no entities, use query text as entity ID
		entityIDs = []string{queryText}
	} else {
		return fmt.Errorf("GraphLocalSearch: no entity IDs available for graph search")
	}

	// Perform local search
	results, err := s.searcher.Search(ctx, entityIDs, s.maxHops, s.topK)
	if err != nil {
		s.logger.Error("failed to search graph", err, map[string]interface{}{
			"step":       "GraphLocalSearch",
			"entity_ids": entityIDs,
			"max_hops":   s.maxHops,
		})
		return fmt.Errorf("GraphLocalSearch failed to search graph: %w", err)
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

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("graph_local", 1)
	}

	s.logger.Info("GraphLocalSearch completed", map[string]interface{}{
		"step":         "GraphLocalSearch",
		"entity_count": len(entityIDs),
		"max_hops":     s.maxHops,
	})

	return nil
}
