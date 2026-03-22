package rerank

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockReranker struct {
	chunks []*core.Chunk
	err    error
}

func (m *mockReranker) Rerank(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chunks, nil
}

type mockLoggerForRerank struct{}

func (m *mockLoggerForRerank) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForRerank) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForRerank) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForRerank) Error(msg string, err error, fields ...map[string]any) {}

type mockMetricsForRerank struct{}

func (m *mockMetricsForRerank) RecordSearchDuration(engine string, duration any)   {}
func (m *mockMetricsForRerank) RecordSearchResult(engine string, resultCount int)  {}
func (m *mockMetricsForRerank) RecordSearchError(engine string, err error)         {}
func (m *mockMetricsForRerank) RecordIndexingDuration(parser string, duration any) {}
func (m *mockMetricsForRerank) RecordIndexingResult(parser string, count int)      {}
func (m *mockMetricsForRerank) RecordEmbeddingCount(count int)                     {}
func (m *mockMetricsForRerank) RecordVectorStoreOperations(op string, count int)   {}

func TestCrossEncoderRerank_Name(t *testing.T) {
	step := CrossEncoderRerank(nil, 10, nil, nil)
	assert.Equal(t, "CrossEncoderRerank", step.Name())
}

func TestCrossEncoderRerank_Execute_Success(t *testing.T) {
	reranker := &mockReranker{
		chunks: []*core.Chunk{
			{ID: "reranked1", Content: "reranked content 1"},
			{ID: "reranked2", Content: "reranked content 2"},
		},
	}
	step := CrossEncoderRerank(reranker, 10, &mockLoggerForRerank{}, &mockMetricsForRerank{})
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
	assert.Len(t, state.RetrievedChunks[0], 2)
	assert.Equal(t, "reranked1", state.RetrievedChunks[0][0].ID)
}

func TestCrossEncoderRerank_Execute_NilQuery(t *testing.T) {
	step := CrossEncoderRerank(nil, 10, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestCrossEncoderRerank_Execute_EmptyQuery(t *testing.T) {
	step := CrossEncoderRerank(nil, 10, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "", nil),
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query required")
}

func TestCrossEncoderRerank_Execute_EmptyChunks(t *testing.T) {
	reranker := &mockReranker{}
	step := CrossEncoderRerank(reranker, 10, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:           core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestCrossEncoderRerank_Execute_RerankerError(t *testing.T) {
	reranker := &mockReranker{err: errors.New("rerank failed")}
	step := CrossEncoderRerank(reranker, 10, &mockLoggerForRerank{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1", Content: "content"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Rerank failed")
}

func TestCrossEncoderRerank_Execute_TopKLimit(t *testing.T) {
	reranker := &mockReranker{
		chunks: []*core.Chunk{
			{ID: "c1"}, {ID: "c2"}, {ID: "c3"}, {ID: "c4"}, {ID: "c5"},
		},
	}
	step := CrossEncoderRerank(reranker, 3, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		RetrievedChunks: [][]*core.Chunk{
			{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}, {ID: "c4"}, {ID: "c5"}},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks[0], 3)
}

func TestCrossEncoderRerank_Execute_DefaultTopK(t *testing.T) {
	reranker := &mockReranker{chunks: []*core.Chunk{{ID: "c1"}}}
	step := CrossEncoderRerank(reranker, 0, nil, nil)
	assert.NotNil(t, step)
}

func TestCrossEncoderRerank_Execute_RecordsMetrics(t *testing.T) {
	reranker := &mockReranker{
		chunks: []*core.Chunk{{ID: "c1"}, {ID: "c2"}},
	}
	metrics := &mockMetricsForRerank{}
	step := CrossEncoderRerank(reranker, 10, nil, metrics)
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
