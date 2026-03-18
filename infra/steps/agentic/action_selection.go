package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// actionSelection reads the current reasoning trace from
// state.Agentic.Custom["reasoning"] and calls AgentActionSelector to decide
// the next action (retrieve / reflect / finish).
// The resulting *retrieval.AgentAction is written to state.Agentic.Custom["agent_action"].
type actionSelection struct {
	selector      retrieval.AgentActionSelector
	maxIterations int
	logger        logging.Logger
}

// ActionSelection creates an action-selection step.
//
// Parameters:
//   - selector: agent action selector implementation
//   - maxIterations: maximum iterations allowed (default: 5)
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//
// Example:
//
//	p.AddStep(agentic.ActionSelection(selector, 5, logger))
func ActionSelection(selector retrieval.AgentActionSelector, maxIterations int, logger logging.Logger) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if maxIterations <= 0 {
		maxIterations = 5
	}
	return &actionSelection{
		selector:      selector,
		maxIterations: maxIterations,
		logger:        logger,
	}
}

// Name returns the step name
func (s *actionSelection) Name() string {
	return "ActionSelection"
}

// Execute reads the reasoning trace, calls the selector, and stores the chosen
// AgentAction in state.Agentic.Custom["agent_action"].
func (s *actionSelection) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("actionSelection: query required")
	}
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}

	reasoning, _ := state.Agentic.Custom["reasoning"].(string)
	iteration, _ := state.Agentic.Custom["iteration"].(int)

	action, err := s.selector.SelectAction(ctx, state.Query.Text, reasoning, iteration, s.maxIterations)
	if err != nil {
		s.logger.Error("action selection failed", err, map[string]interface{}{
			"step":      "ActionSelection",
			"query":     state.Query.Text,
			"iteration": iteration,
		})
		return fmt.Errorf("actionSelection: SelectAction failed: %w", err)
	}

	state.Agentic.Custom["agent_action"] = action
	s.logger.Info("action selected", map[string]interface{}{
		"step":      "ActionSelection",
		"action":    string(action.Type),
		"sub_query": action.Query,
		"iteration": iteration,
	})
	return nil
}
