package image

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockVectorStoreForImage struct {
	vectors []*core.Vector
	scores  []float32
	err     error
}

func (m *mockVectorStoreForImage) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.vectors, m.scores, nil
}

func (m *mockVectorStoreForImage) Upsert(ctx context.Context, vectors []*core.Vector) error {
	return nil
}

func (m *mockVectorStoreForImage) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockVectorStoreForImage) Close(ctx context.Context) error {
	return nil
}

type mockLoggerForImage struct{}

func (m *mockLoggerForImage) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForImage) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForImage) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForImage) Error(msg string, err error, fields ...map[string]any) {}

type mockMetricsForImage struct{}

func (m *mockMetricsForImage) RecordSearchDuration(engine string, duration any)   {}
func (m *mockMetricsForImage) RecordSearchResult(engine string, resultCount int)  {}
func (m *mockMetricsForImage) RecordSearchError(engine string, err error)         {}
func (m *mockMetricsForImage) RecordIndexingDuration(parser string, duration any) {}
func (m *mockMetricsForImage) RecordIndexingResult(parser string, count int)      {}
func (m *mockMetricsForImage) RecordEmbeddingCount(count int)                     {}
func (m *mockMetricsForImage) RecordVectorStoreOperations(op string, count int)   {}
func (m *mockMetricsForImage) RecordQueryCount(engine string)                          {}
func (m *mockMetricsForImage) RecordLLMTokenUsage(model string, prompt int, completion int) {}
func (m *mockMetricsForImage) RecordRAGEvaluation(metric string, score float32)         {}

func TestSearch_Name(t *testing.T) {
	step := Search(nil, 10, nil, nil)
	assert.Equal(t, "ImageSearch", step.Name())
}

func TestSearch_Execute_NilAgentic(t *testing.T) {
	step := Search(nil, 10, &mockLoggerForImage{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: nil,
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearch_Execute_NoImageVector(t *testing.T) {
	step := Search(nil, 10, &mockLoggerForImage{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearch_Execute_WithImageVector(t *testing.T) {
	store := &mockVectorStoreForImage{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "image result 1"}},
			{ID: "v2", Metadata: map[string]any{"content": "image result 2"}},
		},
		scores: []float32{0.9, 0.8},
	}
	step := Search(store, 10, &mockLoggerForImage{}, &mockMetricsForImage{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{
			Custom: map[string]any{
				"image_vector": []float32{1.0, 2.0, 3.0},
			},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Len(t, state.RetrievedChunks, 1)
	assert.Len(t, state.RetrievedChunks[0], 2)
}

func TestSearch_Execute_DefaultTopK(t *testing.T) {
	step := Search(nil, 0, nil, nil)
	assert.NotNil(t, step)
}

func TestSearch_Execute_RecordsMetrics(t *testing.T) {
	store := &mockVectorStoreForImage{
		vectors: []*core.Vector{
			{ID: "v1", Metadata: map[string]any{"content": "image"}},
		},
		scores: []float32{0.9},
	}
	metrics := &mockMetricsForImage{}
	step := Search(store, 10, nil, metrics)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{
			Custom: map[string]any{
				"image_vector": []float32{1.0, 2.0, 3.0},
			},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearch_Execute_EmptyImageVector(t *testing.T) {
	step := Search(nil, 10, &mockLoggerForImage{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{
			Custom: map[string]any{
				"image_vector": []float32{},
			},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestSearch_Execute_InvalidImageVectorType(t *testing.T) {
	step := Search(nil, 10, &mockLoggerForImage{}, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test", nil),
		Agentic: &core.AgenticContext{
			Custom: map[string]any{
				"image_vector": "not a float32 array",
			},
		},
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

var _ core.VectorStore = (*mockVectorStoreForImage)(nil)
