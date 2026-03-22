package stepback

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockDecomposerForStepback struct {
	result *core.DecompositionResult
	err    error
}

func (m *mockDecomposerForStepback) Decompose(ctx context.Context, query *core.Query) (*core.DecompositionResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestGenerate_Name(t *testing.T) {
	step := Generate(nil)
	assert.Equal(t, "StepBackGenerate", step.Name())
}

func TestGenerate_Execute_WithSubQueries(t *testing.T) {
	decomposer := &mockDecomposerForStepback{
		result: &core.DecompositionResult{
			SubQueries: []string{"background question"},
			Reasoning:  "test reasoning",
		},
	}
	step := Generate(decomposer)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "original query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.Equal(t, "background question", state.Agentic.StepBackQuery)
}

func TestGenerate_Execute_WithOnlyReasoning(t *testing.T) {
	decomposer := &mockDecomposerForStepback{
		result: &core.DecompositionResult{
			SubQueries: []string{},
			Reasoning:  "fallback reasoning",
		},
	}
	step := Generate(decomposer)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "original query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Equal(t, "fallback reasoning", state.Agentic.StepBackQuery)
}

func TestGenerate_Execute_NilQuery(t *testing.T) {
	decomposer := &mockDecomposerForStepback{
		result: &core.DecompositionResult{},
	}
	step := Generate(decomposer)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestGenerate_Execute_DecomposerError(t *testing.T) {
	decomposer := &mockDecomposerForStepback{err: errors.New("decompose failed")}
	step := Generate(decomposer)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate step-back query")
}

func TestGenerate_Execute_CreatesAgenticContext(t *testing.T) {
	decomposer := &mockDecomposerForStepback{
		result: &core.DecompositionResult{
			SubQueries: []string{"step back query"},
		},
	}
	step := Generate(decomposer)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "original", nil),
		Agentic: nil,
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.Equal(t, "step back query", state.Agentic.StepBackQuery)
}

func TestGenerate_Execute_PreservesQueryID(t *testing.T) {
	decomposer := &mockDecomposerForStepback{
		result: &core.DecompositionResult{
			SubQueries: []string{"background"},
		},
	}
	step := Generate(decomposer)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("my-id", "original", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
}
