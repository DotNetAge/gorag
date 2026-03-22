package filter

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockFilterExtractor struct {
	filters map[string]any
	err     error
}

func (m *mockFilterExtractor) Extract(ctx context.Context, query *core.Query) (map[string]any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.filters, nil
}

func TestFromQuery_Name(t *testing.T) {
	extractor := &mockFilterExtractor{filters: make(map[string]any)}
	step := FromQuery(extractor)
	assert.Equal(t, "FilterFromQuery", step.Name())
}

func TestFromQuery_Execute_Success(t *testing.T) {
	extractor := &mockFilterExtractor{
		filters: map[string]any{
			"year":   "2024",
			"author": "John Doe",
		},
	}
	step := FromQuery(extractor)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("q1", "Find documents from 2024 by John Doe", nil),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.Filters)
	assert.Equal(t, "2024", state.Filters["year"])
	assert.Equal(t, "John Doe", state.Filters["author"])
	assert.NotNil(t, state.Agentic)
	assert.Equal(t, state.Filters, state.Agentic.Filters)
}

func TestFromQuery_Execute_NilQuery(t *testing.T) {
	extractor := &mockFilterExtractor{filters: make(map[string]any)}
	step := FromQuery(extractor)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: nil,
	}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestFromQuery_Execute_ExtractorError(t *testing.T) {
	extractor := &mockFilterExtractor{
		err: errors.New("extraction failed"),
	}
	step := FromQuery(extractor)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("q1", "test query", nil),
	}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extraction failed")
}

func TestFromQuery_Execute_EmptyFilters(t *testing.T) {
	extractor := &mockFilterExtractor{
		filters: map[string]any{},
	}
	step := FromQuery(extractor)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("q1", "simple query", nil),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.Filters)
	assert.Empty(t, state.Filters)
}

func TestFromQuery_Execute_CreatesAgenticState(t *testing.T) {
	extractor := &mockFilterExtractor{
		filters: map[string]any{"key": "value"},
	}
	step := FromQuery(extractor)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("q1", "test", nil),
		Agentic: nil,
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.NotNil(t, state.Agentic.Filters)
}

func TestFromQuery_Execute_PreservesExistingAgentic(t *testing.T) {
	extractor := &mockFilterExtractor{
		filters: map[string]any{"new": "filter"},
	}
	step := FromQuery(extractor)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("q1", "test", nil),
		Agentic: &core.AgenticContext{
			CacheHit: true,
		},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.NotNil(t, state.Agentic)
	assert.True(t, state.Agentic.CacheHit)
	assert.Equal(t, "filter", state.Agentic.Filters["new"])
}
