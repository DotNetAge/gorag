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
var _ pipeline.Step[*entity.PipelineState] = (*terminationCheckStep)(nil)

// terminationCheckStep inspects the AgentAction produced by actionSelectionStep and
// decides how the agentic loop should proceed:
//
//   - ActionFinish   → sets state.Agentic.Custom["finished"] = true  (loop exits)
//   - ActionRetrieve → overwrites state.Query.Text with action.Query (loop continues with new query)
//   - ActionReflect  → no-op on the query (loop continues for reflection)
type terminationCheckStep struct {
	logger logging.Logger
}

// NewTerminationCheckStep creates a new termination-check step.
func NewTerminationCheckStep(logger logging.Logger) *terminationCheckStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &terminationCheckStep{logger: logger}
}

func (s *terminationCheckStep) Name() string { return "TerminationCheckStep" }

// Execute reads state.Agentic.Custom["agent_action"] and updates the state accordingly.
func (s *terminationCheckStep) Execute(_ context.Context, state *entity.PipelineState) error {
	if state.Agentic == nil {
		return fmt.Errorf("terminationCheckStep: AgenticMetadata is nil")
	}

	raw, ok := state.Agentic.Custom["agent_action"]
	if !ok {
		return fmt.Errorf("terminationCheckStep: agent_action not set by ActionSelectionStep")
	}
	action, ok := raw.(*retrieval.AgentAction)
	if !ok {
		return fmt.Errorf("terminationCheckStep: agent_action has unexpected type %T", raw)
	}

	switch action.Type {
	case retrieval.ActionFinish:
		state.Agentic.Custom["finished"] = true
		s.logger.Info("agent decided to finish", map[string]interface{}{
			"step": "TerminationCheckStep",
		})

	case retrieval.ActionRetrieve:
		if action.Query != "" && state.Query != nil {
			state.Query.Text = action.Query
		}
		s.logger.Info("agent decided to retrieve", map[string]interface{}{
			"step":      "TerminationCheckStep",
			"sub_query": action.Query,
		})

	case retrieval.ActionReflect:
		s.logger.Info("agent decided to reflect", map[string]interface{}{
			"step": "TerminationCheckStep",
		})
	}

	return nil
}
