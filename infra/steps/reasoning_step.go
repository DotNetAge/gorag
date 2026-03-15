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
var _ pipeline.Step[*entity.PipelineState] = (*reasoningStep)(nil)

// reasoningStep calls AgentReasoner to produce a reasoning trace for the current
// retrieval state. The trace is stored in state.Agentic.Custom["reasoning"] so that
// the following actionSelectionStep can use it to decide the next action.
type reasoningStep struct {
	reasoner retrieval.AgentReasoner
	logger   logging.Logger
}

// NewReasoningStep creates a new reasoning step with the given reasoner and logger.
func NewReasoningStep(reasoner retrieval.AgentReasoner, logger logging.Logger) *reasoningStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &reasoningStep{reasoner: reasoner, logger: logger}
}

func (s *reasoningStep) Name() string { return "ReasoningStep" }

// Execute calls the reasoner and writes the resulting trace to
// state.Agentic.Custom["reasoning"].
func (s *reasoningStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("reasoningStep: query required")
	}
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}

	reasoning, err := s.reasoner.Reason(ctx, state.Query.Text, state.RetrievedChunks, state.Answer)
	if err != nil {
		s.logger.Error("reasoning failed", err, map[string]interface{}{
			"step":  "ReasoningStep",
			"query": state.Query.Text,
		})
		return fmt.Errorf("reasoningStep: Reason failed: %w", err)
	}

	state.Agentic.Custom["reasoning"] = reasoning
	s.logger.Info("reasoning completed", map[string]interface{}{
		"step":  "ReasoningStep",
		"query": state.Query.Text,
	})
	return nil
}
