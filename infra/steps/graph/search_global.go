// Package graph provides graph retrieval steps for RAG pipelines.
package graph

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/graph"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// global performs global graph search by synthesizing community summaries for macro-level questions.
type global struct {
	searcher       *graph.GlobalSearcher
	communityLevel int
	logger         logging.Logger
	metrics        abstraction.Metrics
}

// Global creates a new global graph search step with logger and metrics.
//
// Parameters:
//   - searcher: global graph searcher implementation
//   - communityLevel: community level for synthesis (default: 1)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(graph.Global(searcher, 2, logger, metrics))
func Global(
	searcher *graph.GlobalSearcher,
	communityLevel int,
	logger logging.Logger,
	metrics abstraction.Metrics,
) pipeline.Step[*entity.PipelineState] {
	if communityLevel <= 0 {
		communityLevel = 1
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &global{
		searcher:       searcher,
		communityLevel: communityLevel,
		logger:         logger,
		metrics:        metrics,
	}
}

// Name returns the step name
func (s *global) Name() string {
	return "GraphGlobalSearch"
}

// Execute performs global graph search by synthesizing community summaries.
func (s *global) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("GraphGlobalSearch: 'query' not found in state")
	}

	// Perform global search
	results, err := s.searcher.Search(ctx, state.Query.Text, s.communityLevel)
	if err != nil {
		s.logger.Error("failed to search graph", err, map[string]interface{}{
			"step":            "GraphGlobalSearch",
			"query":           state.Query.Text,
			"community_level": s.communityLevel,
		})
		return fmt.Errorf("GraphGlobalSearch failed to search graph: %w", err)
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

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("graph_global", 1)
	}

	s.logger.Info("GraphGlobalSearch completed", map[string]interface{}{
		"step":            "GraphGlobalSearch",
		"community_level": s.communityLevel,
	})

	return nil
}
