// Package filter provides query preprocessing steps for RAG pipelines.
package filter

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// fromQuery extracts structured filter conditions from natural language queries.
type fromQuery struct {
	extractor *enhancer.FilterExtractor
}

// FromQuery creates a new filter extraction step.
//
// Parameters:
//   - extractor: filter extractor implementation
//
// Example:
//
//	p.AddStep(filter.FromQuery(extractor))
func FromQuery(extractor *enhancer.FilterExtractor) pipeline.Step[*entity.PipelineState] {
	return &fromQuery{extractor: extractor}
}

// Name returns the step name
func (s *fromQuery) Name() string {
	return "FilterFromQuery"
}

// Execute extracts metadata filters from the query and stores them in state.Filters.
func (s *fromQuery) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("FilterFromQuery: 'query' not found in state")
	}

	// Extract filters
	filters, err := s.extractor.ExtractFilters(ctx, state.Query)
	if err != nil {
		return fmt.Errorf("FilterFromQuery failed to extract filters: %w", err)
	}

	// Store filters in state for retrieval steps to use
	state.Filters = filters

	// Record that filter extraction was applied via AgenticMetadata
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.Filters = filters

	return nil
}
