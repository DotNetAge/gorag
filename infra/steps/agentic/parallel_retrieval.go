package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// parallelRetriever is a thin adapter that delegates to infra/service.
type parallelRetriever struct {
	retriever retrieval.Retriever
	topK      int
	logger    logging.Logger
}

// ParallelRetrieval creates a parallel retrieval step with logger.
//
// Parameters:
//   - retriever: retriever implementation
//   - topK: number of results to retrieve per query (default: 10)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//
// Example:
//
//	p.AddStep(agentic.ParallelRetrieval(retriever, 10, logger))
func ParallelRetrieval(retriever retrieval.Retriever, topK int, logger logging.Logger) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if topK <= 0 {
		topK = 10
	}
	return &parallelRetriever{
		retriever: retriever,
		topK:      topK,
		logger:    logger,
	}
}

// Name returns the step name
func (s *parallelRetriever) Name() string {
	return "ParallelRetrieval"
}

// Execute performs parallel retrieval using infra/service.
// This is a thin adapter (<30 lines).
func (s *parallelRetriever) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("parallelRetriever: query required")
	}

	s.logger.Debug("starting parallel retrieval", map[string]interface{}{
		"step":  "ParallelRetrieval",
		"query": state.Query.Text,
	})

	// Get sub-queries from AgenticMetadata (from DecompositionStep)
	var queries []string
	if state.Agentic != nil && len(state.Agentic.SubQueries) > 0 {
		queries = state.Agentic.SubQueries
		s.logger.Info("using decomposed queries", map[string]interface{}{
			"step":           "ParallelRetrieval",
			"sub_queries":    len(queries),
			"original_query": state.Query.Text,
		})
	} else {
		queries = []string{state.Query.Text}
	}

	// Delegate to infra/service (thick business logic: parallel retrieval)
	results, err := s.retriever.Retrieve(ctx, queries, s.topK)
	if err != nil {
		s.logger.Error("retrieve failed", err, map[string]interface{}{
			"step":  "ParallelRetrieval",
			"query": state.Query.Text,
		})
		return fmt.Errorf("parallelRetriever: Retrieve failed: %w", err)
	}

	// Update state (thin adapter 职责)
	for _, result := range results {
		state.RetrievedChunks = append(state.RetrievedChunks, result.Chunks)
	}

	s.logger.Info("retrieval completed", map[string]interface{}{
		"step":          "ParallelRetrieval",
		"results_count": len(results),
		"total_chunks":  len(state.RetrievedChunks),
		"query":         state.Query.Text,
	})

	return nil
}
