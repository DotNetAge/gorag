// Package decompose provides query decomposition steps for RAG retrieval pipelines.
package decompose

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// decompose decomposes complex queries into sub-queries.
type decompose struct {
	decomposer core.QueryDecomposer
	logger     logging.Logger
	metrics    core.Metrics
}

// Decompose creates a new query decomposition step with logger and metrics.
//
// Parameters:
//   - decomposer: query decomposer implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(decompose.Decompose(decomposer, logger, metrics))
func Decompose(
	decomposer core.QueryDecomposer,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &decompose{
		decomposer: decomposer,
		logger:     logger,
		metrics:    metrics,
	}
}

// Name returns the step name
func (s *decompose) Name() string {
	return "QueryDecomposition"
}

// Execute decomposes complex queries using infra/service.
func (s *decompose) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("QueryDecomposition: query required")
	}

	// Delegate to infra/service
	result, err := s.decomposer.Decompose(ctx, state.Query)
	if err != nil {
		s.logger.Error("query decomposition failed", err, map[string]interface{}{
			"step":  "QueryDecomposition",
			"query": state.Query.Text,
		})
		return fmt.Errorf("QueryDecomposition failed: %w", err)
	}

	// Update state using AgenticMetadata
	if state.Agentic == nil {
		state.Agentic = core.NewAgenticState()
	}
	state.Agentic.SubQueries = result.SubQueries

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("decompose", len(result.SubQueries))
	}

	s.logger.Info("query decomposed", map[string]interface{}{
		"step":        "QueryDecomposition",
		"sub_queries": len(result.SubQueries),
		"is_complex":  result.IsComplex,
		"query":       state.Query.Text,
	})

	return nil
}
