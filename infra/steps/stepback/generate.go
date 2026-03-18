// Package stepback provides query abstraction steps for RAG pipelines.
package stepback

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// generate abstracts the query to a higher-level background question.
type generate struct {
	generator *enhancer.StepBackGenerator
}

// Generate creates a new step-back prompting step.
//
// Parameters:
//   - generator: step-back generator implementation
//
// Example:
//
//	p.AddStep(stepback.Generate(generator))
func Generate(generator *enhancer.StepBackGenerator) pipeline.Step[*entity.PipelineState] {
	return &generate{generator: generator}
}

// Name returns the step name
func (s *generate) Name() string {
	return "StepBackGenerate"
}

// Execute generates a step-back query that is more abstract and general.
func (s *generate) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("StepBackGenerate: 'query' not found in state")
	}

	// Generate step-back query
	stepBackQuery, err := s.generator.GenerateStepBackQuery(ctx, state.Query)
	if err != nil {
		return fmt.Errorf("StepBackGenerate failed to generate step-back query: %w", err)
	}

	// Store original query and replace with step-back query
	state.OriginalQuery = state.Query
	state.Query = stepBackQuery

	// Record that step-back was applied via AgenticMetadata
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.StepBackQuery = stepBackQuery.Text

	return nil
}
