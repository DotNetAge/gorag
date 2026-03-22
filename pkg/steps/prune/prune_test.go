package prune

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockEnhancer struct {
	chunks []*core.Chunk
	err    error
}

func (m *mockEnhancer) Enhance(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chunks, nil
}

type mockLoggerForPrune struct{}

func (m *mockLoggerForPrune) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForPrune) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForPrune) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForPrune) Error(msg string, err error, fields ...map[string]any) {}

type mockMetricsForPrune struct{}

func (m *mockMetricsForPrune) RecordSearchDuration(engine string, duration any)   {}
func (m *mockMetricsForPrune) RecordSearchResult(engine string, resultCount int)  {}
func (m *mockMetricsForPrune) RecordSearchError(engine string, err error)         {}
func (m *mockMetricsForPrune) RecordIndexingDuration(parser string, duration any) {}
func (m *mockMetricsForPrune) RecordIndexingResult(parser string, count int)      {}
func (m *mockMetricsForPrune) RecordEmbeddingCount(count int)                     {}
func (m *mockMetricsForPrune) RecordVectorStoreOperations(op string, count int)   {}

func TestPrune_Name(t *testing.T) {
	step := Prune(nil, nil, nil)
	assert.Equal(t, "Prune", step.Name())
}

func TestPrune_Execute_Success(t *testing.T) {
	enhancer := &mockEnhancer{
		chunks: []*core.Chunk{
			{ID: "pruned1", Content: "pruned content"},
		},
	}
	step := Prune(enhancer, &mockLoggerForPrune{}, &mockMetricsForPrune{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "original1"}, {ID: "c2", Content: "original2"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 1)
	assert.Equal(t, "pruned1", state.RetrievedChunks[0][0].ID)
}

func TestPrune_Execute_NilQuery(t *testing.T) {
	step := Prune(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestPrune_Execute_EmptyChunks(t *testing.T) {
	enhancer := &mockEnhancer{chunks: []*core.Chunk{}}
	step := Prune(enhancer, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:           core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestPrune_Execute_EnhancerError(t *testing.T) {
	enhancer := &mockEnhancer{err: errors.New("enhancement failed")}
	step := Prune(enhancer, &mockLoggerForPrune{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "content"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "enhance failed")
}

func TestPrune_Execute_WithNilLogger(t *testing.T) {
	enhancer := &mockEnhancer{
		chunks: []*core.Chunk{{ID: "result"}},
	}
	step := Prune(enhancer, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "content"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks[0], 1)
}

func TestPrune_Execute_FlattensChunks(t *testing.T) {
	enhancer := &mockEnhancer{
		chunks: []*core.Chunk{{ID: "flattened"}},
	}
	step := Prune(enhancer, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1"}},
			{{ID: "c2"}},
			{{ID: "c3"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestPrune_Execute_RecordsMetrics(t *testing.T) {
	enhancer := &mockEnhancer{
		chunks: []*core.Chunk{{ID: "c1"}, {ID: "c2"}},
	}
	metrics := &mockMetricsForPrune{}
	step := Prune(enhancer, nil, metrics)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1"}, {ID: "c2"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}
