package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*parallelRetriever)(nil)

// parallelRetriever is a thin adapter that delegates to infra/service.
type parallelRetriever struct {
	retriever retrieval.Retriever
	topK      int
	logger    logging.Logger
}

// NewParallelRetriever creates a new parallel retrieval step with logger.
func NewParallelRetriever(retriever retrieval.Retriever, topK int, logger logging.Logger) *parallelRetriever {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &parallelRetriever{
		retriever: retriever,
		topK:      topK,
		logger:    logger,
	}
}

// Name returns the step name
func (s *parallelRetriever) Name() string {
	return "ParallelRetriever"
}

// Execute performs parallel retrieval using infra/service.
// This is a thin adapter (<30 lines).
func (s *parallelRetriever) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("parallelRetriever: query required")
	}

	s.logger.Debug("starting parallel retrieval", map[string]interface{}{
		"step":  "ParallelRetriever",
		"query": state.Query.Text,
	})

	// Get sub-queries from AgenticMetadata (from DecompositionStep)
	var queries []string
	if state.Agentic != nil && len(state.Agentic.SubQueries) > 0 {
		queries = state.Agentic.SubQueries
		s.logger.Info("using decomposed queries", map[string]interface{}{
			"step":           "ParallelRetriever",
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
			"step":  "ParallelRetriever",
			"query": state.Query.Text,
		})
		return fmt.Errorf("parallelRetriever: Retrieve failed: %w", err)
	}

	// Update state (thin adapter 职责)
	for _, result := range results {
		state.RetrievedChunks = append(state.RetrievedChunks, result.Chunks)
	}

	s.logger.Info("retrieval completed", map[string]interface{}{
		"step":          "ParallelRetriever",
		"results_count": len(results),
		"total_chunks":  len(state.RetrievedChunks),
		"query":         state.Query.Text,
	})

	return nil
}
