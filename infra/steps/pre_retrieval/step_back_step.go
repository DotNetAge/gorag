package pre_retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*StepBackStep)(nil)

// StepBackStep is a pipeline step that abstracts the query to a higher-level
// background question, enabling retrieval of broader context.
type StepBackStep struct {
	generator retrieval.StepBackGenerator
	logger    logging.Logger
}

// NewStepBackStep creates a new step-back prompting step.
//
// Parameters:
// - generator: Any retrieval.StepBackGenerator implementation
// - logger: optional structured logger; pass nil to use noop
func NewStepBackStep(generator retrieval.StepBackGenerator, logger logging.Logger) *StepBackStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &StepBackStep{
		generator: generator,
		logger:    logger,
	}
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

	// Record that step-back was applied via AgenticMetadata (not blackboard)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.StepBackQuery = stepBackQuery.Text

	s.logger.Info("StepBackStep completed", map[string]interface{}{
		"step":            "StepBackStep",
		"step_back_query": stepBackQuery.Text,
	})
	return nil
}
