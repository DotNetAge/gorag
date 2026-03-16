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
var _ pipeline.Step[*entity.PipelineState] = (*QueryToFilterStep)(nil)

// QueryToFilterStep is a pipeline step that extracts structured filter
// conditions from natural language queries.
type QueryToFilterStep struct {
	extractor retrieval.FilterExtractor
	logger    logging.Logger
}

// NewQueryToFilterStep creates a new filter extraction step.
//
// Parameters:
// - extractor: Any retrieval.FilterExtractor implementation
// - logger: optional structured logger; pass nil to use noop
func NewQueryToFilterStep(extractor retrieval.FilterExtractor, logger logging.Logger) *QueryToFilterStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &QueryToFilterStep{
		extractor: extractor,
		logger:    logger,
	}
}

// Name returns the step name
func (s *QueryToFilterStep) Name() string {
	return "QueryToFilterStep"
}

// Execute extracts metadata filters from the query and stores them in state.Agentic.Filters.
// These filters can be used by subsequent retrieval steps for pre-filtering.
func (s *QueryToFilterStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("QueryToFilterStep: 'query' not found in state")
	}

	// Extract filters
	filters, err := s.extractor.ExtractFilters(ctx, state.Query)
	if err != nil {
		return fmt.Errorf("QueryToFilterStep failed to extract filters: %w", err)
	}

	// Store filters in state for retrieval steps to use
	state.Filters = filters

	// Record that filter extraction was applied via AgenticMetadata (not blackboard)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.Filters = filters

	s.logger.Info("QueryToFilterStep completed", map[string]interface{}{
		"step":         "QueryToFilterStep",
		"filter_count": len(filters),
	})
	return nil
}
