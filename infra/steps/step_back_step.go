package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*StepBackStep)(nil)

// StepBackStep is a pipeline step that abstracts the query to a higher-level
// background question, enabling retrieval of broader context.
type StepBackStep struct {
	generator *enhancer.StepBackGenerator
}

// NewStepBackStep creates a new step-back prompting step.
//
// Parameters:
// - generator: The step-back generator instance
//
// Returns:
// - A new StepBackStep instance
func NewStepBackStep(generator *enhancer.StepBackGenerator) *StepBackStep {
	return &StepBackStep{generator: generator}
}

// Name returns the step name
func (s *StepBackStep) Name() string {
	return "StepBackStep"
}

// Execute generates a step-back query that is more abstract and general,
// which can retrieve richer contextual information.
func (s *StepBackStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("StepBackStep: 'query' not found in state")
	}

	// Generate step-back query
	stepBackQuery, err := s.generator.GenerateStepBackQuery(ctx, state.Query)
	if err != nil {
		return fmt.Errorf("StepBackStep failed to generate step-back query: %w", err)
	}

	// Store original query and replace with step-back query
	state.OriginalQuery = state.Query
	state.Query = stepBackQuery
	
	// Mark that step-back was applied
	state.Query.Metadata["step_back_applied"] = true

	fmt.Printf("StepBackStep: generated step-back query: %s\n", stepBackQuery.Text)
	return nil
}
