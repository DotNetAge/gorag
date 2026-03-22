package sparse

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockSearcher struct {
	results []*Result
	err     error
}

func (m *mockSearcher) Search(ctx context.Context, query string, topK int) ([]*Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

type mockLoggerForSparse struct{}

func (m *mockLoggerForSparse) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForSparse) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForSparse) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForSparse) Error(msg string, err error, fields ...map[string]any) {}

type mockMetricsForSparse struct{}

func (m *mockMetricsForSparse) RecordSearchDuration(engine string, duration any)   {}
func (m *mockMetricsForSparse) RecordSearchResult(engine string, resultCount int)  {}
func (m *mockMetricsForSparse) RecordSearchError(engine string, err error)         {}
func (m *mockMetricsForSparse) RecordIndexingDuration(parser string, duration any) {}
func (m *mockMetricsForSparse) RecordIndexingResult(parser string, count int)      {}
func (m *mockMetricsForSparse) RecordEmbeddingCount(count int)                     {}
func (m *mockMetricsForSparse) RecordVectorStoreOperations(op string, count int)   {}

func TestSearch_Name(t *testing.T) {
	step := Search(nil, 10, nil, nil)
	assert.Equal(t, "SparseSearch", step.Name())
}

func TestSearch_Execute_Success(t *testing.T) {
	searcher := &mockSearcher{
		results: []*Result{
			{Chunk: &core.Chunk{ID: "c1", Content: "result1"}, Score: 0.9},
			{Chunk: &core.Chunk{ID: "c2", Content: "result2"}, Score: 0.8},
		},
	}
	step := Search(searcher, 10, &mockLoggerForSparse{}, &mockMetricsForSparse{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 2)
	assert.Equal(t, "c1", state.RetrievedChunks[0][0].ID)
}

func TestSearch_Execute_NilQuery(t *testing.T) {
	step := Search(nil, 10, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query")
}

func TestSearch_Execute_SearcherError(t *testing.T) {
	searcher := &mockSearcher{err: errors.New("search failed")}
	step := Search(searcher, 10, &mockLoggerForSparse{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
	}

	err := step.Execute(ctx, state)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retrieve")
}

func TestSearch_Execute_EmptyResults(t *testing.T) {
	searcher := &mockSearcher{results: []*Result{}}
	step := Search(searcher, 10, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Empty(t, state.RetrievedChunks[0])
}

func TestSearch_Execute_DefaultTopK(t *testing.T) {
	searcher := &mockSearcher{results: []*Result{}}
	step := Search(searcher, 0, nil, nil)
	assert.NotNil(t, step)
}

func TestSearch_Execute_RecordsMetrics(t *testing.T) {
	searcher := &mockSearcher{
		results: []*Result{
			{Chunk: &core.Chunk{ID: "c1"}, Score: 0.9},
		},
	}
	metrics := &mockMetricsForSparse{}
	step := Search(searcher, 10, nil, metrics)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestResult_Structure(t *testing.T) {
	result := &Result{
		Chunk: &core.Chunk{ID: "test-chunk", Content: "test content"},
		Score: 0.95,
	}

	assert.Equal(t, "test-chunk", result.Chunk.ID)
	assert.Equal(t, 0.95, result.Score)
}
