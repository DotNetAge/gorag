package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// terminationCheck inspects the AgentAction produced by actionSelection and
// decides how the agentic loop should proceed:
//
//   - ActionFinish   → sets state.Agentic.Custom["finished"] = true  (loop exits)
//   - ActionRetrieve → overwrites state.Query.Text with action.Query (loop continues with new query)
//   - ActionReflect  → no-op on the query (loop continues for reflection)
type terminationCheck struct {
	logger logging.Logger
}

// TerminationCheck creates a termination-check step.
//
// Parameters:
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//
// Example:
//
//	p.AddStep(agentic.TerminationCheck(logger))
func TerminationCheck(logger logging.Logger) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &terminationCheck{logger: logger}
}

// Name returns the step name
func (s *terminationCheck) Name() string {
	return "TerminationCheck"
}

// Execute reads state.Agentic.Custom["agent_action"] and updates the state accordingly.
func (s *terminationCheck) Execute(_ context.Context, state *entity.PipelineState) error {
	if state.Agentic == nil {
		return fmt.Errorf("terminationCheck: AgenticMetadata is nil")
	}

	raw, ok := state.Agentic.Custom["agent_action"]
	if !ok {
		return fmt.Errorf("terminationCheck: agent_action not set by ActionSelection")
	}
	action, ok := raw.(*retrieval.AgentAction)
	if !ok {
		return fmt.Errorf("terminationCheck: agent_action has unexpected type %T", raw)
	}

	switch action.Type {
	case retrieval.ActionFinish:
		state.Agentic.Custom["finished"] = true
		s.logger.Info("agent decided to finish", map[string]interface{}{
			"step": "TerminationCheck",
		})

	case retrieval.ActionRetrieve:
		if action.Query != "" && state.Query != nil {
			state.Query.Text = action.Query
		}
		s.logger.Info("agent decided to retrieve", map[string]interface{}{
			"step":      "TerminationCheck",
			"sub_query": action.Query,
		})

	case retrieval.ActionReflect:
		s.logger.Info("agent decided to reflect", map[string]interface{}{
			"step": "TerminationCheck",
		})
	}

	return nil
}
