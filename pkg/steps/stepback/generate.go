// Package stepback provides query abstraction steps for RAG pipelines.
package stepback

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
)

// generate abstracts the query to a higher-level background question.
type generate struct {
	generator core.QueryDecomposer // Using interface instead of concrete type
}

// Generate creates a new step-back prompting step.
//
// Parameters:
//   - generator: step-back generator implementation
//
// Example:
//
//	p.AddStep(stepback.Generate(generator))
func Generate(generator core.QueryDecomposer) pipeline.Step[*core.State] {
	return &generate{generator: generator}
}

// Name returns the step name
func (s *generate) Name() string {
	return "StepBackGenerate"
}

// Execute generates a step-back query that is more abstract and general.
func (s *generate) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil {
		return fmt.Errorf("StepBackGenerate: 'query' not found in state")
	}

	// Generate step-back query
	result, err := s.generator.Decompose(ctx, state.Query)
	if err != nil {
		return fmt.Errorf("StepBackGenerate failed to generate step-back query: %w", err)
	}

	// Create new query from decomposition result
	var stepBackQuery *core.Query
	if len(result.SubQueries) > 0 {
		stepBackQuery = core.NewQuery(state.Query.ID, result.SubQueries[0], state.Query.Metadata)
	} else {
		stepBackQuery = core.NewQuery(state.Query.ID, result.Reasoning, state.Query.Metadata)
	}

	// Record that step-back was applied via AgenticMetadata
	if state.Agentic == nil {
		state.Agentic = core.NewAgenticState()
	}
	state.Agentic.StepBackQuery = stepBackQuery.Text

	return nil
}
