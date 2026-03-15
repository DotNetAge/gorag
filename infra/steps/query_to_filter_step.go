package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*QueryToFilterStep)(nil)

// QueryToFilterStep is a pipeline step that extracts structured filter
// conditions from natural language queries.
type QueryToFilterStep struct {
	extractor *enhancer.FilterExtractor
}

// NewQueryToFilterStep creates a new filter extraction step.
//
// Parameters:
// - extractor: The filter extractor instance
//
// Returns:
// - A new QueryToFilterStep instance
func NewQueryToFilterStep(extractor *enhancer.FilterExtractor) *QueryToFilterStep {
	return &QueryToFilterStep{extractor: extractor}
}

// Name returns the step name
func (s *QueryToFilterStep) Name() string {
	return "QueryToFilterStep"
}

// Execute extracts metadata filters from the query and stores them in state.
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
	
	// Mark that filter extraction was applied
	state.Query.Metadata["filters_extracted"] = true

	fmt.Printf("QueryToFilterStep: extracted %d filter(s)\n", len(filters))
	return nil
}
