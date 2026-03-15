package steps

import (
	"context"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*observationStep)(nil)

// observationSummary records a single iteration's retrieval statistics.
type observationSummary struct {
	Iteration   int
	ChunkGroups int
	TotalChunks int
}

// observationStep records a snapshot of the retrieval state at the end of each
// agentic loop iteration. The snapshot list is accumulated in
// state.Agentic.Custom["observations"] and is available to ReasoningStep on the
// next iteration, enabling the reasoner to track incremental information gain.
type observationStep struct {
	logger logging.Logger
}

// NewObservationStep creates a new observation step.
func NewObservationStep(logger logging.Logger) *observationStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &observationStep{logger: logger}
}

func (s *observationStep) Name() string { return "ObservationStep" }

// Execute appends a retrieval summary to state.Agentic.Custom["observations"].
func (s *observationStep) Execute(_ context.Context, state *entity.PipelineState) error {
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}

	iteration, _ := state.Agentic.Custom["iteration"].(int)
	total := 0
	for _, g := range state.RetrievedChunks {
		total += len(g)
	}

	obs := observationSummary{
		Iteration:   iteration,
		ChunkGroups: len(state.RetrievedChunks),
		TotalChunks: total,
	}

	prev, _ := state.Agentic.Custom["observations"].([]observationSummary)
	state.Agentic.Custom["observations"] = append(prev, obs)

	s.logger.Info("observation recorded", map[string]interface{}{
		"step":         "ObservationStep",
		"iteration":    iteration,
		"total_chunks": total,
	})

	// Guard: if no new chunks were retrieved in this iteration and the agent chose
	// to retrieve, log a warning — the caller may use this to short-circuit.
	if obs.TotalChunks == 0 {
		s.logger.Info("observation: no chunks retrieved", map[string]interface{}{
			"step":      "ObservationStep",
			"iteration": iteration,
		})
	}

	return nil
}

// AgentFinished is a helper used by Searcher.Search to check whether
// TerminationCheckStep has set the finished flag.
func AgentFinished(state *entity.PipelineState) bool {
	if state.Agentic == nil {
		return false
	}
	finished, _ := state.Agentic.Custom["finished"].(bool)
	return finished
}

// AgentCurrentQuery returns the query text overwritten by TerminationCheckStep
// for the retrieve action, falling back to the original query if unchanged.
func AgentCurrentQuery(state *entity.PipelineState) string {
	if state.Query == nil {
		return ""
	}
	return state.Query.Text
}

// AgentSetIteration sets the current iteration counter in the state.
func AgentSetIteration(state *entity.PipelineState, i int) {
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.Custom["iteration"] = i
}
