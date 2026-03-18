package agentic

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// reasoning calls AgentReasoner to produce a reasoning trace for the current
// retrieval state. The trace is stored in state.Agentic.Custom["reasoning"] so that
// the following actionSelection can use it to decide the next action.
type reasoning struct {
	reasoner retrieval.AgentReasoner
	logger   logging.Logger
}

// Reasoning creates a reasoning step with the given reasoner and logger.
//
// Parameters:
//   - reasoner: agent reasoner implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//
// Example:
//
//	p.AddStep(agentic.Reasoning(reasoner, logger))
func Reasoning(reasoner retrieval.AgentReasoner, logger logging.Logger) pipeline.Step[*entity.PipelineState] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &reasoning{
		reasoner: reasoner,
		logger:   logger,
	}
}

// Name returns the step name
func (s *reasoning) Name() string {
	return "Reasoning"
}

// Execute calls the reasoner and writes the resulting trace to
// state.Agentic.Custom["reasoning"].
func (s *reasoning) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("reasoning: query required")
	}
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}

	reasoning, err := s.reasoner.Reason(ctx, state.Query.Text, state.RetrievedChunks, state.Answer)
	if err != nil {
		s.logger.Error("reasoning failed", err, map[string]interface{}{
			"step":  "Reasoning",
			"query": state.Query.Text,
		})
		return fmt.Errorf("reasoning: Reason failed: %w", err)
	}

	state.Agentic.Custom["reasoning"] = reasoning
	s.logger.Info("reasoning completed", map[string]interface{}{
		"step":  "Reasoning",
		"query": state.Query.Text,
	})
	return nil
}
