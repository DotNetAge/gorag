package decompose

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockDecomposer struct {
	result *core.DecompositionResult
	err    error
}

func (m *mockDecomposer) Decompose(ctx context.Context, query *core.Query) (*core.DecompositionResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestDecompose_Name(t *testing.T) {
	decomposer := &mockDecomposer{result: &core.DecompositionResult{}}
	step := Decompose(decomposer, nil)
	assert.Equal(t, "QueryDecomposition", step.Name())
}

func TestDecompose_Execute_Success(t *testing.T) {
	decomposer := &mockDecomposer{
		result: &core.DecompositionResult{
			SubQueries: []string{"sub1", "sub2"},
			Reasoning:  "test reasoning",
			IsComplex:  true,
		},
	}
	step := Decompose(decomposer, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "original query", nil),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.Equal(t, []string{"sub1", "sub2"}, state.Agentic.SubQueries)
}

func TestDecompose_Execute_NilQuery(t *testing.T) {
	decomposer := &mockDecomposer{result: &core.DecompositionResult{}}
	step := Decompose(decomposer, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestDecompose_Execute_EmptyQuery(t *testing.T) {
	decomposer := &mockDecomposer{result: &core.DecompositionResult{}}
	step := Decompose(decomposer, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "", nil),
	}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestDecompose_Execute_DecomposerError(t *testing.T) {
	decomposer := &mockDecomposer{err: errors.New("decomposer failed")}
	step := Decompose(decomposer, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decomposer failed")
}

func TestDecompose_Execute_EmptySubQueries(t *testing.T) {
	decomposer := &mockDecomposer{
		result: &core.DecompositionResult{
			SubQueries: []string{},
			IsComplex:  false,
		},
	}
	step := Decompose(decomposer, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.Empty(t, state.Agentic.SubQueries)
}
