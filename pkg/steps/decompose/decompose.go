// Package decompose provides query decomposition steps for RAG retrieval pipelines.
package decompose

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// decompose decomposes complex queries into sub-queries.
type decompose struct {
	decomposer core.QueryDecomposer
	logger     logging.Logger
}

// Decompose creates a new query decomposition step with logger.
func Decompose(
	decomposer core.QueryDecomposer,
	logger logging.Logger,
) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &decompose{
		decomposer: decomposer,
		logger:     logger,
	}
}

// Name returns the step name
func (s *decompose) Name() string {
	return "QueryDecomposition"
}

// Execute decomposes complex queries using infra/service.
func (s *decompose) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if context.Query == nil || context.Query.Text == "" {
		return fmt.Errorf("QueryDecomposition: query required")
	}

	s.logger.Debug("decomposing query", map[string]interface{}{
		"query": context.Query.Text,
	})

	result, err := s.decomposer.Decompose(ctx, context.Query)
	if err != nil {
		return fmt.Errorf("QueryDecomposition failed: %w", err)
	}

	if context.Agentic == nil {
		context.Agentic = &core.AgenticContext{}
	}
	context.Agentic.SubQueries = result.SubQueries

	s.logger.Info("query decomposed", map[string]interface{}{
		"sub_queries": len(result.SubQueries),
		"is_complex":  result.IsComplex,
	})

	return nil
}
