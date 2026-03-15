package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*queryDecomposer)(nil)

// queryDecomposer is a thin adapter that delegates to infra/service.
type queryDecomposer struct {
	decomposer retrieval.QueryDecomposer
	logger     logging.Logger
}

// NewQueryDecomposer creates a new query decomposition step with logger.
func NewQueryDecomposer(decomposer retrieval.QueryDecomposer, logger logging.Logger) *queryDecomposer {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &queryDecomposer{
		decomposer: decomposer,
		logger:     logger,
	}
}

// Name returns the step name
func (s *queryDecomposer) Name() string {
	return "QueryDecomposer"
}

// Execute decomposes complex queries using infra/service.
// This is a thin adapter (<30 lines).
func (s *queryDecomposer) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("queryDecomposer: query required")
	}

	// Delegate to infra/service (thick business logic)
	result, err := s.decomposer.Decompose(ctx, state.Query)
	if err != nil {
		s.logger.Error("decompose failed", err, map[string]interface{}{
			"step":  "QueryDecomposer",
			"query": state.Query.Text,
		})
		return fmt.Errorf("queryDecomposer: Decompose failed: %w", err)
	}

	// Update state using AgenticMetadata (thin adapter 职责)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.SubQueries = result.SubQueries

	s.logger.Info("query decomposed", map[string]interface{}{
		"step":        "QueryDecomposer",
		"sub_queries": len(result.SubQueries),
		"is_complex":  result.IsComplex,
		"query":       state.Query.Text,
	})

	return nil
}
